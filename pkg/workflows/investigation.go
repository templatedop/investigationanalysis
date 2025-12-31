// Package workflows provides Temporal workflow definitions for fraud investigation
package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/fraudinvestigation/rag-framework/pkg/agents"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

const (
	// TaskQueueName is the Temporal task queue for fraud investigation
	TaskQueueName = "fraud-investigation"

	// HumanReviewSignal is the signal name for human review decisions
	HumanReviewSignal = "human-review"

	// DefaultHumanReviewTimeout is the timeout for human review
	DefaultHumanReviewTimeout = 24 * time.Hour
)

// FraudInvestigationWorkflow orchestrates the multi-agent fraud investigation
func FraudInvestigationWorkflow(ctx workflow.Context, input *models.FraudAlertInput) (*models.InvestigationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting fraud investigation workflow", "claimID", input.ClaimID)

	// Initialize investigation state
	state := models.NewInvestigationState(input)

	// Configure activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// ============================================
	// PHASE 1: DATA GATHERING (Parallel)
	// ============================================
	state.UpdateStatus(models.StatusInProgress)
	state.AddAuditEntry("phase_started", "", "Phase 1: Data Gathering")

	var enrichedData *models.EnrichedData
	var entityLinks *models.EntityLinks

	// Run data gathering agents in parallel
	dataFuture := workflow.ExecuteActivity(ctx, DataEnrichmentActivity, input)
	entityFuture := workflow.ExecuteActivity(ctx, EntityResolutionActivity, input)

	// Wait for both to complete
	if err := dataFuture.Get(ctx, &enrichedData); err != nil {
		logger.Error("Data enrichment failed", "error", err)
		// Continue with partial data
	}

	if err := entityFuture.Get(ctx, &entityLinks); err != nil {
		logger.Error("Entity resolution failed", "error", err)
	}

	state.EnrichedData = enrichedData
	state.EntityLinks = entityLinks
	state.AddAuditEntry("phase_completed", "", "Phase 1 completed")

	// ============================================
	// PHASE 2: ANALYSIS (Parallel)
	// ============================================
	state.AddAuditEntry("phase_started", "", "Phase 2: Analysis")

	analysisCtx := &agents.AnalysisContext{
		AlertInput:   input,
		EnrichedData: enrichedData,
		EntityLinks:  entityLinks,
		State:        state,
	}

	findings := &models.AgentFindings{}

	// Run analysis agents in parallel
	patternFuture := workflow.ExecuteActivity(ctx, PatternAnalysisActivity, analysisCtx)
	networkFuture := workflow.ExecuteActivity(ctx, NetworkAnalysisActivity, analysisCtx)
	temporalFuture := workflow.ExecuteActivity(ctx, TemporalAnalysisActivity, analysisCtx)
	documentFuture := workflow.ExecuteActivity(ctx, DocumentAnalysisActivity, analysisCtx)
	policyFuture := workflow.ExecuteActivity(ctx, PolicyComplianceActivity, analysisCtx)

	// Collect all findings
	patternFuture.Get(ctx, &findings.Pattern)
	networkFuture.Get(ctx, &findings.Network)
	temporalFuture.Get(ctx, &findings.Temporal)
	documentFuture.Get(ctx, &findings.Document)
	policyFuture.Get(ctx, &findings.Policy)

	state.Findings = findings
	state.AddAuditEntry("phase_completed", "", "Phase 2 completed")

	// ============================================
	// PHASE 3: RISK ASSESSMENT (Sequential)
	// ============================================
	state.AddAuditEntry("phase_started", "", "Phase 3: Risk Assessment")

	var riskAssessment *models.RiskAssessment
	riskInput := &agents.RiskInput{
		State:    state,
		Findings: findings,
	}

	if err := workflow.ExecuteActivity(ctx, RiskAssessmentActivity, riskInput).Get(ctx, &riskAssessment); err != nil {
		logger.Error("Risk assessment failed", "error", err)
		// Use default medium risk
		riskAssessment = &models.RiskAssessment{
			Score:             0.5,
			Level:             models.RiskMedium,
			Confidence:        0.5,
			RecommendedAction: "MANUAL_REVIEW",
			RequiresHumanReview: true,
		}
	}

	state.RiskAssessment = riskAssessment
	state.AddAuditEntry("phase_completed", "", "Phase 3 completed")

	// ============================================
	// PHASE 4: DECISION & ESCALATION
	// ============================================
	state.AddAuditEntry("phase_started", "", "Phase 4: Decision")

	if riskAssessment.RequiresHumanReview {
		state.UpdateStatus(models.StatusReview)

		// Generate explanation for human reviewer
		explanationInput := &agents.ExplanationInput{
			Audience: "claims_adjuster",
			State:    state,
			Findings: findings,
		}

		var explanation *agents.HumanReadableExplanation
		workflow.ExecuteActivity(ctx, ExplanationActivity, explanationInput).Get(ctx, &explanation)

		// Wait for human decision with timeout
		var humanDecision *models.HumanDecision
		humanReviewCh := workflow.GetSignalChannel(ctx, HumanReviewSignal)

		selector := workflow.NewSelector(ctx)
		selector.AddReceive(humanReviewCh, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &humanDecision)
		})
		selector.AddFuture(workflow.NewTimer(ctx, DefaultHumanReviewTimeout), func(f workflow.Future) {
			humanDecision = &models.HumanDecision{
				Action: "AUTO_ESCALATE",
				Reason: "Human review timeout exceeded",
			}
		})
		selector.Select(ctx)

		state.HumanDecision = humanDecision
		state.AddAuditEntry("human_decision", "", humanDecision.Action)
	}

	// Determine final decision
	state.FinalDecision = determineFinalDecision(state)
	state.AddAuditEntry("phase_completed", "", "Phase 4 completed: "+state.FinalDecision)

	// ============================================
	// PHASE 5: COMPILE EVIDENCE & REPORT
	// ============================================
	state.AddAuditEntry("phase_started", "", "Phase 5: Evidence Compilation")

	var report *models.InvestigationReport
	if err := workflow.ExecuteActivity(ctx, EvidenceCompilationActivity, state).Get(ctx, &report); err != nil {
		logger.Error("Evidence compilation failed", "error", err)
	}

	state.AddAuditEntry("phase_completed", "", "Phase 5 completed")

	// ============================================
	// PHASE 6: EXECUTE ACTION
	// ============================================
	if state.FinalDecision == "FRAUD_CONFIRMED" || state.FinalDecision == "DENY_CLAIM" {
		actionInput := &ActionInput{
			ClaimID:    input.ClaimID,
			Action:     "DENY_AND_FLAG",
			Report:     report,
			AuditTrail: state.AuditTrail,
		}

		if err := workflow.ExecuteActivity(ctx, ExecuteFraudActionActivity, actionInput).Get(ctx, nil); err != nil {
			logger.Error("Action execution failed", "error", err)
		}
	}

	// Mark investigation complete
	state.Complete(state.FinalDecision)

	// Calculate processing time
	processingTime := time.Since(state.StartedAt)

	return &models.InvestigationResult{
		InvestigationID: state.ID,
		RiskScore:       riskAssessment.Score,
		RiskLevel:       riskAssessment.Level,
		Decision:        state.FinalDecision,
		Report:          report,
		AuditTrail:      state.AuditTrail,
		ProcessingTime:  processingTime,
	}, nil
}

// determineFinalDecision determines the final decision based on state
func determineFinalDecision(state *models.InvestigationState) string {
	// If human made decision, use that
	if state.HumanDecision != nil {
		switch state.HumanDecision.Action {
		case "APPROVE":
			return "APPROVED"
		case "DENY":
			return "FRAUD_CONFIRMED"
		case "ESCALATE":
			return "ESCALATED"
		}
	}

	// Automatic decision based on risk
	if state.RiskAssessment != nil {
		switch state.RiskAssessment.Level {
		case models.RiskCritical:
			return "DENY_CLAIM"
		case models.RiskHigh:
			return "REQUIRES_REVIEW"
		case models.RiskMedium:
			return "APPROVED_WITH_AUDIT"
		case models.RiskLow:
			return "APPROVED"
		}
	}

	return "PENDING_REVIEW"
}

// ActionInput contains input for fraud action execution
type ActionInput struct {
	ClaimID    string                     `json:"claim_id"`
	Action     string                     `json:"action"`
	Report     *models.InvestigationReport `json:"report"`
	AuditTrail []models.AuditEntry        `json:"audit_trail"`
}

// QuickInvestigationWorkflow is a simplified workflow for low-priority cases
func QuickInvestigationWorkflow(ctx workflow.Context, input *models.FraudAlertInput) (*models.InvestigationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting quick investigation workflow", "claimID", input.ClaimID)

	state := models.NewInvestigationState(input)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Simplified flow: just data enrichment and pattern analysis
	var enrichedData *models.EnrichedData
	workflow.ExecuteActivity(ctx, DataEnrichmentActivity, input).Get(ctx, &enrichedData)
	state.EnrichedData = enrichedData

	analysisCtx := &agents.AnalysisContext{
		AlertInput:   input,
		EnrichedData: enrichedData,
	}

	findings := &models.AgentFindings{}
	workflow.ExecuteActivity(ctx, PatternAnalysisActivity, analysisCtx).Get(ctx, &findings.Pattern)
	workflow.ExecuteActivity(ctx, PolicyComplianceActivity, analysisCtx).Get(ctx, &findings.Policy)
	state.Findings = findings

	// Quick risk assessment
	riskInput := &agents.RiskInput{State: state, Findings: findings}
	var riskAssessment *models.RiskAssessment
	workflow.ExecuteActivity(ctx, RiskAssessmentActivity, riskInput).Get(ctx, &riskAssessment)
	state.RiskAssessment = riskAssessment

	// Auto-decision for quick workflow
	if riskAssessment.Level == models.RiskLow {
		state.FinalDecision = "APPROVED"
	} else {
		state.FinalDecision = "ESCALATE_TO_FULL"
	}

	state.Complete(state.FinalDecision)

	return &models.InvestigationResult{
		InvestigationID: state.ID,
		RiskScore:       riskAssessment.Score,
		RiskLevel:       riskAssessment.Level,
		Decision:        state.FinalDecision,
		AuditTrail:      state.AuditTrail,
		ProcessingTime:  time.Since(state.StartedAt),
	}, nil
}

// BatchInvestigationWorkflow processes multiple alerts in batch
func BatchInvestigationWorkflow(ctx workflow.Context, inputs []*models.FraudAlertInput) ([]*models.InvestigationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting batch investigation workflow", "count", len(inputs))

	results := make([]*models.InvestigationResult, len(inputs))

	// Process in parallel using child workflows
	var futures []workflow.ChildWorkflowFuture
	for _, input := range inputs {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "investigation-" + input.ID,
		})

		future := workflow.ExecuteChildWorkflow(childCtx, FraudInvestigationWorkflow, input)
		futures = append(futures, future)
	}

	// Collect results
	for i, future := range futures {
		var result *models.InvestigationResult
		if err := future.Get(ctx, &result); err != nil {
			logger.Error("Child workflow failed", "index", i, "error", err)
			continue
		}
		results[i] = result
	}

	return results, nil
}

// SignalHumanDecision sends a human decision signal to a running workflow
func SignalHumanDecision(ctx workflow.Context, workflowID string, decision *models.HumanDecision) error {
	return workflow.SignalExternalWorkflow(ctx, workflowID, "", HumanReviewSignal, decision).Get(ctx, nil)
}

// WorkflowQuery definitions
const (
	QueryInvestigationStatus = "status"
	QueryRiskScore           = "risk_score"
)

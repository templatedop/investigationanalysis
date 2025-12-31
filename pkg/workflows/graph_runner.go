// Package workflows provides workflow definitions for fraud investigation
package workflows

import (
	"context"
	"time"

	"github.com/investigationanalysis/pkg/agents"
	"github.com/investigationanalysis/pkg/config"
	"github.com/investigationanalysis/pkg/graph"
	"github.com/investigationanalysis/pkg/models"
	"github.com/investigationanalysis/pkg/observability/logging"
	"github.com/investigationanalysis/pkg/observability/metrics"
)

// GraphWorkflowRunner executes fraud investigation using the graph-based execution model
type GraphWorkflowRunner struct {
	config       *config.Config
	agentFactory *agents.Factory
	logger       *logging.Logger
	metrics      *metrics.InvestigationMetrics
	graph        *graph.CompiledGraph
}

// NewGraphWorkflowRunner creates a new graph-based workflow runner
func NewGraphWorkflowRunner(cfg *config.Config, factory *agents.Factory, logger *logging.Logger, m *metrics.InvestigationMetrics) (*GraphWorkflowRunner, error) {
	runner := &GraphWorkflowRunner{
		config:       cfg,
		agentFactory: factory,
		logger:       logger,
		metrics:      m,
	}

	// Build the investigation graph
	compiledGraph, err := runner.buildInvestigationGraph()
	if err != nil {
		return nil, err
	}
	runner.graph = compiledGraph

	return runner, nil
}

// RunInvestigation executes the investigation using graph-based execution
func (r *GraphWorkflowRunner) RunInvestigation(ctx context.Context, input *models.FraudAlertInput) (*models.InvestigationResult, error) {
	startTime := time.Now()
	invLogger := logging.NewInvestigationLogger(input.ID, input.ClaimID)
	invLogger.Info("Starting graph-based investigation")

	// Initialize graph state with input
	graphInput := map[string]interface{}{
		"alert_input":      input,
		"claim_id":         input.ClaimID,
		"customer_id":      input.CustomerID,
		"investigation_id": input.ID,
		"start_time":       startTime,
	}

	// Execute the graph
	state, err := r.graph.Invoke(ctx, graphInput)
	if err != nil {
		invLogger.Error("Graph execution failed", map[string]interface{}{"error": err.Error()})
		r.metrics.RecordInvestigation("failed", time.Since(startTime))
		return nil, err
	}

	// Extract result from state
	result := r.extractResult(state, input, startTime)

	// Record metrics
	r.metrics.RecordInvestigation(string(result.Decision), time.Since(startTime))
	r.metrics.RecordRiskScore(result.RiskScore, string(result.RiskLevel))

	invLogger.Info("Investigation completed", map[string]interface{}{
		"decision":   result.Decision,
		"risk_score": result.RiskScore,
		"duration":   time.Since(startTime).String(),
	})

	return result, nil
}

// buildInvestigationGraph builds the complete investigation graph
func (r *GraphWorkflowRunner) buildInvestigationGraph() (*graph.CompiledGraph, error) {
	sg := graph.NewStateGraph("fraud_investigation")

	// ====== PHASE 1: Data Gathering (Parallel) ======
	sg.AddNode("data_enrichment", r.createDataEnrichmentNode())
	sg.AddNode("entity_resolution", r.createEntityResolutionNode())

	// Parallel node for data gathering
	dataGatheringNode := graph.NewParallelNode("data_gathering",
		graph.NewFunctionNode("data_enrichment_exec", r.createDataEnrichmentNode()),
		graph.NewFunctionNode("entity_resolution_exec", r.createEntityResolutionNode()),
	)
	sg.AddNode("data_gathering", func(ctx context.Context, state *graph.State) (*graph.State, error) {
		return dataGatheringNode.Execute(ctx, state)
	})

	// ====== PHASE 2: Analysis (Parallel) ======
	analysisNodes := []graph.Node{
		graph.NewFunctionNode("pattern_analysis_exec", r.createPatternAnalysisNode()),
		graph.NewFunctionNode("network_analysis_exec", r.createNetworkAnalysisNode()),
		graph.NewFunctionNode("temporal_analysis_exec", r.createTemporalAnalysisNode()),
		graph.NewFunctionNode("document_analysis_exec", r.createDocumentAnalysisNode()),
		graph.NewFunctionNode("policy_compliance_exec", r.createPolicyComplianceNode()),
	}
	analysisParallel := graph.NewParallelNode("analysis_parallel", analysisNodes...)
	sg.AddNode("analysis", func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.AddMessage(graph.RoleSystem, "Phase 2: Running analysis agents")
		return analysisParallel.Execute(ctx, state)
	})

	// ====== PHASE 3: Risk Assessment ======
	sg.AddNode("risk_assessment", r.createRiskAssessmentNode())

	// ====== PHASE 4: Decision Router ======
	sg.AddNode("decision_router", r.createDecisionRouterNode())

	// ====== PHASE 5: Human Review (Conditional) ======
	sg.AddNode("human_review", r.createHumanReviewNode())

	// ====== PHASE 6: Evidence Compilation ======
	sg.AddNode("evidence_compilation", r.createEvidenceCompilationNode())

	// ====== PHASE 7: Explanation Generation ======
	sg.AddNode("explanation", r.createExplanationNode())

	// ====== PHASE 8: Final Output ======
	sg.AddNode("finalize", r.createFinalizeNode())

	// Set entry point and end nodes
	sg.SetEntryPoint("data_gathering")
	sg.SetFinishPoint("finalize")

	// Define edges
	sg.AddEdge("data_gathering", "analysis")
	sg.AddEdge("analysis", "risk_assessment")
	sg.AddEdge("risk_assessment", "decision_router")

	// Conditional edges from decision router
	sg.AddConditionalEdges("decision_router",
		func(state *graph.State) string {
			if state.GetBool("requires_human_review") {
				return "human"
			}
			return "auto"
		},
		map[string]string{
			"human": "human_review",
			"auto":  "evidence_compilation",
		},
	)

	sg.AddEdge("human_review", "evidence_compilation")
	sg.AddEdge("evidence_compilation", "explanation")
	sg.AddEdge("explanation", "finalize")

	return sg.Compile()
}

// createDataEnrichmentNode creates the data enrichment node function
func (r *GraphWorkflowRunner) createDataEnrichmentNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()
		state.AddMessage(graph.RoleSystem, "Running data enrichment")

		input, _ := state.Get("alert_input")
		alertInput := input.(*models.FraudAlertInput)

		// Execute data enrichment agent
		agent := r.agentFactory.GetAgent("data_enrichment")
		if agent != nil {
			result, err := agent.Execute(ctx, alertInput)
			if err != nil {
				r.logger.Error("Data enrichment failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("enriched_data", result)
				state.SetNodeOutput("data_enrichment", result)
			}
		}

		r.metrics.RecordAgentExecution("data_enrichment", "success", time.Since(start))
		return state, nil
	}
}

// createEntityResolutionNode creates the entity resolution node function
func (r *GraphWorkflowRunner) createEntityResolutionNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()
		state.AddMessage(graph.RoleSystem, "Running entity resolution")

		input, _ := state.Get("alert_input")
		alertInput := input.(*models.FraudAlertInput)

		agent := r.agentFactory.GetAgent("entity_resolution")
		if agent != nil {
			result, err := agent.Execute(ctx, alertInput)
			if err != nil {
				r.logger.Error("Entity resolution failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("entity_links", result)
				state.SetNodeOutput("entity_resolution", result)
			}
		}

		r.metrics.RecordAgentExecution("entity_resolution", "success", time.Since(start))
		return state, nil
	}
}

// createPatternAnalysisNode creates the pattern analysis node function
func (r *GraphWorkflowRunner) createPatternAnalysisNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()

		agent := r.agentFactory.GetAgent("pattern_analysis")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Pattern analysis failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("pattern_findings", result)
				state.SetNodeOutput("pattern_analysis", result)
			}
		}

		r.metrics.RecordAgentExecution("pattern_analysis", "success", time.Since(start))
		return state, nil
	}
}

// createNetworkAnalysisNode creates the network analysis node function
func (r *GraphWorkflowRunner) createNetworkAnalysisNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()

		agent := r.agentFactory.GetAgent("network_analysis")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Network analysis failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("network_findings", result)
				state.SetNodeOutput("network_analysis", result)
			}
		}

		r.metrics.RecordAgentExecution("network_analysis", "success", time.Since(start))
		return state, nil
	}
}

// createTemporalAnalysisNode creates the temporal analysis node function
func (r *GraphWorkflowRunner) createTemporalAnalysisNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()

		agent := r.agentFactory.GetAgent("temporal_analysis")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Temporal analysis failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("temporal_findings", result)
				state.SetNodeOutput("temporal_analysis", result)
			}
		}

		r.metrics.RecordAgentExecution("temporal_analysis", "success", time.Since(start))
		return state, nil
	}
}

// createDocumentAnalysisNode creates the document analysis node function
func (r *GraphWorkflowRunner) createDocumentAnalysisNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()

		agent := r.agentFactory.GetAgent("document_analysis")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Document analysis failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("document_findings", result)
				state.SetNodeOutput("document_analysis", result)
			}
		}

		r.metrics.RecordAgentExecution("document_analysis", "success", time.Since(start))
		return state, nil
	}
}

// createPolicyComplianceNode creates the policy compliance node function
func (r *GraphWorkflowRunner) createPolicyComplianceNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()

		agent := r.agentFactory.GetAgent("policy_compliance")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Policy compliance failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("policy_findings", result)
				state.SetNodeOutput("policy_compliance", result)
			}
		}

		r.metrics.RecordAgentExecution("policy_compliance", "success", time.Since(start))
		return state, nil
	}
}

// createRiskAssessmentNode creates the risk assessment node function
func (r *GraphWorkflowRunner) createRiskAssessmentNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()
		state.AddMessage(graph.RoleSystem, "Phase 3: Running risk assessment")

		agent := r.agentFactory.GetAgent("risk_assessment")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Risk assessment failed", map[string]interface{}{"error": err.Error()})
				// Default to medium risk
				state.Set("risk_score", 0.5)
				state.Set("risk_level", "medium")
			} else {
				if assessment, ok := result.(*models.RiskAssessment); ok {
					state.Set("risk_score", assessment.Score)
					state.Set("risk_level", string(assessment.Level))
					state.Set("requires_human_review", assessment.RequiresHumanReview)
					state.Set("recommended_action", assessment.RecommendedAction)
					state.SetNodeOutput("risk_assessment", assessment)
				}
			}
		}

		r.metrics.RecordAgentExecution("risk_assessment", "success", time.Since(start))
		return state, nil
	}
}

// createDecisionRouterNode creates the decision router node function
func (r *GraphWorkflowRunner) createDecisionRouterNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.AddMessage(graph.RoleSystem, "Phase 4: Decision routing")

		riskScore := state.GetFloat("risk_score")
		riskLevel := state.GetString("risk_level")

		// Determine if human review is needed
		needsHumanReview := state.GetBool("requires_human_review")
		if !needsHumanReview {
			// Auto-determine based on thresholds
			needsHumanReview = riskScore >= r.config.Agents.EscalationRules.HumanReviewThreshold
		}

		// Check claim amount
		if input, ok := state.Get("alert_input"); ok {
			if alertInput, ok := input.(*models.FraudAlertInput); ok {
				if alertInput.ClaimAmount > r.config.Agents.EscalationRules.MaxAutoDecisionAmount {
					needsHumanReview = true
				}
			}
		}

		state.Set("requires_human_review", needsHumanReview)

		r.logger.Info("Decision routing complete", map[string]interface{}{
			"risk_score":    riskScore,
			"risk_level":    riskLevel,
			"human_review":  needsHumanReview,
		})

		return state, nil
	}
}

// createHumanReviewNode creates the human review node function
func (r *GraphWorkflowRunner) createHumanReviewNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.AddMessage(graph.RoleSystem, "Waiting for human review")
		r.logger.Info("Investigation escalated to human review")

		// In a real implementation, this would:
		// 1. Create a checkpoint
		// 2. Notify human reviewers
		// 3. Wait for response or timeout
		// For now, we simulate auto-approval after timeout

		// Check for existing human decision
		if decision, ok := state.Get("human_decision"); ok {
			r.logger.Info("Human decision received", map[string]interface{}{"decision": decision})
		} else {
			// Timeout case - auto escalate
			state.Set("human_decision", "AUTO_ESCALATE")
			state.Set("human_decision_reason", "Review timeout exceeded")
		}

		return state, nil
	}
}

// createEvidenceCompilationNode creates the evidence compilation node function
func (r *GraphWorkflowRunner) createEvidenceCompilationNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()
		state.AddMessage(graph.RoleSystem, "Phase 5: Compiling evidence")

		agent := r.agentFactory.GetAgent("evidence_compilation")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Evidence compilation failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("evidence_report", result)
				state.SetNodeOutput("evidence_compilation", result)
			}
		}

		r.metrics.RecordAgentExecution("evidence_compilation", "success", time.Since(start))
		return state, nil
	}
}

// createExplanationNode creates the explanation node function
func (r *GraphWorkflowRunner) createExplanationNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		start := time.Now()
		state.AddMessage(graph.RoleSystem, "Phase 6: Generating explanation")

		agent := r.agentFactory.GetAgent("explanation")
		if agent != nil {
			result, err := agent.Execute(ctx, state.Data)
			if err != nil {
				r.logger.Error("Explanation generation failed", map[string]interface{}{"error": err.Error()})
			} else {
				state.Set("explanation", result)
				state.SetNodeOutput("explanation", result)
			}
		}

		r.metrics.RecordAgentExecution("explanation", "success", time.Since(start))
		return state, nil
	}
}

// createFinalizeNode creates the finalize node function
func (r *GraphWorkflowRunner) createFinalizeNode() graph.NodeFunc {
	return func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.AddMessage(graph.RoleSystem, "Finalizing investigation")

		// Determine final decision
		riskLevel := state.GetString("risk_level")
		humanDecision := state.GetString("human_decision")

		var finalDecision string
		if humanDecision != "" {
			switch humanDecision {
			case "APPROVE":
				finalDecision = "APPROVED"
			case "DENY":
				finalDecision = "FRAUD_CONFIRMED"
			case "ESCALATE":
				finalDecision = "ESCALATED"
			default:
				finalDecision = "PENDING_REVIEW"
			}
		} else {
			switch riskLevel {
			case "critical":
				finalDecision = "DENY_CLAIM"
			case "high":
				finalDecision = "REQUIRES_REVIEW"
			case "medium":
				finalDecision = "APPROVED_WITH_AUDIT"
			case "low":
				finalDecision = "APPROVED"
			default:
				finalDecision = "PENDING_REVIEW"
			}
		}

		state.Set("final_decision", finalDecision)
		state.Status = graph.StatusCompleted

		r.logger.Info("Investigation finalized", map[string]interface{}{
			"final_decision": finalDecision,
		})

		return state, nil
	}
}

// extractResult extracts the investigation result from the graph state
func (r *GraphWorkflowRunner) extractResult(state *graph.State, input *models.FraudAlertInput, startTime time.Time) *models.InvestigationResult {
	result := &models.InvestigationResult{
		InvestigationID: input.ID,
		RiskScore:       state.GetFloat("risk_score"),
		Decision:        state.GetString("final_decision"),
		ProcessingTime:  time.Since(startTime),
	}

	// Set risk level
	switch state.GetString("risk_level") {
	case "critical":
		result.RiskLevel = models.RiskCritical
	case "high":
		result.RiskLevel = models.RiskHigh
	case "medium":
		result.RiskLevel = models.RiskMedium
	case "low":
		result.RiskLevel = models.RiskLow
	default:
		result.RiskLevel = models.RiskMedium
	}

	// Extract report if available
	if report, ok := state.Get("evidence_report"); ok {
		if r, ok := report.(*models.InvestigationReport); ok {
			result.Report = r
		}
	}

	// Build audit trail from state history
	for _, transition := range state.History {
		result.AuditTrail = append(result.AuditTrail, models.AuditEntry{
			Timestamp: transition.Timestamp,
			Type:      "transition",
			AgentName: transition.From,
			Details:   "Transitioned to " + transition.To,
		})
	}

	return result
}

// StreamInvestigation executes the investigation and streams updates
func (r *GraphWorkflowRunner) StreamInvestigation(ctx context.Context, input *models.FraudAlertInput) (<-chan *graph.State, <-chan error) {
	graphInput := map[string]interface{}{
		"alert_input":      input,
		"claim_id":         input.ClaimID,
		"customer_id":      input.CustomerID,
		"investigation_id": input.ID,
		"start_time":       time.Now(),
	}

	return r.graph.Stream(ctx, graphInput)
}

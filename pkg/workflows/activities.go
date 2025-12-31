// Package workflows provides Temporal activity implementations
package workflows

import (
	"context"

	"github.com/fraudinvestigation/rag-framework/pkg/agents"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

// Activities holds all activity implementations
type Activities struct {
	dataEnrichmentAgent   *agents.DataEnrichmentAgent
	entityResolutionAgent *agents.EntityResolutionAgent
	patternAnalysisAgent  *agents.PatternAnalysisAgent
	networkAnalysisAgent  *agents.NetworkAnalysisAgent
	temporalAnalysisAgent *agents.TemporalAnalysisAgent
	documentAnalysisAgent *agents.DocumentAnalysisAgent
	policyComplianceAgent *agents.PolicyComplianceAgent
	riskAssessmentAgent   *agents.RiskAssessmentAgent
	evidenceAgent         *agents.EvidenceCompilationAgent
	explanationAgent      *agents.ExplanationAgent
}

// NewActivities creates a new Activities instance
func NewActivities(
	dataEnrichment *agents.DataEnrichmentAgent,
	entityResolution *agents.EntityResolutionAgent,
	patternAnalysis *agents.PatternAnalysisAgent,
	networkAnalysis *agents.NetworkAnalysisAgent,
	temporalAnalysis *agents.TemporalAnalysisAgent,
	documentAnalysis *agents.DocumentAnalysisAgent,
	policyCompliance *agents.PolicyComplianceAgent,
	riskAssessment *agents.RiskAssessmentAgent,
	evidence *agents.EvidenceCompilationAgent,
	explanation *agents.ExplanationAgent,
) *Activities {
	return &Activities{
		dataEnrichmentAgent:   dataEnrichment,
		entityResolutionAgent: entityResolution,
		patternAnalysisAgent:  patternAnalysis,
		networkAnalysisAgent:  networkAnalysis,
		temporalAnalysisAgent: temporalAnalysis,
		documentAnalysisAgent: documentAnalysis,
		policyComplianceAgent: policyCompliance,
		riskAssessmentAgent:   riskAssessment,
		evidenceAgent:         evidence,
		explanationAgent:      explanation,
	}
}

// DataEnrichmentActivity enriches data for investigation
func (a *Activities) DataEnrichmentActivity(ctx context.Context, input *models.FraudAlertInput) (*models.EnrichedData, error) {
	finding, err := a.dataEnrichmentAgent.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	data, ok := agents.GetEnrichedData(finding)
	if !ok {
		return &models.EnrichedData{}, nil
	}

	return data, nil
}

// EntityResolutionActivity resolves entity relationships
func (a *Activities) EntityResolutionActivity(ctx context.Context, input *models.FraudAlertInput) (*models.EntityLinks, error) {
	analysisCtx := &agents.AnalysisContext{
		AlertInput: input,
	}

	finding, err := a.entityResolutionAgent.Execute(ctx, analysisCtx)
	if err != nil {
		return nil, err
	}

	if finding.Metadata == nil {
		return &models.EntityLinks{}, nil
	}

	links, ok := finding.Metadata["entity_links"].(*models.EntityLinks)
	if !ok {
		return &models.EntityLinks{}, nil
	}

	return links, nil
}

// PatternAnalysisActivity performs pattern analysis
func (a *Activities) PatternAnalysisActivity(ctx context.Context, analysisCtx *agents.AnalysisContext) (*models.AgentFinding, error) {
	return a.patternAnalysisAgent.Execute(ctx, analysisCtx)
}

// NetworkAnalysisActivity performs network analysis
func (a *Activities) NetworkAnalysisActivity(ctx context.Context, analysisCtx *agents.AnalysisContext) (*models.AgentFinding, error) {
	return a.networkAnalysisAgent.Execute(ctx, analysisCtx)
}

// TemporalAnalysisActivity performs temporal analysis
func (a *Activities) TemporalAnalysisActivity(ctx context.Context, analysisCtx *agents.AnalysisContext) (*models.AgentFinding, error) {
	return a.temporalAnalysisAgent.Execute(ctx, analysisCtx)
}

// DocumentAnalysisActivity performs document analysis
func (a *Activities) DocumentAnalysisActivity(ctx context.Context, analysisCtx *agents.AnalysisContext) (*models.AgentFinding, error) {
	return a.documentAnalysisAgent.Execute(ctx, analysisCtx)
}

// PolicyComplianceActivity performs policy compliance checks
func (a *Activities) PolicyComplianceActivity(ctx context.Context, analysisCtx *agents.AnalysisContext) (*models.AgentFinding, error) {
	return a.policyComplianceAgent.Execute(ctx, analysisCtx)
}

// RiskAssessmentActivity performs risk assessment
func (a *Activities) RiskAssessmentActivity(ctx context.Context, riskInput *agents.RiskInput) (*models.RiskAssessment, error) {
	finding, err := a.riskAssessmentAgent.Execute(ctx, riskInput)
	if err != nil {
		return nil, err
	}

	if finding.Metadata == nil {
		return &models.RiskAssessment{}, nil
	}

	assessment, ok := finding.Metadata["risk_assessment"].(*models.RiskAssessment)
	if !ok {
		return &models.RiskAssessment{}, nil
	}

	return assessment, nil
}

// EvidenceCompilationActivity compiles investigation evidence
func (a *Activities) EvidenceCompilationActivity(ctx context.Context, state *models.InvestigationState) (*models.InvestigationReport, error) {
	finding, err := a.evidenceAgent.Execute(ctx, state)
	if err != nil {
		return nil, err
	}

	report, ok := agents.GetReport(finding)
	if !ok {
		return &models.InvestigationReport{}, nil
	}

	return report, nil
}

// ExplanationActivity generates explanations
func (a *Activities) ExplanationActivity(ctx context.Context, input *agents.ExplanationInput) (*agents.HumanReadableExplanation, error) {
	finding, err := a.explanationAgent.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	explanation, ok := agents.GetExplanation(finding)
	if !ok {
		return &agents.HumanReadableExplanation{}, nil
	}

	return explanation, nil
}

// ExecuteFraudActionActivity executes actions for confirmed fraud
func (a *Activities) ExecuteFraudActionActivity(ctx context.Context, input *ActionInput) error {
	// Implementation would:
	// 1. Update claim status in database
	// 2. Notify relevant parties
	// 3. Create SIU referral if needed
	// 4. Store investigation report
	// 5. Log for compliance

	return nil
}

// Activity function references for registration
var (
	DataEnrichmentActivity    = (*Activities).DataEnrichmentActivity
	EntityResolutionActivity  = (*Activities).EntityResolutionActivity
	PatternAnalysisActivity   = (*Activities).PatternAnalysisActivity
	NetworkAnalysisActivity   = (*Activities).NetworkAnalysisActivity
	TemporalAnalysisActivity  = (*Activities).TemporalAnalysisActivity
	DocumentAnalysisActivity  = (*Activities).DocumentAnalysisActivity
	PolicyComplianceActivity  = (*Activities).PolicyComplianceActivity
	RiskAssessmentActivity    = (*Activities).RiskAssessmentActivity
	EvidenceCompilationActivity = (*Activities).EvidenceCompilationActivity
	ExplanationActivity       = (*Activities).ExplanationActivity
	ExecuteFraudActionActivity = (*Activities).ExecuteFraudActionActivity
)

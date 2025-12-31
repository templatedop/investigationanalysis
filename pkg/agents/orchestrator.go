// Package agents provides the orchestrator agent for coordinating investigations
package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// OrchestratorAgent coordinates all other agents in an investigation
type OrchestratorAgent struct {
	*BaseAgent
	escalationRules *EscalationConfig
	registry        *AgentRegistry
}

// EscalationConfig defines escalation rules
type EscalationConfig struct {
	AutoEscalateThreshold float64 `json:"auto_escalate_threshold"`
	HumanReviewThreshold  float64 `json:"human_review_threshold"`
	MaxAutoDecisionAmount float64 `json:"max_auto_decision_amount"`
	RequiredConfidence    float64 `json:"required_confidence"`
}

// DefaultEscalationConfig returns default escalation settings
func DefaultEscalationConfig() *EscalationConfig {
	return &EscalationConfig{
		AutoEscalateThreshold: 0.9,
		HumanReviewThreshold:  0.7,
		MaxAutoDecisionAmount: 100000,
		RequiredConfidence:    0.85,
	}
}

// NewOrchestratorAgent creates a new orchestrator agent
func NewOrchestratorAgent(llmClient *llm.Client, ragSystem *rag.System, registry *AgentRegistry) *OrchestratorAgent {
	return &OrchestratorAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeOrchestrator,
			"Investigation Orchestrator",
			"Coordinates all agents and manages investigation workflow",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "fast",
			},
		),
		escalationRules: DefaultEscalationConfig(),
		registry:        registry,
	}
}

// SetEscalationConfig sets custom escalation rules
func (o *OrchestratorAgent) SetEscalationConfig(cfg *EscalationConfig) {
	o.escalationRules = cfg
}

// Execute orchestrates the investigation
func (o *OrchestratorAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	alertInput, ok := input.(*models.FraudAlertInput)
	if !ok {
		return nil, fmt.Errorf("expected *models.FraudAlertInput, got %T", input)
	}

	// Determine investigation strategy
	strategy, err := o.planInvestigation(ctx, alertInput)
	if err != nil {
		return nil, err
	}

	finding := o.CreateFinding(0.95, nil, nil, nil)
	finding.Metadata["strategy"] = strategy
	finding.Metadata["planned_agents"] = strategy.AgentSequence
	finding.Recommendations = strategy.Recommendations

	return finding, nil
}

// InvestigationStrategy defines how to investigate an alert
type InvestigationStrategy struct {
	Priority          int                  `json:"priority"`
	AgentSequence     []models.AgentType   `json:"agent_sequence"`
	ParallelAgents    [][]models.AgentType `json:"parallel_agents"`
	EstimatedDuration string               `json:"estimated_duration"`
	Recommendations   []string             `json:"recommendations"`
	RiskCategory      string               `json:"risk_category"`
}

// planInvestigation creates an investigation plan based on the alert
func (o *OrchestratorAgent) planInvestigation(ctx context.Context, alert *models.FraudAlertInput) (*InvestigationStrategy, error) {
	systemPrompt := `You are an investigation coordinator. Your role is to analyze fraud alerts and determine the optimal investigation strategy.

Based on the alert characteristics, determine:
1. Which specialist agents to invoke
2. The optimal execution order (parallel vs sequential)
3. Priority level
4. Initial risk assessment

Always return a valid JSON response.`

	userPrompt := fmt.Sprintf(`Analyze this fraud alert and create an investigation strategy:

Alert ID: %s
Claim Type: %s
Amount: %.2f
Description: %s
Alert Triggers: %v
Priority: %d

Respond with a JSON object containing:
{
  "priority": <1-5>,
  "risk_category": "<low|medium|high|critical>",
  "agent_sequence": ["agent_type_1", "agent_type_2", ...],
  "parallel_agents": [["agent1", "agent2"], ["agent3"]],
  "estimated_duration": "<duration string>",
  "recommendations": ["recommendation1", "recommendation2"]
}`,
		alert.ID, alert.ClaimType, alert.Amount, alert.Description,
		alert.AlertTriggers, alert.Priority)

	response, _, err := o.GenerateWithRAG(ctx, systemPrompt, userPrompt, []string{
		"investigation playbook " + alert.ClaimType,
		"fraud detection strategy",
	})
	if err != nil {
		// Return default strategy on error
		return o.defaultStrategy(alert), nil
	}

	var strategy InvestigationStrategy
	if err := o.ParseJSONResponse(response, &strategy); err != nil {
		return o.defaultStrategy(alert), nil
	}

	return &strategy, nil
}

// defaultStrategy returns a default investigation strategy
func (o *OrchestratorAgent) defaultStrategy(alert *models.FraudAlertInput) *InvestigationStrategy {
	strategy := &InvestigationStrategy{
		Priority:          alert.Priority,
		EstimatedDuration: "30 minutes",
		RiskCategory:      "medium",
	}

	// Default agent sequence based on claim amount
	if alert.Amount > 50000 {
		strategy.RiskCategory = "high"
		strategy.ParallelAgents = [][]models.AgentType{
			{models.AgentTypeDataEnrichment, models.AgentTypeEntityResolution},
			{models.AgentTypePatternAnalysis, models.AgentTypeNetworkAnalysis, models.AgentTypeTemporalAnalysis},
			{models.AgentTypeDocumentAnalysis, models.AgentTypePolicyCompliance},
		}
		strategy.AgentSequence = []models.AgentType{
			models.AgentTypeRiskAssessment,
			models.AgentTypeEvidence,
			models.AgentTypeExplanation,
		}
	} else {
		strategy.ParallelAgents = [][]models.AgentType{
			{models.AgentTypeDataEnrichment},
			{models.AgentTypePatternAnalysis, models.AgentTypePolicyCompliance},
		}
		strategy.AgentSequence = []models.AgentType{
			models.AgentTypeRiskAssessment,
			models.AgentTypeExplanation,
		}
	}

	strategy.Recommendations = []string{
		"Gather all related transaction data",
		"Check for entity relationships",
		"Verify policy compliance",
	}

	return strategy
}

// DetermineAction decides the next action based on findings
func (o *OrchestratorAgent) DetermineAction(ctx context.Context, state *models.InvestigationState, findings *models.AgentFindings) (*ActionDecision, error) {
	riskScore := o.calculateAggregateRisk(findings)

	decision := &ActionDecision{
		RiskScore: riskScore,
	}

	// Check against escalation rules
	if riskScore >= o.escalationRules.AutoEscalateThreshold {
		decision.Action = ActionDeny
		decision.RequiresHumanReview = state.AlertInput.Amount > o.escalationRules.MaxAutoDecisionAmount
		decision.Reason = "High fraud risk score exceeds auto-escalation threshold"
	} else if riskScore >= o.escalationRules.HumanReviewThreshold {
		decision.Action = ActionReview
		decision.RequiresHumanReview = true
		decision.Reason = "Risk score requires human review"
	} else {
		decision.Action = ActionApprove
		decision.RequiresHumanReview = false
		decision.Reason = "Risk score within acceptable limits"
	}

	// Check confidence
	avgConfidence := o.calculateAverageConfidence(findings)
	if avgConfidence < o.escalationRules.RequiredConfidence {
		decision.RequiresHumanReview = true
		decision.Reason += " (low confidence)"
	}

	return decision, nil
}

// ActionType represents possible actions
type ActionType string

const (
	ActionApprove   ActionType = "approve"
	ActionDeny      ActionType = "deny"
	ActionReview    ActionType = "review"
	ActionEscalate  ActionType = "escalate"
	ActionInvestigate ActionType = "investigate_further"
)

// ActionDecision contains the orchestrator's decision
type ActionDecision struct {
	Action            ActionType `json:"action"`
	RiskScore         float64    `json:"risk_score"`
	RequiresHumanReview bool     `json:"requires_human_review"`
	Reason            string     `json:"reason"`
	NextSteps         []string   `json:"next_steps"`
}

// calculateAggregateRisk computes overall risk from agent findings
func (o *OrchestratorAgent) calculateAggregateRisk(findings *models.AgentFindings) float64 {
	weights := map[models.AgentType]float64{
		models.AgentTypePatternAnalysis:  0.25,
		models.AgentTypeNetworkAnalysis:  0.20,
		models.AgentTypeTemporalAnalysis: 0.15,
		models.AgentTypeDocumentAnalysis: 0.15,
		models.AgentTypePolicyCompliance: 0.15,
		models.AgentTypeEntityResolution: 0.10,
	}

	var totalWeight, weightedSum float64

	if findings.Pattern != nil {
		w := weights[models.AgentTypePatternAnalysis]
		weightedSum += o.findingToRiskScore(findings.Pattern) * w
		totalWeight += w
	}
	if findings.Network != nil {
		w := weights[models.AgentTypeNetworkAnalysis]
		weightedSum += o.findingToRiskScore(findings.Network) * w
		totalWeight += w
	}
	if findings.Temporal != nil {
		w := weights[models.AgentTypeTemporalAnalysis]
		weightedSum += o.findingToRiskScore(findings.Temporal) * w
		totalWeight += w
	}
	if findings.Document != nil {
		w := weights[models.AgentTypeDocumentAnalysis]
		weightedSum += o.findingToRiskScore(findings.Document) * w
		totalWeight += w
	}
	if findings.Policy != nil {
		w := weights[models.AgentTypePolicyCompliance]
		weightedSum += o.findingToRiskScore(findings.Policy) * w
		totalWeight += w
	}
	if findings.Entity != nil {
		w := weights[models.AgentTypeEntityResolution]
		weightedSum += o.findingToRiskScore(findings.Entity) * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5 // Default medium risk
	}

	return weightedSum / totalWeight
}

// findingToRiskScore converts a finding to a risk score
func (o *OrchestratorAgent) findingToRiskScore(finding *models.AgentFinding) float64 {
	if finding == nil {
		return 0.5
	}

	// Base on indicator severity
	var totalSeverity float64
	for _, indicator := range finding.RiskIndicators {
		switch indicator.Severity {
		case "critical":
			totalSeverity += 1.0
		case "high":
			totalSeverity += 0.75
		case "medium":
			totalSeverity += 0.5
		case "low":
			totalSeverity += 0.25
		}
	}

	if len(finding.RiskIndicators) == 0 {
		return 0.3 // Low risk if no indicators
	}

	avgSeverity := totalSeverity / float64(len(finding.RiskIndicators))
	return avgSeverity * finding.Confidence
}

// calculateAverageConfidence computes average confidence across findings
func (o *OrchestratorAgent) calculateAverageConfidence(findings *models.AgentFindings) float64 {
	var total float64
	var count int

	check := func(f *models.AgentFinding) {
		if f != nil {
			total += f.Confidence
			count++
		}
	}

	check(findings.Pattern)
	check(findings.Network)
	check(findings.Temporal)
	check(findings.Document)
	check(findings.Policy)
	check(findings.Entity)

	if count == 0 {
		return 0.5
	}

	return total / float64(count)
}

// ShouldContinueInvestigation determines if more investigation is needed
func (o *OrchestratorAgent) ShouldContinueInvestigation(findings *models.AgentFindings) bool {
	// Check if we have enough confident findings
	confidenceThreshold := 0.7
	requiredFindings := 3

	count := 0
	if findings.Pattern != nil && findings.Pattern.Confidence >= confidenceThreshold {
		count++
	}
	if findings.Network != nil && findings.Network.Confidence >= confidenceThreshold {
		count++
	}
	if findings.Temporal != nil && findings.Temporal.Confidence >= confidenceThreshold {
		count++
	}
	if findings.Document != nil && findings.Document.Confidence >= confidenceThreshold {
		count++
	}
	if findings.Policy != nil && findings.Policy.Confidence >= confidenceThreshold {
		count++
	}

	return count < requiredFindings
}

// GetNextAgents returns the next agents to run based on current findings
func (o *OrchestratorAgent) GetNextAgents(currentFindings *models.AgentFindings) []models.AgentType {
	var next []models.AgentType

	// If pattern analysis found anomalies, run network and temporal
	if currentFindings.Pattern != nil && len(currentFindings.Pattern.RiskIndicators) > 0 {
		if currentFindings.Network == nil {
			next = append(next, models.AgentTypeNetworkAnalysis)
		}
		if currentFindings.Temporal == nil {
			next = append(next, models.AgentTypeTemporalAnalysis)
		}
	}

	// If entity resolution found links, run network analysis
	if currentFindings.Entity != nil && currentFindings.Network == nil {
		next = append(next, models.AgentTypeNetworkAnalysis)
	}

	// Always run policy compliance if not done
	if currentFindings.Policy == nil {
		next = append(next, models.AgentTypePolicyCompliance)
	}

	return next
}

// MarshalJSON implements custom JSON marshaling
func (s *InvestigationStrategy) MarshalJSON() ([]byte, error) {
	type Alias InvestigationStrategy
	return json.Marshal(&struct {
		*Alias
		AgentSequenceStr  []string   `json:"agent_sequence"`
		ParallelAgentsStr [][]string `json:"parallel_agents"`
	}{
		Alias:             (*Alias)(s),
		AgentSequenceStr:  agentTypesToStrings(s.AgentSequence),
		ParallelAgentsStr: agentTypesListToStrings(s.ParallelAgents),
	})
}

func agentTypesToStrings(types []models.AgentType) []string {
	result := make([]string, len(types))
	for i, t := range types {
		result[i] = string(t)
	}
	return result
}

func agentTypesListToStrings(typesList [][]models.AgentType) [][]string {
	result := make([][]string, len(typesList))
	for i, types := range typesList {
		result[i] = agentTypesToStrings(types)
	}
	return result
}

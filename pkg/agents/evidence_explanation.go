// Package agents provides evidence compilation and explanation agents
package agents

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// EvidenceCompilationAgent assembles investigation package for review
type EvidenceCompilationAgent struct {
	*BaseAgent
}

// NewEvidenceCompilationAgent creates a new evidence compilation agent
func NewEvidenceCompilationAgent(llmClient *llm.Client, ragSystem *rag.System) *EvidenceCompilationAgent {
	return &EvidenceCompilationAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeEvidence,
			"Evidence Compilation Agent",
			"Assembles investigation package for review and action",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium",
			},
		),
	}
}

// Execute compiles evidence into a report
func (a *EvidenceCompilationAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	state, ok := input.(*models.InvestigationState)
	if !ok {
		return nil, fmt.Errorf("expected *models.InvestigationState, got %T", input)
	}

	startTime := time.Now()

	// Build chronological timeline
	timeline := a.buildTimeline(state)

	// Build entity diagram
	entityDiagram := a.buildEntityDiagram(state)

	// Collect all evidence
	evidencePackage := a.collectEvidence(state)

	// Generate agent summaries
	agentSummaries := a.generateAgentSummaries(state)

	// Generate recommendations
	recommendations := a.generateRecommendations(state)

	// Create report
	report := &models.InvestigationReport{
		ID:              fmt.Sprintf("RPT-%s", state.ID),
		InvestigationID: state.ID,
		Summary:         a.generateSummary(state),
		Timeline:        timeline,
		EntityDiagram:   entityDiagram,
		EvidencePackage: evidencePackage,
		AgentSummaries:  agentSummaries,
		RiskAssessment:  state.RiskAssessment,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}

	// Get RAG context for report templates
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"investigation report template",
		"evidence documentation requirements",
	})

	finding := a.CreateFinding(0.95, nil, evidencePackage, sources)
	finding.Metadata["investigation_report"] = report
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// buildTimeline creates chronological event timeline
func (a *EvidenceCompilationAgent) buildTimeline(state *models.InvestigationState) []models.TimelineEvent {
	var events []models.TimelineEvent

	// Add alert event
	if state.AlertInput != nil {
		events = append(events, models.TimelineEvent{
			Timestamp:    state.AlertInput.SubmittedAt,
			Event:        "Fraud Alert Triggered",
			Description:  state.AlertInput.Description,
			Significance: "high",
		})
	}

	// Add investigation start
	events = append(events, models.TimelineEvent{
		Timestamp:    state.StartedAt,
		Event:        "Investigation Started",
		Description:  "Automated investigation workflow initiated",
		Significance: "info",
	})

	// Add audit trail events
	for _, entry := range state.AuditTrail {
		events = append(events, models.TimelineEvent{
			Timestamp:    entry.Timestamp,
			Event:        entry.Action,
			Description:  entry.Details,
			Significance: "info",
		})
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}

// buildEntityDiagram creates entity relationship visualization
func (a *EvidenceCompilationAgent) buildEntityDiagram(state *models.InvestigationState) *models.EntityDiagram {
	diagram := &models.EntityDiagram{
		Nodes: []models.EntityNode{},
		Edges: []models.EntityEdge{},
	}

	if state.EntityLinks == nil {
		return diagram
	}

	// Add nodes from entity links
	nodeMap := make(map[string]bool)

	addNode := func(id, nodeType, risk string) {
		if !nodeMap[id] {
			diagram.Nodes = append(diagram.Nodes, models.EntityNode{
				ID:    id,
				Label: id,
				Type:  nodeType,
				Risk:  risk,
			})
			nodeMap[id] = true
		}
	}

	// Add from direct matches
	for _, link := range state.EntityLinks.DirectMatches {
		addNode(link.SourceID, "entity", "")
		addNode(link.TargetID, "entity", "")
		diagram.Edges = append(diagram.Edges, models.EntityEdge{
			Source:   link.SourceID,
			Target:   link.TargetID,
			Relation: link.LinkType,
			Weight:   link.Confidence,
		})
	}

	// Add from transitive links
	for _, link := range state.EntityLinks.TransitiveLinks {
		addNode(link.SourceID, "entity", "")
		addNode(link.TargetID, "entity", "")
		diagram.Edges = append(diagram.Edges, models.EntityEdge{
			Source:   link.SourceID,
			Target:   link.TargetID,
			Relation: "transitive",
			Weight:   link.Confidence * 0.8,
		})
	}

	return diagram
}

// collectEvidence gathers all evidence from findings
func (a *EvidenceCompilationAgent) collectEvidence(state *models.InvestigationState) []models.Evidence {
	var evidence []models.Evidence

	if state.Findings == nil {
		return evidence
	}

	// Collect from each agent
	collectFromFinding := func(f *models.AgentFinding) {
		if f != nil {
			evidence = append(evidence, f.Evidence...)
		}
	}

	collectFromFinding(state.Findings.Pattern)
	collectFromFinding(state.Findings.Network)
	collectFromFinding(state.Findings.Temporal)
	collectFromFinding(state.Findings.Document)
	collectFromFinding(state.Findings.Policy)
	collectFromFinding(state.Findings.Entity)

	// Sort by confidence descending
	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].Confidence > evidence[j].Confidence
	})

	return evidence
}

// generateAgentSummaries creates summary for each agent's findings
func (a *EvidenceCompilationAgent) generateAgentSummaries(state *models.InvestigationState) map[models.AgentType]string {
	summaries := make(map[models.AgentType]string)

	if state.Findings == nil {
		return summaries
	}

	summarize := func(agentType models.AgentType, f *models.AgentFinding) {
		if f == nil {
			return
		}

		summary := fmt.Sprintf("Confidence: %.2f, Indicators: %d",
			f.Confidence, len(f.RiskIndicators))

		if len(f.RiskIndicators) > 0 {
			summary += "\nTop indicators: "
			for i, ind := range f.RiskIndicators {
				if i >= 3 {
					break
				}
				summary += fmt.Sprintf("\n- %s (%s)", ind.Description, ind.Severity)
			}
		}

		summaries[agentType] = summary
	}

	summarize(models.AgentTypePatternAnalysis, state.Findings.Pattern)
	summarize(models.AgentTypeNetworkAnalysis, state.Findings.Network)
	summarize(models.AgentTypeTemporalAnalysis, state.Findings.Temporal)
	summarize(models.AgentTypeDocumentAnalysis, state.Findings.Document)
	summarize(models.AgentTypePolicyCompliance, state.Findings.Policy)
	summarize(models.AgentTypeEntityResolution, state.Findings.Entity)

	return summaries
}

// generateRecommendations creates action recommendations
func (a *EvidenceCompilationAgent) generateRecommendations(state *models.InvestigationState) []models.Recommendation {
	var recommendations []models.Recommendation

	if state.RiskAssessment == nil {
		return recommendations
	}

	// Based on risk level
	switch state.RiskAssessment.Level {
	case models.RiskCritical:
		recommendations = append(recommendations, models.Recommendation{
			Action:    "Deny Claim",
			Priority:  1,
			Rationale: "Critical risk indicators detected",
		})
		recommendations = append(recommendations, models.Recommendation{
			Action:    "Report to SIU",
			Priority:  1,
			Rationale: "Special Investigation Unit review required",
		})
	case models.RiskHigh:
		recommendations = append(recommendations, models.Recommendation{
			Action:    "Enhanced Review",
			Priority:  2,
			Rationale: "High risk indicators require manual verification",
		})
	case models.RiskMedium:
		recommendations = append(recommendations, models.Recommendation{
			Action:    "Standard Review",
			Priority:  3,
			Rationale: "Some concerns require clarification",
		})
	case models.RiskLow:
		recommendations = append(recommendations, models.Recommendation{
			Action:    "Approve with Audit",
			Priority:  4,
			Rationale: "Low risk, can proceed with standard audit trail",
		})
	}

	return recommendations
}

// generateSummary creates executive summary
func (a *EvidenceCompilationAgent) generateSummary(state *models.InvestigationState) string {
	summary := "Investigation Summary\n\n"

	if state.AlertInput != nil {
		summary += fmt.Sprintf("Claim ID: %s\n", state.AlertInput.ClaimID)
		summary += fmt.Sprintf("Amount: %.2f\n", state.AlertInput.Amount)
		summary += fmt.Sprintf("Type: %s\n\n", state.AlertInput.ClaimType)
	}

	if state.RiskAssessment != nil {
		summary += fmt.Sprintf("Risk Level: %s\n", state.RiskAssessment.Level)
		summary += fmt.Sprintf("Risk Score: %.2f\n", state.RiskAssessment.Score)
		summary += fmt.Sprintf("Confidence: %.2f\n\n", state.RiskAssessment.Confidence)
		summary += fmt.Sprintf("Recommended Action: %s\n", state.RiskAssessment.RecommendedAction)
	}

	summary += fmt.Sprintf("\nFinal Decision: %s\n", state.FinalDecision)

	return summary
}

// ExplanationAgent generates human-readable explanations
type ExplanationAgent struct {
	*BaseAgent
	audienceProfiles map[string]AudienceProfile
}

// AudienceProfile defines explanation style for an audience
type AudienceProfile struct {
	TechnicalLevel string
	FocusAreas     []string
	Language       string
}

// NewExplanationAgent creates a new explanation agent
func NewExplanationAgent(llmClient *llm.Client, ragSystem *rag.System) *ExplanationAgent {
	return &ExplanationAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeExplanation,
			"Explanation Agent",
			"Generates human-readable explanations for different audiences",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium",
			},
		),
		audienceProfiles: map[string]AudienceProfile{
			"claims_adjuster": {
				TechnicalLevel: "high",
				FocusAreas:     []string{"evidence", "risk_indicators", "recommendations"},
				Language:       "technical",
			},
			"customer": {
				TechnicalLevel: "low",
				FocusAreas:     []string{"decision", "next_steps"},
				Language:       "simple",
			},
			"legal": {
				TechnicalLevel: "high",
				FocusAreas:     []string{"evidence", "compliance", "regulations"},
				Language:       "formal",
			},
			"management": {
				TechnicalLevel: "medium",
				FocusAreas:     []string{"summary", "risk", "impact"},
				Language:       "business",
			},
		},
	}
}

// ExplanationInput contains input for explanation generation
type ExplanationInput struct {
	Audience  string                      `json:"audience"`
	State     *models.InvestigationState  `json:"state"`
	Findings  *models.AgentFindings       `json:"findings"`
}

// Execute generates explanations
func (a *ExplanationAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	expInput, ok := input.(*ExplanationInput)
	if !ok {
		return nil, fmt.Errorf("expected *ExplanationInput, got %T", input)
	}

	startTime := time.Now()

	profile, exists := a.audienceProfiles[expInput.Audience]
	if !exists {
		profile = a.audienceProfiles["claims_adjuster"] // Default
	}

	// Generate explanation using LLM
	explanation, err := a.generateExplanation(ctx, expInput, profile)
	if err != nil {
		explanation = a.generateFallbackExplanation(expInput)
	}

	// Get RAG context
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"fraud investigation explanation guidelines",
		fmt.Sprintf("communication %s audience", expInput.Audience),
	})

	finding := a.CreateFinding(0.9, nil, nil, sources)
	finding.Metadata["explanation"] = explanation
	finding.Metadata["audience"] = expInput.Audience
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// generateExplanation uses LLM to create explanation
func (a *ExplanationAgent) generateExplanation(ctx context.Context, input *ExplanationInput, profile AudienceProfile) (*HumanReadableExplanation, error) {
	systemPrompt := fmt.Sprintf(`You are an expert at explaining fraud investigation findings.
Generate a clear, %s explanation for a %s audience.
Focus on: %v
Use %s language.

Provide a structured explanation that is easy to understand.`,
		profile.TechnicalLevel, input.Audience, profile.FocusAreas, profile.Language)

	// Build context
	stateContext := ""
	if input.State != nil {
		if input.State.RiskAssessment != nil {
			stateContext = fmt.Sprintf("Risk Level: %s, Score: %.2f, Decision: %s",
				input.State.RiskAssessment.Level,
				input.State.RiskAssessment.Score,
				input.State.FinalDecision)
		}
	}

	userPrompt := fmt.Sprintf(`Generate an explanation for this fraud investigation:

%s

Provide:
1. A brief summary
2. Key findings
3. The decision and rationale
4. Next steps (if applicable)

Format as JSON:
{
  "summary": "...",
  "key_findings": ["finding1", "finding2"],
  "decision_rationale": "...",
  "next_steps": ["step1", "step2"]
}`, stateContext)

	response, _, err := a.GenerateWithRAG(ctx, systemPrompt, userPrompt, []string{
		"investigation explanation",
	})
	if err != nil {
		return nil, err
	}

	var explanation HumanReadableExplanation
	if err := a.ParseJSONResponse(response, &explanation); err != nil {
		return nil, err
	}

	return &explanation, nil
}

// generateFallbackExplanation creates a basic explanation without LLM
func (a *ExplanationAgent) generateFallbackExplanation(input *ExplanationInput) *HumanReadableExplanation {
	explanation := &HumanReadableExplanation{
		Summary:   "Investigation completed",
		NextSteps: []string{"Review attached report"},
	}

	if input.State != nil && input.State.RiskAssessment != nil {
		explanation.Summary = fmt.Sprintf("Investigation completed with %s risk level",
			input.State.RiskAssessment.Level)
		explanation.DecisionRationale = input.State.RiskAssessment.Explanation

		for _, factor := range input.State.RiskAssessment.ContributingFactors {
			explanation.KeyFindings = append(explanation.KeyFindings, factor.Description)
		}
	}

	return explanation
}

// HumanReadableExplanation contains the generated explanation
type HumanReadableExplanation struct {
	Summary           string   `json:"summary"`
	KeyFindings       []string `json:"key_findings"`
	DecisionRationale string   `json:"decision_rationale"`
	NextSteps         []string `json:"next_steps"`
}

// GetExplanation extracts explanation from finding
func GetExplanation(finding *models.AgentFinding) (*HumanReadableExplanation, bool) {
	if finding == nil || finding.Metadata == nil {
		return nil, false
	}

	exp, ok := finding.Metadata["explanation"].(*HumanReadableExplanation)
	return exp, ok
}

// GetReport extracts report from finding
func GetReport(finding *models.AgentFinding) (*models.InvestigationReport, bool) {
	if finding == nil || finding.Metadata == nil {
		return nil, false
	}

	report, ok := finding.Metadata["investigation_report"].(*models.InvestigationReport)
	return report, ok
}

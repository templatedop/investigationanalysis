// Package agents provides the risk assessment agent
package agents

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// RiskAssessmentAgent synthesizes all findings into a risk score
type RiskAssessmentAgent struct {
	*BaseAgent
	thresholds *RiskThresholds
}

// RiskThresholds defines thresholds for risk classification
type RiskThresholds struct {
	LowMax      float64 `json:"low_max"`
	MediumMax   float64 `json:"medium_max"`
	HighMax     float64 `json:"high_max"`
	// Above HighMax is Critical
}

// DefaultRiskThresholds returns default thresholds
func DefaultRiskThresholds() *RiskThresholds {
	return &RiskThresholds{
		LowMax:    0.3,
		MediumMax: 0.6,
		HighMax:   0.8,
	}
}

// NewRiskAssessmentAgent creates a new risk assessment agent
func NewRiskAssessmentAgent(llmClient *llm.Client, ragSystem *rag.System) *RiskAssessmentAgent {
	return &RiskAssessmentAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeRiskAssessment,
			"Risk Assessment Agent",
			"Synthesizes all findings into a final risk assessment",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "reasoning",
			},
		),
		thresholds: DefaultRiskThresholds(),
	}
}

// SetThresholds sets custom risk thresholds
func (a *RiskAssessmentAgent) SetThresholds(thresholds *RiskThresholds) {
	a.thresholds = thresholds
}

// RiskInput contains input for risk assessment
type RiskInput struct {
	State    *models.InvestigationState `json:"state"`
	Findings *models.AgentFindings      `json:"findings"`
}

// Execute performs risk assessment
func (a *RiskAssessmentAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	riskInput, ok := input.(*RiskInput)
	if !ok {
		return nil, fmt.Errorf("expected *RiskInput, got %T", input)
	}

	startTime := time.Now()

	// Collect all risk indicators from all agents
	allIndicators := a.collectIndicators(riskInput.Findings)

	// Calculate base risk score
	baseScore := a.calculateBaseScore(allIndicators)

	// Apply contextual adjustments
	adjustedScore := a.applyContextualAdjustments(baseScore, riskInput.State)

	// Determine risk level
	riskLevel := a.determineRiskLevel(adjustedScore)

	// Calculate confidence
	confidence := a.calculateConfidence(riskInput.Findings)

	// Get contributing factors
	factors := a.getContributingFactors(allIndicators)

	// Generate recommended action
	action := a.recommendAction(adjustedScore, riskLevel, confidence, riskInput.State)

	// Create risk assessment
	assessment := &models.RiskAssessment{
		Score:               adjustedScore,
		Level:               riskLevel,
		Confidence:          confidence,
		ContributingFactors: factors,
		RecommendedAction:   action,
		RequiresHumanReview: a.requiresHumanReview(adjustedScore, confidence, riskInput.State),
		Explanation:         a.generateExplanation(adjustedScore, riskLevel, factors),
	}

	// Get RAG context
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"fraud risk assessment criteria",
		"risk scoring guidelines",
	})

	finding := a.CreateFinding(confidence, allIndicators, nil, sources)
	finding.Metadata["risk_assessment"] = assessment
	finding.Metadata["base_score"] = baseScore
	finding.Metadata["adjusted_score"] = adjustedScore
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// collectIndicators gathers all risk indicators from agent findings
func (a *RiskAssessmentAgent) collectIndicators(findings *models.AgentFindings) []models.RiskIndicator {
	var all []models.RiskIndicator

	addFromFinding := func(f *models.AgentFinding) {
		if f != nil {
			all = append(all, f.RiskIndicators...)
		}
	}

	addFromFinding(findings.Pattern)
	addFromFinding(findings.Network)
	addFromFinding(findings.Temporal)
	addFromFinding(findings.Document)
	addFromFinding(findings.Policy)
	addFromFinding(findings.Entity)

	return all
}

// calculateBaseScore computes the base risk score from indicators
func (a *RiskAssessmentAgent) calculateBaseScore(indicators []models.RiskIndicator) float64 {
	if len(indicators) == 0 {
		return 0.1 // Low base risk
	}

	// Weight by severity
	weights := map[string]float64{
		"critical": 1.0,
		"high":     0.75,
		"medium":   0.5,
		"low":      0.25,
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for _, ind := range indicators {
		w := weights[ind.Severity]
		if w == 0 {
			w = 0.5 // Default weight
		}
		weightedSum += ind.Score * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0.5
	}

	return math.Min(weightedSum/totalWeight, 1.0)
}

// applyContextualAdjustments adjusts score based on context
func (a *RiskAssessmentAgent) applyContextualAdjustments(baseScore float64, state *models.InvestigationState) float64 {
	score := baseScore

	if state == nil || state.AlertInput == nil {
		return score
	}

	// High-value claims get boost
	if state.AlertInput.Amount > 100000 {
		score *= 1.1
	}

	// Priority claims get boost
	if state.AlertInput.Priority >= 4 {
		score *= 1.05
	}

	// Multiple alert triggers indicate higher risk
	if len(state.AlertInput.AlertTriggers) > 3 {
		score *= 1.1
	}

	// Previous fraud history increases risk
	if state.EnrichedData != nil {
		for _, claim := range state.EnrichedData.RelatedClaims {
			if claim.IsFraud {
				score *= 1.3
				break
			}
		}
	}

	return math.Min(score, 1.0)
}

// determineRiskLevel classifies the risk score
func (a *RiskAssessmentAgent) determineRiskLevel(score float64) models.RiskLevel {
	if score <= a.thresholds.LowMax {
		return models.RiskLow
	}
	if score <= a.thresholds.MediumMax {
		return models.RiskMedium
	}
	if score <= a.thresholds.HighMax {
		return models.RiskHigh
	}
	return models.RiskCritical
}

// calculateConfidence computes confidence in the assessment
func (a *RiskAssessmentAgent) calculateConfidence(findings *models.AgentFindings) float64 {
	var totalConfidence float64
	count := 0

	check := func(f *models.AgentFinding) {
		if f != nil {
			totalConfidence += f.Confidence
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

	// More agents = higher confidence
	agentBonus := float64(count) * 0.02

	return math.Min(totalConfidence/float64(count)+agentBonus, 1.0)
}

// getContributingFactors identifies top contributing factors
func (a *RiskAssessmentAgent) getContributingFactors(indicators []models.RiskIndicator) []models.ContributingFactor {
	// Sort by score descending
	sort.Slice(indicators, func(i, j int) bool {
		return indicators[i].Score > indicators[j].Score
	})

	// Take top 5
	limit := 5
	if len(indicators) < limit {
		limit = len(indicators)
	}

	factors := make([]models.ContributingFactor, limit)
	for i := 0; i < limit; i++ {
		ind := indicators[i]
		factors[i] = models.ContributingFactor{
			Factor:       ind.Code,
			Weight:       a.severityWeight(ind.Severity),
			Contribution: ind.Score,
			Description:  ind.Description,
		}
	}

	return factors
}

// severityWeight returns weight for severity level
func (a *RiskAssessmentAgent) severityWeight(severity string) float64 {
	switch severity {
	case "critical":
		return 1.0
	case "high":
		return 0.75
	case "medium":
		return 0.5
	case "low":
		return 0.25
	default:
		return 0.5
	}
}

// recommendAction suggests an action based on assessment
func (a *RiskAssessmentAgent) recommendAction(score float64, level models.RiskLevel, confidence float64, state *models.InvestigationState) string {
	// High confidence thresholds
	if confidence >= 0.85 {
		switch level {
		case models.RiskCritical:
			return "DENY_CLAIM"
		case models.RiskHigh:
			return "ESCALATE_TO_SIU"
		case models.RiskMedium:
			return "ENHANCED_REVIEW"
		case models.RiskLow:
			return "APPROVE_WITH_AUDIT"
		}
	}

	// Lower confidence requires human review
	return "HUMAN_REVIEW_REQUIRED"
}

// requiresHumanReview determines if human review is needed
func (a *RiskAssessmentAgent) requiresHumanReview(score float64, confidence float64, state *models.InvestigationState) bool {
	// Low confidence always needs review
	if confidence < 0.8 {
		return true
	}

	// High-value claims need review
	if state != nil && state.AlertInput != nil && state.AlertInput.Amount > 500000 {
		return true
	}

	// Medium-high risk with any uncertainty
	if score >= 0.6 && confidence < 0.9 {
		return true
	}

	return false
}

// generateExplanation creates a human-readable explanation
func (a *RiskAssessmentAgent) generateExplanation(score float64, level models.RiskLevel, factors []models.ContributingFactor) string {
	explanation := fmt.Sprintf("Risk Level: %s (Score: %.2f)\n\n", level, score)
	explanation += "Key Contributing Factors:\n"

	for i, f := range factors {
		explanation += fmt.Sprintf("%d. %s (%.2f contribution)\n", i+1, f.Description, f.Contribution)
	}

	return explanation
}

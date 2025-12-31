// Package agents provides the policy compliance agent
package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// PolicyComplianceAgent checks claims against policy terms and regulations
type PolicyComplianceAgent struct {
	*BaseAgent
}

// NewPolicyComplianceAgent creates a new policy compliance agent
func NewPolicyComplianceAgent(llmClient *llm.Client, ragSystem *rag.System) *PolicyComplianceAgent {
	return &PolicyComplianceAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypePolicyCompliance,
			"Policy Compliance Agent",
			"Checks claims against policy terms, conditions, and regulations",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium",
			},
		),
	}
}

// Execute performs policy compliance checks
func (a *PolicyComplianceAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	// Perform compliance checks
	analysis := &ComplianceAnalysis{
		PolicyChecks:     a.checkPolicyTerms(analysisCtx),
		ExclusionChecks:  a.checkExclusions(analysisCtx),
		WaitingPeriodChecks: a.checkWaitingPeriods(analysisCtx),
		CoverageChecks:   a.checkCoverage(analysisCtx),
		RegulatoryChecks: a.checkRegulatory(ctx, analysisCtx),
		AMLChecks:        a.checkAML(analysisCtx),
	}

	// Get RAG context for policy interpretation
	ragQuery := a.buildPolicyQuery(analysisCtx)
	ragContext, sources, _ := a.GetRAGContext(ctx, []string{
		ragQuery,
		"policy exclusion clauses",
		"regulatory requirements insurance",
	})

	// Use LLM to interpret policy terms
	interpretation, err := a.interpretPolicyWithLLM(ctx, analysisCtx, ragContext)
	if err == nil {
		analysis.LLMInterpretation = interpretation
	}

	// Generate findings
	indicators, evidence := a.generateComplianceFindings(analysis)

	finding := a.CreateFinding(
		a.calculateComplianceConfidence(analysis),
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["compliance_analysis"] = analysis
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// ComplianceAnalysis contains all compliance check results
type ComplianceAnalysis struct {
	PolicyChecks        []ComplianceCheck  `json:"policy_checks"`
	ExclusionChecks     []ExclusionCheck   `json:"exclusion_checks"`
	WaitingPeriodChecks []WaitingPeriodCheck `json:"waiting_period_checks"`
	CoverageChecks      []CoverageCheck    `json:"coverage_checks"`
	RegulatoryChecks    []RegulatoryCheck  `json:"regulatory_checks"`
	AMLChecks           []AMLCheck         `json:"aml_checks"`
	LLMInterpretation   *PolicyInterpretation `json:"llm_interpretation,omitempty"`
}

// ComplianceCheck represents a general policy compliance check
type ComplianceCheck struct {
	CheckType   string `json:"check_type"`
	Description string `json:"description"`
	Result      string `json:"result"` // "compliant", "non_compliant", "review_required"
	Details     string `json:"details"`
	Severity    string `json:"severity"`
}

// ExclusionCheck represents an exclusion clause check
type ExclusionCheck struct {
	ExclusionType   string  `json:"exclusion_type"`
	ExclusionText   string  `json:"exclusion_text"`
	IsTriggered     bool    `json:"is_triggered"`
	TriggerReason   string  `json:"trigger_reason,omitempty"`
	Confidence      float64 `json:"confidence"`
}

// WaitingPeriodCheck represents a waiting period check
type WaitingPeriodCheck struct {
	PeriodType      string `json:"period_type"`
	RequiredDays    int    `json:"required_days"`
	ActualDays      int    `json:"actual_days"`
	IsViolation     bool   `json:"is_violation"`
}

// CoverageCheck represents a coverage limit check
type CoverageCheck struct {
	CoverageType    string  `json:"coverage_type"`
	ClaimedAmount   float64 `json:"claimed_amount"`
	MaxCoverage     float64 `json:"max_coverage"`
	RemainingLimit  float64 `json:"remaining_limit"`
	IsWithinLimits  bool    `json:"is_within_limits"`
}

// RegulatoryCheck represents a regulatory compliance check
type RegulatoryCheck struct {
	Regulation   string `json:"regulation"`
	Requirement  string `json:"requirement"`
	IsCompliant  bool   `json:"is_compliant"`
	Details      string `json:"details"`
}

// AMLCheck represents anti-money laundering check
type AMLCheck struct {
	CheckType      string `json:"check_type"`
	Result         string `json:"result"`
	RiskIndicators []string `json:"risk_indicators,omitempty"`
	RequiresReview bool   `json:"requires_review"`
}

// PolicyInterpretation contains LLM's interpretation of policy terms
type PolicyInterpretation struct {
	ClaimEligibility    string   `json:"claim_eligibility"`
	ApplicableTerms     []string `json:"applicable_terms"`
	PotentialIssues     []string `json:"potential_issues"`
	Recommendations     []string `json:"recommendations"`
	ConfidenceLevel     float64  `json:"confidence_level"`
}

// checkPolicyTerms verifies claim against policy terms
func (a *PolicyComplianceAgent) checkPolicyTerms(analysisCtx *AnalysisContext) []ComplianceCheck {
	var checks []ComplianceCheck

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		checks = append(checks, ComplianceCheck{
			CheckType:   "policy_validity",
			Description: "Policy details not available",
			Result:      "review_required",
			Severity:    "medium",
		})
		return checks
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	alert := analysisCtx.AlertInput

	// Check if policy is active
	now := time.Now()
	if now.Before(policy.StartDate) {
		checks = append(checks, ComplianceCheck{
			CheckType:   "policy_not_started",
			Description: "Policy has not yet started",
			Result:      "non_compliant",
			Details:     fmt.Sprintf("Policy starts on %s", policy.StartDate.Format("2006-01-02")),
			Severity:    "critical",
		})
	} else if now.After(policy.EndDate) {
		checks = append(checks, ComplianceCheck{
			CheckType:   "policy_expired",
			Description: "Policy has expired",
			Result:      "non_compliant",
			Details:     fmt.Sprintf("Policy ended on %s", policy.EndDate.Format("2006-01-02")),
			Severity:    "critical",
		})
	} else {
		checks = append(checks, ComplianceCheck{
			CheckType:   "policy_active",
			Description: "Policy is currently active",
			Result:      "compliant",
			Severity:    "info",
		})
	}

	// Check claim type matches policy type
	if alert != nil && policy.Type != "" {
		if !a.isClaimTypeValid(alert.ClaimType, policy.Type) {
			checks = append(checks, ComplianceCheck{
				CheckType:   "claim_type_mismatch",
				Description: "Claim type may not match policy coverage",
				Result:      "review_required",
				Details:     fmt.Sprintf("Claim: %s, Policy: %s", alert.ClaimType, policy.Type),
				Severity:    "high",
			})
		}
	}

	return checks
}

// isClaimTypeValid checks if claim type is valid for policy type
func (a *PolicyComplianceAgent) isClaimTypeValid(claimType, policyType string) bool {
	validMappings := map[string][]string{
		"health":   {"medical", "hospitalization", "surgery", "prescription"},
		"auto":     {"accident", "theft", "damage", "liability"},
		"property": {"fire", "theft", "flood", "damage"},
		"life":     {"death", "critical_illness", "disability"},
	}

	validTypes, ok := validMappings[policyType]
	if !ok {
		return true // Unknown policy type, allow
	}

	for _, vt := range validTypes {
		if vt == claimType {
			return true
		}
	}
	return false
}

// checkExclusions verifies claim doesn't fall under exclusions
func (a *PolicyComplianceAgent) checkExclusions(analysisCtx *AnalysisContext) []ExclusionCheck {
	var checks []ExclusionCheck

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		return checks
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	alert := analysisCtx.AlertInput

	for _, exclusion := range policy.Exclusions {
		check := ExclusionCheck{
			ExclusionType: exclusion,
			ExclusionText: exclusion,
			IsTriggered:   false,
			Confidence:    0.8,
		}

		// Simple keyword matching for exclusions
		if alert != nil && a.mightTriggerExclusion(alert.Description, exclusion) {
			check.IsTriggered = true
			check.TriggerReason = "Claim description contains keywords related to exclusion"
		}

		checks = append(checks, check)
	}

	return checks
}

// mightTriggerExclusion checks if claim might trigger an exclusion
func (a *PolicyComplianceAgent) mightTriggerExclusion(description, exclusion string) bool {
	// This would use more sophisticated NLP in production
	exclusionKeywords := map[string][]string{
		"pre-existing conditions": {"chronic", "prior", "existing", "previous condition"},
		"cosmetic procedures":     {"cosmetic", "plastic surgery", "aesthetic"},
		"self-inflicted":         {"self-harm", "suicide", "intentional"},
		"war":                    {"war", "terrorism", "civil unrest"},
		"illegal activities":     {"illegal", "criminal", "unlawful"},
	}

	keywords, ok := exclusionKeywords[exclusion]
	if !ok {
		return false
	}

	for _, kw := range keywords {
		if containsIgnoreCase(description, kw) {
			return true
		}
	}

	return false
}

// checkWaitingPeriods verifies waiting period compliance
func (a *PolicyComplianceAgent) checkWaitingPeriods(analysisCtx *AnalysisContext) []WaitingPeriodCheck {
	var checks []WaitingPeriodCheck

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		return checks
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	alert := analysisCtx.AlertInput

	if alert == nil {
		return checks
	}

	daysSinceStart := int(alert.SubmittedAt.Sub(policy.StartDate).Hours() / 24)

	for periodType, requiredDays := range policy.WaitingPeriods {
		check := WaitingPeriodCheck{
			PeriodType:   periodType,
			RequiredDays: requiredDays,
			ActualDays:   daysSinceStart,
			IsViolation:  daysSinceStart < requiredDays,
		}
		checks = append(checks, check)
	}

	return checks
}

// checkCoverage verifies claim is within coverage limits
func (a *PolicyComplianceAgent) checkCoverage(analysisCtx *AnalysisContext) []CoverageCheck {
	var checks []CoverageCheck

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		return checks
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	alert := analysisCtx.AlertInput

	if alert == nil {
		return checks
	}

	// Check against total coverage amount
	claimedAmount := alert.Amount
	maxCoverage := policy.CoverageAmount

	// Calculate already claimed amount
	alreadyClaimed := 0.0
	for _, claim := range analysisCtx.EnrichedData.RelatedClaims {
		if claim.Status == "approved" || claim.Status == "paid" {
			alreadyClaimed += claim.Amount
		}
	}

	remainingLimit := maxCoverage - alreadyClaimed

	checks = append(checks, CoverageCheck{
		CoverageType:   "total_coverage",
		ClaimedAmount:  claimedAmount,
		MaxCoverage:    maxCoverage,
		RemainingLimit: remainingLimit,
		IsWithinLimits: claimedAmount <= remainingLimit,
	})

	return checks
}

// checkRegulatory verifies regulatory compliance
func (a *PolicyComplianceAgent) checkRegulatory(ctx context.Context, analysisCtx *AnalysisContext) []RegulatoryCheck {
	var checks []RegulatoryCheck

	// IRDAI regulations for insurance (India-specific)
	checks = append(checks, RegulatoryCheck{
		Regulation:  "IRDAI",
		Requirement: "Claim documentation requirements",
		IsCompliant: true, // Would need actual document check
		Details:     "Documentation status to be verified",
	})

	// KYC requirements
	if analysisCtx.EnrichedData != nil && analysisCtx.EnrichedData.CustomerProfile != nil {
		checks = append(checks, RegulatoryCheck{
			Regulation:  "KYC",
			Requirement: "Customer identification verified",
			IsCompliant: true,
			Details:     "Customer profile exists",
		})
	} else {
		checks = append(checks, RegulatoryCheck{
			Regulation:  "KYC",
			Requirement: "Customer identification",
			IsCompliant: false,
			Details:     "Customer profile not available for verification",
		})
	}

	return checks
}

// checkAML performs anti-money laundering checks
func (a *PolicyComplianceAgent) checkAML(analysisCtx *AnalysisContext) []AMLCheck {
	var checks []AMLCheck

	alert := analysisCtx.AlertInput
	if alert == nil {
		return checks
	}

	// High-value transaction check
	if alert.Amount > 1000000 { // 10 lakh threshold
		checks = append(checks, AMLCheck{
			CheckType:      "high_value_transaction",
			Result:         "flagged",
			RiskIndicators: []string{"Amount exceeds reporting threshold"},
			RequiresReview: true,
		})
	}

	// Structuring check (multiple claims just under threshold)
	if analysisCtx.EnrichedData != nil {
		recentClaims := 0
		for _, claim := range analysisCtx.EnrichedData.RelatedClaims {
			if claim.Amount > 800000 && claim.Amount < 1000000 {
				recentClaims++
			}
		}
		if recentClaims > 2 {
			checks = append(checks, AMLCheck{
				CheckType:      "potential_structuring",
				Result:         "suspicious",
				RiskIndicators: []string{"Multiple claims near reporting threshold"},
				RequiresReview: true,
			})
		}
	}

	return checks
}

// buildPolicyQuery creates a query for RAG based on context
func (a *PolicyComplianceAgent) buildPolicyQuery(analysisCtx *AnalysisContext) string {
	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		return "insurance policy terms and conditions"
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	return fmt.Sprintf("%s insurance policy coverage terms", policy.Type)
}

// interpretPolicyWithLLM uses LLM to interpret policy for the claim
func (a *PolicyComplianceAgent) interpretPolicyWithLLM(ctx context.Context, analysisCtx *AnalysisContext, ragContext string) (*PolicyInterpretation, error) {
	systemPrompt := `You are an insurance policy expert. Analyze the claim against the policy terms and provide a structured assessment.

Your analysis should:
1. Determine if the claim is eligible under the policy
2. Identify applicable terms and conditions
3. Flag any potential issues or exclusions
4. Provide recommendations

Return your analysis in JSON format.`

	alert := analysisCtx.AlertInput
	policy := analysisCtx.EnrichedData.PolicyDetails

	userPrompt := fmt.Sprintf(`## Policy Context
%s

## Policy Details
Type: %s
Coverage: %.2f
Exclusions: %v
Waiting Periods: %v

## Claim Details
Type: %s
Amount: %.2f
Description: %s

Analyze this claim against the policy and respond with JSON:
{
  "claim_eligibility": "eligible|not_eligible|review_required",
  "applicable_terms": ["term1", "term2"],
  "potential_issues": ["issue1", "issue2"],
  "recommendations": ["recommendation1", "recommendation2"],
  "confidence_level": 0.0-1.0
}`,
		ragContext,
		policy.Type, policy.CoverageAmount, policy.Exclusions, policy.WaitingPeriods,
		alert.ClaimType, alert.Amount, alert.Description)

	response, _, err := a.GenerateWithRAG(ctx, systemPrompt, userPrompt, nil)
	if err != nil {
		return nil, err
	}

	var interpretation PolicyInterpretation
	if err := a.ParseJSONResponse(response, &interpretation); err != nil {
		return nil, err
	}

	return &interpretation, nil
}

// generateComplianceFindings creates indicators and evidence
func (a *PolicyComplianceAgent) generateComplianceFindings(analysis *ComplianceAnalysis) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	// Policy check findings
	for _, check := range analysis.PolicyChecks {
		if check.Result == "non_compliant" {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "POL001",
				Description: check.Description,
				Severity:    check.Severity,
				Score:       a.severityScore(check.Severity),
				Category:    "policy_compliance",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "policy_check",
				Description: fmt.Sprintf("%s: %s", check.CheckType, check.Details),
				Source:      "policy_compliance_agent",
				Timestamp:   time.Now(),
				Confidence:  0.95,
			})
		}
	}

	// Exclusion findings
	for _, check := range analysis.ExclusionChecks {
		if check.IsTriggered {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "POL002",
				Description: fmt.Sprintf("Exclusion triggered: %s", check.ExclusionType),
				Severity:    "high",
				Score:       check.Confidence,
				Category:    "exclusion",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "exclusion_trigger",
				Description: check.TriggerReason,
				Source:      "policy_compliance_agent",
				Timestamp:   time.Now(),
				Confidence:  check.Confidence,
			})
		}
	}

	// Waiting period violations
	for _, check := range analysis.WaitingPeriodChecks {
		if check.IsViolation {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "POL003",
				Description: fmt.Sprintf("Waiting period violation: %s", check.PeriodType),
				Severity:    "high",
				Score:       0.9,
				Category:    "waiting_period",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "waiting_period",
				Description: fmt.Sprintf("Required %d days, actual %d days", check.RequiredDays, check.ActualDays),
				Source:      "policy_compliance_agent",
				Timestamp:   time.Now(),
				Confidence:  0.95,
			})
		}
	}

	// Coverage limit issues
	for _, check := range analysis.CoverageChecks {
		if !check.IsWithinLimits {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "POL004",
				Description: "Claim exceeds coverage limits",
				Severity:    "high",
				Score:       0.85,
				Category:    "coverage",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "coverage_limit",
				Description: fmt.Sprintf("Claimed %.2f, remaining limit %.2f", check.ClaimedAmount, check.RemainingLimit),
				Source:      "policy_compliance_agent",
				Timestamp:   time.Now(),
				Confidence:  0.95,
			})
		}
	}

	// AML findings
	for _, check := range analysis.AMLChecks {
		if check.RequiresReview {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "POL005",
				Description: fmt.Sprintf("AML flag: %s", check.CheckType),
				Severity:    "high",
				Score:       0.8,
				Category:    "aml",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "aml_check",
				Description: fmt.Sprintf("%s - %v", check.Result, check.RiskIndicators),
				Source:      "policy_compliance_agent",
				Timestamp:   time.Now(),
				Confidence:  0.9,
			})
		}
	}

	return indicators, evidence
}

// severityScore converts severity to numeric score
func (a *PolicyComplianceAgent) severityScore(severity string) float64 {
	switch severity {
	case "critical":
		return 0.95
	case "high":
		return 0.8
	case "medium":
		return 0.6
	case "low":
		return 0.4
	default:
		return 0.5
	}
}

// calculateComplianceConfidence computes overall confidence
func (a *PolicyComplianceAgent) calculateComplianceConfidence(analysis *ComplianceAnalysis) float64 {
	// Base confidence
	confidence := 0.85

	// LLM interpretation adds confidence
	if analysis.LLMInterpretation != nil {
		confidence = (confidence + analysis.LLMInterpretation.ConfidenceLevel) / 2
	}

	return confidence
}

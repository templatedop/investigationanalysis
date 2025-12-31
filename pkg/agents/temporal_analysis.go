// Package agents provides the temporal analysis agent
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

// TemporalAnalysisAgent analyzes time-based patterns and sequences
type TemporalAnalysisAgent struct {
	*BaseAgent
}

// NewTemporalAnalysisAgent creates a new temporal analysis agent
func NewTemporalAnalysisAgent(llmClient *llm.Client, ragSystem *rag.System) *TemporalAnalysisAgent {
	return &TemporalAnalysisAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeTemporalAnalysis,
			"Temporal Analysis Agent",
			"Analyzes time-based patterns and event sequences",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium",
			},
		),
	}
}

// Execute performs temporal analysis
func (a *TemporalAnalysisAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	// Build timeline
	timeline := a.buildTimeline(analysisCtx)

	// Analyze temporal patterns
	temporalAnalysis := &TemporalAnalysis{
		Timeline: timeline,
	}

	// Check event sequence plausibility
	temporalAnalysis.SequenceAnomalies = a.analyzeSequences(timeline)

	// Check timing patterns
	temporalAnalysis.TimingPatterns = a.analyzeTimingPatterns(analysisCtx)

	// Check for coordinated timing
	temporalAnalysis.CoordinatedEvents = a.detectCoordinatedTiming(analysisCtx)

	// Check claim timing relative to policy
	temporalAnalysis.PolicyTimingIssues = a.analyzePolicyTiming(analysisCtx)

	// Get RAG context
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"temporal fraud patterns",
		"timing-based fraud detection",
		"claim timing anomalies",
	})

	// Generate findings
	indicators, evidence := a.generateTemporalFindings(temporalAnalysis)

	finding := a.CreateFinding(
		a.calculateTemporalConfidence(temporalAnalysis),
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["temporal_analysis"] = temporalAnalysis
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// TemporalAnalysis contains temporal analysis results
type TemporalAnalysis struct {
	Timeline            []TimelineEvent       `json:"timeline"`
	SequenceAnomalies   []SequenceAnomaly     `json:"sequence_anomalies"`
	TimingPatterns      []TimingPattern       `json:"timing_patterns"`
	CoordinatedEvents   []CoordinatedEvent    `json:"coordinated_events"`
	PolicyTimingIssues  []PolicyTimingIssue   `json:"policy_timing_issues"`
}

// TimelineEvent represents an event in the timeline
type TimelineEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	Description string                 `json:"description"`
	EntityID    string                 `json:"entity_id"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SequenceAnomaly represents an implausible event sequence
type SequenceAnomaly struct {
	Event1       TimelineEvent `json:"event1"`
	Event2       TimelineEvent `json:"event2"`
	TimeDiff     time.Duration `json:"time_diff"`
	AnomalyType  string        `json:"anomaly_type"`
	Description  string        `json:"description"`
	Severity     string        `json:"severity"`
}

// TimingPattern represents a detected timing pattern
type TimingPattern struct {
	PatternType   string    `json:"pattern_type"`
	Description   string    `json:"description"`
	Frequency     float64   `json:"frequency"`
	SampleEvents  []time.Time `json:"sample_events"`
	IsSuspicious  bool      `json:"is_suspicious"`
}

// CoordinatedEvent represents potentially coordinated events
type CoordinatedEvent struct {
	Events      []TimelineEvent `json:"events"`
	TimeDiff    time.Duration   `json:"time_diff"`
	EntityIDs   []string        `json:"entity_ids"`
	Description string          `json:"description"`
}

// PolicyTimingIssue represents suspicious timing relative to policy
type PolicyTimingIssue struct {
	IssueType    string        `json:"issue_type"`
	Description  string        `json:"description"`
	ClaimDate    time.Time     `json:"claim_date"`
	PolicyDate   time.Time     `json:"policy_date"`
	DaysDiff     int           `json:"days_diff"`
	Severity     string        `json:"severity"`
}

// buildTimeline creates a chronological timeline of events
func (a *TemporalAnalysisAgent) buildTimeline(analysisCtx *AnalysisContext) []TimelineEvent {
	var events []TimelineEvent

	if analysisCtx.AlertInput != nil {
		events = append(events, TimelineEvent{
			Timestamp:   analysisCtx.AlertInput.SubmittedAt,
			EventType:   "claim_submitted",
			Description: fmt.Sprintf("Claim submitted: %s", analysisCtx.AlertInput.Description),
			EntityID:    analysisCtx.AlertInput.CustomerID,
			Metadata: map[string]interface{}{
				"amount": analysisCtx.AlertInput.Amount,
			},
		})
	}

	if analysisCtx.EnrichedData != nil {
		// Add policy events
		if policy := analysisCtx.EnrichedData.PolicyDetails; policy != nil {
			events = append(events, TimelineEvent{
				Timestamp:   policy.StartDate,
				EventType:   "policy_started",
				Description: fmt.Sprintf("Policy %s started", policy.Type),
				EntityID:    policy.PolicyID,
			})
		}

		// Add transaction events
		for _, tx := range analysisCtx.EnrichedData.Transactions {
			events = append(events, TimelineEvent{
				Timestamp:   tx.Timestamp,
				EventType:   "transaction",
				Description: fmt.Sprintf("%s transaction: %.2f %s", tx.Type, tx.Amount, tx.Currency),
				EntityID:    analysisCtx.AlertInput.CustomerID,
				Metadata: map[string]interface{}{
					"amount": tx.Amount,
				},
			})
		}

		// Add previous claims
		for _, claim := range analysisCtx.EnrichedData.RelatedClaims {
			events = append(events, TimelineEvent{
				Timestamp:   claim.SubmittedAt,
				EventType:   "previous_claim",
				Description: fmt.Sprintf("Previous claim: %s (%.2f)", claim.Type, claim.Amount),
				EntityID:    analysisCtx.AlertInput.CustomerID,
				Metadata: map[string]interface{}{
					"amount":   claim.Amount,
					"is_fraud": claim.IsFraud,
				},
			})
		}

		// Add account opening
		if profile := analysisCtx.EnrichedData.CustomerProfile; profile != nil {
			events = append(events, TimelineEvent{
				Timestamp:   profile.AccountOpenDate,
				EventType:   "account_opened",
				Description: "Customer account created",
				EntityID:    profile.ID,
			})
		}
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events
}

// analyzeSequences checks for implausible event sequences
func (a *TemporalAnalysisAgent) analyzeSequences(timeline []TimelineEvent) []SequenceAnomaly {
	var anomalies []SequenceAnomaly

	for i := 0; i < len(timeline)-1; i++ {
		event1 := timeline[i]
		event2 := timeline[i+1]
		timeDiff := event2.Timestamp.Sub(event1.Timestamp)

		// Check for claim immediately after policy purchase
		if event1.EventType == "policy_started" && event2.EventType == "claim_submitted" {
			if timeDiff < 7*24*time.Hour { // Within 7 days
				anomalies = append(anomalies, SequenceAnomaly{
					Event1:      event1,
					Event2:      event2,
					TimeDiff:    timeDiff,
					AnomalyType: "early_claim",
					Description: "Claim submitted very soon after policy started",
					Severity:    a.earlyClaimSeverity(timeDiff),
				})
			}
		}

		// Check for rapid succession of claims
		if event1.EventType == "previous_claim" && event2.EventType == "claim_submitted" {
			if timeDiff < 24*time.Hour {
				anomalies = append(anomalies, SequenceAnomaly{
					Event1:      event1,
					Event2:      event2,
					TimeDiff:    timeDiff,
					AnomalyType: "rapid_claims",
					Description: "Multiple claims within 24 hours",
					Severity:    "high",
				})
			}
		}

		// Check for events happening simultaneously
		if timeDiff < time.Minute && event1.EntityID != event2.EntityID {
			anomalies = append(anomalies, SequenceAnomaly{
				Event1:      event1,
				Event2:      event2,
				TimeDiff:    timeDiff,
				AnomalyType: "simultaneous_events",
				Description: "Near-simultaneous events from different entities",
				Severity:    "medium",
			})
		}
	}

	return anomalies
}

// earlyClaimSeverity determines severity based on time gap
func (a *TemporalAnalysisAgent) earlyClaimSeverity(timeDiff time.Duration) string {
	days := timeDiff.Hours() / 24
	if days < 1 {
		return "critical"
	}
	if days < 3 {
		return "high"
	}
	if days < 7 {
		return "medium"
	}
	return "low"
}

// analyzeTimingPatterns identifies recurring timing patterns
func (a *TemporalAnalysisAgent) analyzeTimingPatterns(analysisCtx *AnalysisContext) []TimingPattern {
	var patterns []TimingPattern

	if analysisCtx.EnrichedData == nil {
		return patterns
	}

	claims := analysisCtx.EnrichedData.RelatedClaims
	if len(claims) < 2 {
		return patterns
	}

	// Analyze day of week distribution
	dayCount := make(map[time.Weekday]int)
	var claimTimes []time.Time
	for _, claim := range claims {
		dayCount[claim.SubmittedAt.Weekday()]++
		claimTimes = append(claimTimes, claim.SubmittedAt)
	}

	// Check for weekend concentration
	weekendCount := dayCount[time.Saturday] + dayCount[time.Sunday]
	if float64(weekendCount)/float64(len(claims)) > 0.6 {
		patterns = append(patterns, TimingPattern{
			PatternType:  "weekend_concentration",
			Description:  "Most claims submitted on weekends",
			Frequency:    float64(weekendCount) / float64(len(claims)),
			SampleEvents: claimTimes,
			IsSuspicious: true,
		})
	}

	// Check for end-of-month pattern
	endOfMonthCount := 0
	for _, claim := range claims {
		if claim.SubmittedAt.Day() > 25 {
			endOfMonthCount++
		}
	}
	if float64(endOfMonthCount)/float64(len(claims)) > 0.5 {
		patterns = append(patterns, TimingPattern{
			PatternType:  "end_of_month",
			Description:  "Claims concentrated at end of month",
			Frequency:    float64(endOfMonthCount) / float64(len(claims)),
			IsSuspicious: true,
		})
	}

	// Check for holiday period claims
	holidayCount := 0
	for _, claim := range claims {
		if a.isHolidayPeriod(claim.SubmittedAt) {
			holidayCount++
		}
	}
	if float64(holidayCount)/float64(len(claims)) > 0.4 {
		patterns = append(patterns, TimingPattern{
			PatternType:  "holiday_concentration",
			Description:  "Claims concentrated during holiday periods",
			Frequency:    float64(holidayCount) / float64(len(claims)),
			IsSuspicious: true,
		})
	}

	return patterns
}

// isHolidayPeriod checks if a date falls in a holiday period
func (a *TemporalAnalysisAgent) isHolidayPeriod(t time.Time) bool {
	month := t.Month()
	day := t.Day()

	// Major holiday periods (simplified)
	if month == time.December && day > 20 {
		return true
	}
	if month == time.January && day < 5 {
		return true
	}
	if month == time.October || month == time.November {
		// Diwali period (approximate)
		if day > 25 || day < 10 {
			return true
		}
	}

	return false
}

// detectCoordinatedTiming finds potentially coordinated events
func (a *TemporalAnalysisAgent) detectCoordinatedTiming(analysisCtx *AnalysisContext) []CoordinatedEvent {
	var coordinated []CoordinatedEvent

	// This would need access to related entity events
	// For now, check within the timeline

	return coordinated
}

// analyzePolicyTiming checks timing relative to policy lifecycle
func (a *TemporalAnalysisAgent) analyzePolicyTiming(analysisCtx *AnalysisContext) []PolicyTimingIssue {
	var issues []PolicyTimingIssue

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.PolicyDetails == nil {
		return issues
	}

	policy := analysisCtx.EnrichedData.PolicyDetails
	alert := analysisCtx.AlertInput

	if alert == nil {
		return issues
	}

	// Days since policy started
	daysSinceStart := int(alert.SubmittedAt.Sub(policy.StartDate).Hours() / 24)

	// Check for claim within waiting period
	for claimType, waitingDays := range policy.WaitingPeriods {
		if daysSinceStart < waitingDays {
			issues = append(issues, PolicyTimingIssue{
				IssueType:   "waiting_period_violation",
				Description: fmt.Sprintf("Claim submitted during %s waiting period", claimType),
				ClaimDate:   alert.SubmittedAt,
				PolicyDate:  policy.StartDate,
				DaysDiff:    daysSinceStart,
				Severity:    "high",
			})
		}
	}

	// Check for claim near policy end
	daysUntilEnd := int(policy.EndDate.Sub(alert.SubmittedAt).Hours() / 24)
	if daysUntilEnd > 0 && daysUntilEnd < 30 {
		issues = append(issues, PolicyTimingIssue{
			IssueType:   "pre_expiry_claim",
			Description: "Claim submitted shortly before policy expiry",
			ClaimDate:   alert.SubmittedAt,
			PolicyDate:  policy.EndDate,
			DaysDiff:    daysUntilEnd,
			Severity:    "medium",
		})
	}

	// Check for very early claim
	if daysSinceStart < 7 {
		issues = append(issues, PolicyTimingIssue{
			IssueType:   "immediate_claim",
			Description: "Claim submitted within first week of policy",
			ClaimDate:   alert.SubmittedAt,
			PolicyDate:  policy.StartDate,
			DaysDiff:    daysSinceStart,
			Severity:    "critical",
		})
	}

	return issues
}

// generateTemporalFindings creates indicators and evidence
func (a *TemporalAnalysisAgent) generateTemporalFindings(analysis *TemporalAnalysis) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	// Sequence anomalies
	for _, anomaly := range analysis.SequenceAnomalies {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "TMP001",
			Description: anomaly.Description,
			Severity:    anomaly.Severity,
			Score:       a.anomalyScore(anomaly.Severity),
			Category:    "temporal",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "sequence_anomaly",
			Description: fmt.Sprintf("%s: %v between events", anomaly.AnomalyType, anomaly.TimeDiff),
			Source:      "temporal_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  0.9,
		})
	}

	// Timing patterns
	for _, pattern := range analysis.TimingPatterns {
		if pattern.IsSuspicious {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "TMP002",
				Description: pattern.Description,
				Severity:    "medium",
				Score:       pattern.Frequency,
				Category:    "temporal",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "timing_pattern",
				Description: pattern.Description,
				Source:      "temporal_analysis_agent",
				Timestamp:   time.Now(),
				Confidence:  0.8,
			})
		}
	}

	// Policy timing issues
	for _, issue := range analysis.PolicyTimingIssues {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "TMP003",
			Description: issue.Description,
			Severity:    issue.Severity,
			Score:       a.anomalyScore(issue.Severity),
			Category:    "policy_timing",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "policy_timing",
			Description: fmt.Sprintf("%s (%d days)", issue.IssueType, issue.DaysDiff),
			Source:      "temporal_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  0.95,
		})
	}

	return indicators, evidence
}

// anomalyScore converts severity to score
func (a *TemporalAnalysisAgent) anomalyScore(severity string) float64 {
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

// calculateTemporalConfidence computes confidence in temporal analysis
func (a *TemporalAnalysisAgent) calculateTemporalConfidence(analysis *TemporalAnalysis) float64 {
	if len(analysis.Timeline) < 3 {
		return 0.7 // Lower confidence with limited data
	}

	baseConfidence := 0.85

	// More events = higher confidence
	if len(analysis.Timeline) > 10 {
		baseConfidence += 0.1
	}

	// Anomalies increase confidence in findings
	if len(analysis.SequenceAnomalies) > 0 {
		baseConfidence += 0.05
	}

	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}

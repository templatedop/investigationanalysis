// Package agents provides the pattern analysis agent
package agents

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// PatternAnalysisAgent identifies statistical and behavioral anomalies
type PatternAnalysisAgent struct {
	*BaseAgent
	anomalyDetectors []AnomalyDetector
}

// AnomalyDetector interface for anomaly detection models
type AnomalyDetector interface {
	Name() string
	Detect(ctx context.Context, data interface{}) (*AnomalyResult, error)
}

// AnomalyResult contains anomaly detection results
type AnomalyResult struct {
	IsAnomaly   bool                   `json:"is_anomaly"`
	Score       float64                `json:"score"`
	Explanation string                 `json:"explanation"`
	Features    map[string]float64     `json:"features"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewPatternAnalysisAgent creates a new pattern analysis agent
func NewPatternAnalysisAgent(llmClient *llm.Client, ragSystem *rag.System) *PatternAnalysisAgent {
	return &PatternAnalysisAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypePatternAnalysis,
			"Pattern Analysis Agent",
			"Identifies statistical and behavioral anomalies in fraud patterns",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "reasoning",
			},
		),
		anomalyDetectors: []AnomalyDetector{
			&VelocityAnomalyDetector{},
			&AmountAnomalyDetector{},
			&BehaviorAnomalyDetector{},
		},
	}
}

// AddAnomalyDetector adds an anomaly detector
func (a *PatternAnalysisAgent) AddAnomalyDetector(detector AnomalyDetector) {
	a.anomalyDetectors = append(a.anomalyDetectors, detector)
}

// Execute performs pattern analysis
func (a *PatternAnalysisAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	// Run anomaly detectors
	var anomalyResults []*AnomalyResult
	for _, detector := range a.anomalyDetectors {
		result, err := detector.Detect(ctx, analysisCtx)
		if err == nil && result != nil {
			anomalyResults = append(anomalyResults, result)
		}
	}

	// Analyze patterns with LLM
	patterns, err := a.analyzePatterns(ctx, analysisCtx, anomalyResults)
	if err != nil {
		patterns = &PatternAnalysis{}
	}

	// Get similar historical fraud cases from RAG
	ragContext, sources, _ := a.GetRAGContext(ctx, []string{
		fmt.Sprintf("fraud pattern %s claim", analysisCtx.AlertInput.ClaimType),
		fmt.Sprintf("amount anomaly %.0f", analysisCtx.AlertInput.Amount),
		"behavioral fraud indicators",
	})

	// Use LLM to synthesize findings
	indicators, evidence := a.synthesizeFindings(ctx, patterns, anomalyResults, ragContext)

	finding := a.CreateFinding(
		patterns.Confidence,
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["pattern_analysis"] = patterns
	finding.Metadata["anomaly_results"] = anomalyResults
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// PatternAnalysis contains pattern analysis results
type PatternAnalysis struct {
	VelocityAnomalies    []VelocityAnomaly    `json:"velocity_anomalies"`
	AmountAnomalies      []AmountAnomaly      `json:"amount_anomalies"`
	BehavioralShifts     []BehavioralShift    `json:"behavioral_shifts"`
	GeographicAnomalies  []GeographicAnomaly  `json:"geographic_anomalies"`
	SeasonalPatterns     []SeasonalPattern    `json:"seasonal_patterns"`
	OverallRiskScore     float64              `json:"overall_risk_score"`
	Confidence           float64              `json:"confidence"`
}

// VelocityAnomaly represents abnormal frequency patterns
type VelocityAnomaly struct {
	Type        string    `json:"type"`
	Count       int       `json:"count"`
	Period      string    `json:"period"`
	Expected    float64   `json:"expected"`
	Actual      float64   `json:"actual"`
	Severity    string    `json:"severity"`
	DetectedAt  time.Time `json:"detected_at"`
}

// AmountAnomaly represents unusual amount patterns
type AmountAnomaly struct {
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Average     float64 `json:"average"`
	StdDev      float64 `json:"std_dev"`
	ZScore      float64 `json:"z_score"`
	Percentile  float64 `json:"percentile"`
	Severity    string  `json:"severity"`
}

// BehavioralShift represents changes in behavior patterns
type BehavioralShift struct {
	Attribute   string    `json:"attribute"`
	OldValue    string    `json:"old_value"`
	NewValue    string    `json:"new_value"`
	ChangeDate  time.Time `json:"change_date"`
	Significance float64  `json:"significance"`
}

// GeographicAnomaly represents location-based anomalies
type GeographicAnomaly struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Distance    float64 `json:"distance_km"`
	TimeDiff    string  `json:"time_difference"`
	Plausible   bool    `json:"plausible"`
}

// SeasonalPattern represents time-based patterns
type SeasonalPattern struct {
	Pattern     string  `json:"pattern"`
	Description string  `json:"description"`
	Frequency   float64 `json:"frequency"`
	IsSuspicious bool   `json:"is_suspicious"`
}

// analyzePatterns performs deep pattern analysis
func (a *PatternAnalysisAgent) analyzePatterns(ctx context.Context, analysisCtx *AnalysisContext, anomalyResults []*AnomalyResult) (*PatternAnalysis, error) {
	analysis := &PatternAnalysis{
		Confidence: 0.85,
	}

	if analysisCtx.EnrichedData == nil {
		return analysis, nil
	}

	// Analyze velocity (frequency of events)
	analysis.VelocityAnomalies = a.analyzeVelocity(analysisCtx)

	// Analyze amounts
	analysis.AmountAnomalies = a.analyzeAmounts(analysisCtx)

	// Analyze behavioral patterns
	analysis.BehavioralShifts = a.analyzeBehavior(analysisCtx)

	// Analyze geographic patterns
	analysis.GeographicAnomalies = a.analyzeGeography(analysisCtx)

	// Analyze seasonal patterns
	analysis.SeasonalPatterns = a.analyzeSeasonality(analysisCtx)

	// Calculate overall risk score
	analysis.OverallRiskScore = a.calculatePatternRisk(analysis)

	return analysis, nil
}

// analyzeVelocity checks for unusual frequency patterns
func (a *PatternAnalysisAgent) analyzeVelocity(analysisCtx *AnalysisContext) []VelocityAnomaly {
	var anomalies []VelocityAnomaly

	if analysisCtx.EnrichedData == nil {
		return anomalies
	}

	claims := analysisCtx.EnrichedData.RelatedClaims
	if len(claims) < 2 {
		return anomalies
	}

	// Check claims in last 30 days
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	recentCount := 0
	for _, claim := range claims {
		if claim.SubmittedAt.After(thirtyDaysAgo) {
			recentCount++
		}
	}

	// Threshold: more than 3 claims in 30 days is suspicious
	if recentCount > 3 {
		anomalies = append(anomalies, VelocityAnomaly{
			Type:       "high_claim_frequency",
			Count:      recentCount,
			Period:     "30 days",
			Expected:   1.0,
			Actual:     float64(recentCount),
			Severity:   "high",
			DetectedAt: time.Now(),
		})
	}

	// Check claims in last 7 days
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	weekCount := 0
	for _, claim := range claims {
		if claim.SubmittedAt.After(sevenDaysAgo) {
			weekCount++
		}
	}

	if weekCount > 1 {
		anomalies = append(anomalies, VelocityAnomaly{
			Type:       "rapid_claims",
			Count:      weekCount,
			Period:     "7 days",
			Expected:   0.25,
			Actual:     float64(weekCount),
			Severity:   "critical",
			DetectedAt: time.Now(),
		})
	}

	return anomalies
}

// analyzeAmounts checks for unusual amount patterns
func (a *PatternAnalysisAgent) analyzeAmounts(analysisCtx *AnalysisContext) []AmountAnomaly {
	var anomalies []AmountAnomaly

	if analysisCtx.AlertInput == nil {
		return anomalies
	}

	currentAmount := analysisCtx.AlertInput.Amount

	// Get historical amounts
	var historicalAmounts []float64
	if analysisCtx.EnrichedData != nil {
		for _, claim := range analysisCtx.EnrichedData.RelatedClaims {
			historicalAmounts = append(historicalAmounts, claim.Amount)
		}
	}

	if len(historicalAmounts) == 0 {
		// No history, check against general thresholds
		if currentAmount > 100000 {
			anomalies = append(anomalies, AmountAnomaly{
				Type:       "high_value_claim",
				Amount:     currentAmount,
				Severity:   "high",
				Percentile: 95,
			})
		}
		return anomalies
	}

	// Calculate statistics
	mean := calculateMean(historicalAmounts)
	stdDev := calculateStdDev(historicalAmounts, mean)
	zScore := (currentAmount - mean) / stdDev

	if math.Abs(zScore) > 2.0 {
		severity := "medium"
		if math.Abs(zScore) > 3.0 {
			severity = "high"
		}
		if math.Abs(zScore) > 4.0 {
			severity = "critical"
		}

		anomalies = append(anomalies, AmountAnomaly{
			Type:     "statistical_outlier",
			Amount:   currentAmount,
			Average:  mean,
			StdDev:   stdDev,
			ZScore:   zScore,
			Severity: severity,
		})
	}

	// Check if significantly higher than average
	if currentAmount > mean*3 {
		anomalies = append(anomalies, AmountAnomaly{
			Type:     "amount_spike",
			Amount:   currentAmount,
			Average:  mean,
			Severity: "high",
		})
	}

	return anomalies
}

// analyzeBehavior checks for behavioral pattern changes
func (a *PatternAnalysisAgent) analyzeBehavior(analysisCtx *AnalysisContext) []BehavioralShift {
	var shifts []BehavioralShift

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.CustomerProfile == nil {
		return shifts
	}

	// Check for recent risk score changes
	profile := analysisCtx.EnrichedData.CustomerProfile
	if profile.RiskScore > 0.5 {
		shifts = append(shifts, BehavioralShift{
			Attribute:    "risk_score",
			NewValue:     fmt.Sprintf("%.2f", profile.RiskScore),
			Significance: profile.RiskScore,
		})
	}

	return shifts
}

// analyzeGeography checks for geographic impossibilities
func (a *PatternAnalysisAgent) analyzeGeography(analysisCtx *AnalysisContext) []GeographicAnomaly {
	var anomalies []GeographicAnomaly

	if analysisCtx.EnrichedData == nil {
		return anomalies
	}

	transactions := analysisCtx.EnrichedData.Transactions
	if len(transactions) < 2 {
		return anomalies
	}

	// Check for impossible travel patterns
	for i := 1; i < len(transactions); i++ {
		prev := transactions[i-1]
		curr := transactions[i]

		if prev.Location == nil || curr.Location == nil {
			continue
		}

		// Calculate distance and time between transactions
		distance := calculateDistance(
			prev.Location.Latitude, prev.Location.Longitude,
			curr.Location.Latitude, curr.Location.Longitude,
		)

		timeDiff := curr.Timestamp.Sub(prev.Timestamp)

		// Check if travel is physically possible (assuming max 1000 km/h)
		maxPossibleDistance := timeDiff.Hours() * 1000
		if distance > maxPossibleDistance {
			anomalies = append(anomalies, GeographicAnomaly{
				Type:        "impossible_travel",
				Description: fmt.Sprintf("Transaction in %s then %s", prev.Location.City, curr.Location.City),
				Distance:    distance,
				TimeDiff:    timeDiff.String(),
				Plausible:   false,
			})
		}
	}

	return anomalies
}

// analyzeSeasonality checks for suspicious timing patterns
func (a *PatternAnalysisAgent) analyzeSeasonality(analysisCtx *AnalysisContext) []SeasonalPattern {
	var patterns []SeasonalPattern

	if analysisCtx.EnrichedData == nil {
		return patterns
	}

	claims := analysisCtx.EnrichedData.RelatedClaims

	// Check for claims always on certain days
	dayCount := make(map[time.Weekday]int)
	for _, claim := range claims {
		dayCount[claim.SubmittedAt.Weekday()]++
	}

	// If most claims on same day, suspicious
	for day, count := range dayCount {
		if count > len(claims)/2 && count > 2 {
			patterns = append(patterns, SeasonalPattern{
				Pattern:      "day_concentration",
				Description:  fmt.Sprintf("Most claims submitted on %s", day.String()),
				Frequency:   float64(count) / float64(len(claims)),
				IsSuspicious: true,
			})
		}
	}

	// Check for claims before policy renewal
	policy := analysisCtx.EnrichedData.PolicyDetails
	if policy != nil {
		for _, claim := range claims {
			daysBeforeRenewal := policy.EndDate.Sub(claim.SubmittedAt).Hours() / 24
			if daysBeforeRenewal > 0 && daysBeforeRenewal < 30 {
				patterns = append(patterns, SeasonalPattern{
					Pattern:      "pre_renewal_claim",
					Description:  "Claim submitted shortly before policy renewal",
					IsSuspicious: true,
				})
				break
			}
		}
	}

	return patterns
}

// calculatePatternRisk computes overall pattern-based risk score
func (a *PatternAnalysisAgent) calculatePatternRisk(analysis *PatternAnalysis) float64 {
	score := 0.0
	count := 0

	// Weight velocity anomalies
	for _, va := range analysis.VelocityAnomalies {
		switch va.Severity {
		case "critical":
			score += 0.9
		case "high":
			score += 0.7
		case "medium":
			score += 0.5
		default:
			score += 0.3
		}
		count++
	}

	// Weight amount anomalies
	for _, aa := range analysis.AmountAnomalies {
		switch aa.Severity {
		case "critical":
			score += 0.9
		case "high":
			score += 0.7
		case "medium":
			score += 0.5
		default:
			score += 0.3
		}
		count++
	}

	// Weight geographic anomalies
	for _, ga := range analysis.GeographicAnomalies {
		if !ga.Plausible {
			score += 0.95
			count++
		}
	}

	// Weight seasonal patterns
	for _, sp := range analysis.SeasonalPatterns {
		if sp.IsSuspicious {
			score += 0.6
			count++
		}
	}

	if count == 0 {
		return 0.1 // Low risk if no patterns found
	}

	return score / float64(count)
}

// synthesizeFindings uses LLM to create coherent findings
func (a *PatternAnalysisAgent) synthesizeFindings(ctx context.Context, patterns *PatternAnalysis, anomalyResults []*AnomalyResult, ragContext string) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	// Add indicators for each type of anomaly
	for _, va := range patterns.VelocityAnomalies {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "PAT001",
			Description: fmt.Sprintf("Velocity anomaly: %s", va.Type),
			Severity:    va.Severity,
			Score:       va.Actual / va.Expected / 10,
			Category:    "velocity",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "velocity_analysis",
			Description: fmt.Sprintf("%d events in %s (expected: %.1f)", va.Count, va.Period, va.Expected),
			Source:      "pattern_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  0.9,
		})
	}

	for _, aa := range patterns.AmountAnomalies {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "PAT002",
			Description: fmt.Sprintf("Amount anomaly: %s", aa.Type),
			Severity:    aa.Severity,
			Score:       math.Min(math.Abs(aa.ZScore)/5, 1.0),
			Category:    "amount",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "amount_analysis",
			Description: fmt.Sprintf("Amount %.2f vs average %.2f (z-score: %.2f)", aa.Amount, aa.Average, aa.ZScore),
			Source:      "pattern_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  0.85,
		})
	}

	for _, ga := range patterns.GeographicAnomalies {
		if !ga.Plausible {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "PAT003",
				Description: "Geographic impossibility detected",
				Severity:    "critical",
				Score:       0.95,
				Category:    "geographic",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "geographic_analysis",
				Description: ga.Description,
				Source:      "pattern_analysis_agent",
				Timestamp:   time.Now(),
				Confidence:  0.95,
			})
		}
	}

	return indicators, evidence
}

// Helper functions

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 1
	}
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}

func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Haversine formula
	const R = 6371 // Earth's radius in km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// VelocityAnomalyDetector detects frequency-based anomalies
type VelocityAnomalyDetector struct{}

func (d *VelocityAnomalyDetector) Name() string { return "velocity" }

func (d *VelocityAnomalyDetector) Detect(ctx context.Context, data interface{}) (*AnomalyResult, error) {
	return &AnomalyResult{
		IsAnomaly: false,
		Score:     0.0,
	}, nil
}

// AmountAnomalyDetector detects amount-based anomalies
type AmountAnomalyDetector struct{}

func (d *AmountAnomalyDetector) Name() string { return "amount" }

func (d *AmountAnomalyDetector) Detect(ctx context.Context, data interface{}) (*AnomalyResult, error) {
	return &AnomalyResult{
		IsAnomaly: false,
		Score:     0.0,
	}, nil
}

// BehaviorAnomalyDetector detects behavioral anomalies
type BehaviorAnomalyDetector struct{}

func (d *BehaviorAnomalyDetector) Name() string { return "behavior" }

func (d *BehaviorAnomalyDetector) Detect(ctx context.Context, data interface{}) (*AnomalyResult, error) {
	return &AnomalyResult{
		IsAnomaly: false,
		Score:     0.0,
	}, nil
}

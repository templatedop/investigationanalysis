// Package agents provides the entity resolution agent
package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// EntityResolutionAgent links entities across different records
type EntityResolutionAgent struct {
	*BaseAgent
	matchingStrategies []MatchStrategy
}

// MatchStrategy defines a matching approach
type MatchStrategy interface {
	Name() string
	Match(ctx context.Context, entity1, entity2 interface{}) (*MatchResult, error)
}

// MatchResult contains matching results
type MatchResult struct {
	IsMatch    bool                   `json:"is_match"`
	Confidence float64                `json:"confidence"`
	MatchType  string                 `json:"match_type"`
	Evidence   []string               `json:"evidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// NewEntityResolutionAgent creates a new entity resolution agent
func NewEntityResolutionAgent(llmClient *llm.Client, ragSystem *rag.System) *EntityResolutionAgent {
	return &EntityResolutionAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeEntityResolution,
			"Entity Resolution Agent",
			"Links entities across different records to detect fraud rings",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium",
			},
		),
		matchingStrategies: []MatchStrategy{
			&ExactMatchStrategy{},
			&FuzzyMatchStrategy{threshold: 0.85},
			&AddressMatchStrategy{},
			&PhoneMatchStrategy{},
		},
	}
}

// AddMatchStrategy adds a matching strategy
func (a *EntityResolutionAgent) AddMatchStrategy(strategy MatchStrategy) {
	a.matchingStrategies = append(a.matchingStrategies, strategy)
}

// Execute performs entity resolution
func (a *EntityResolutionAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	// Perform various types of entity resolution
	entityLinks := &models.EntityLinks{}

	// Find direct matches
	directMatches, err := a.findDirectMatches(ctx, analysisCtx)
	if err == nil {
		entityLinks.DirectMatches = directMatches
	}

	// Find transitive links
	transitiveLinks, err := a.findTransitiveLinks(ctx, analysisCtx, directMatches)
	if err == nil {
		entityLinks.TransitiveLinks = transitiveLinks
	}

	// Identify household clusters
	clusters, err := a.identifyHouseholdClusters(ctx, analysisCtx)
	if err == nil {
		entityLinks.HouseholdClusters = clusters
	}

	// Detect movement patterns
	movements, err := a.detectMovementPatterns(ctx, analysisCtx)
	if err == nil {
		entityLinks.MovementPatterns = movements
	}

	// Analyze for fraud indicators
	indicators, evidence := a.analyzeForFraud(ctx, entityLinks)

	// Get RAG context for known patterns
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"entity resolution fraud patterns",
		"fraud ring detection",
		"identity linking patterns",
	})

	finding := a.CreateFinding(
		a.calculateConfidence(entityLinks),
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["entity_links"] = entityLinks
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// findDirectMatches finds entities with direct attribute matches
func (a *EntityResolutionAgent) findDirectMatches(ctx context.Context, analysisCtx *AnalysisContext) ([]models.EntityLink, error) {
	var matches []models.EntityLink

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.CustomerProfile == nil {
		return matches, nil
	}

	profile := analysisCtx.EnrichedData.CustomerProfile

	// Use LLM to analyze potential matches
	systemPrompt := `You are an entity resolution specialist. Analyze the provided data to identify potential entity matches and relationships.

Look for:
1. Same name with different variations (nicknames, transliterations)
2. Shared addresses or phone numbers
3. Related beneficiaries or contacts
4. Common device fingerprints or IP addresses

Return findings in JSON format.`

	userPrompt := fmt.Sprintf(`Analyze this customer profile for entity resolution:

Customer ID: %s
Name: %s
Email: %s
Phone: %s
Address: %v
Related Entities: %v

Claims History: %d claims

Look for patterns that might indicate:
- Identity fraud (same person multiple identities)
- Family/household relationships
- Business relationships
- Potential fraud ring connections

Respond with JSON:
{
  "potential_matches": [
    {
      "entity_id": "...",
      "match_type": "direct|fuzzy|transitive",
      "confidence": 0.0-1.0,
      "evidence": ["reason1", "reason2"]
    }
  ]
}`,
		profile.ID, profile.Name, profile.Email, profile.Phone,
		profile.Address, profile.RelatedEntities, len(profile.ClaimHistory))

	response, _, err := a.GenerateWithRAG(ctx, systemPrompt, userPrompt, []string{
		"entity matching patterns",
		"identity verification",
	})
	if err != nil {
		return matches, nil
	}

	var result struct {
		PotentialMatches []struct {
			EntityID   string   `json:"entity_id"`
			MatchType  string   `json:"match_type"`
			Confidence float64  `json:"confidence"`
			Evidence   []string `json:"evidence"`
		} `json:"potential_matches"`
	}

	if err := a.ParseJSONResponse(response, &result); err != nil {
		return matches, nil
	}

	for _, pm := range result.PotentialMatches {
		matches = append(matches, models.EntityLink{
			SourceID:     profile.ID,
			TargetID:     pm.EntityID,
			LinkType:     pm.MatchType,
			Confidence:   pm.Confidence,
			Evidence:     pm.Evidence,
			DiscoveredAt: time.Now(),
		})
	}

	return matches, nil
}

// findTransitiveLinks finds indirect connections through intermediate entities
func (a *EntityResolutionAgent) findTransitiveLinks(ctx context.Context, analysisCtx *AnalysisContext, directLinks []models.EntityLink) ([]models.EntityLink, error) {
	var transitiveLinks []models.EntityLink

	// Build a graph from direct links
	graph := make(map[string][]string)
	for _, link := range directLinks {
		graph[link.SourceID] = append(graph[link.SourceID], link.TargetID)
		graph[link.TargetID] = append(graph[link.TargetID], link.SourceID)
	}

	// Find paths of length 2 (transitive connections)
	sourceID := ""
	if analysisCtx.AlertInput != nil {
		sourceID = analysisCtx.AlertInput.CustomerID
	}

	if sourceID == "" {
		return transitiveLinks, nil
	}

	visited := make(map[string]bool)
	visited[sourceID] = true

	// BFS to find transitive links
	for _, intermediate := range graph[sourceID] {
		visited[intermediate] = true
		for _, target := range graph[intermediate] {
			if !visited[target] {
				transitiveLinks = append(transitiveLinks, models.EntityLink{
					SourceID:     sourceID,
					TargetID:     target,
					LinkType:     "transitive",
					Confidence:   0.7, // Lower confidence for transitive links
					Evidence:     []string{fmt.Sprintf("Connected through %s", intermediate)},
					DiscoveredAt: time.Now(),
					Metadata: map[string]interface{}{
						"intermediate_entity": intermediate,
					},
				})
			}
		}
	}

	return transitiveLinks, nil
}

// identifyHouseholdClusters groups related entities into households
func (a *EntityResolutionAgent) identifyHouseholdClusters(ctx context.Context, analysisCtx *AnalysisContext) ([]models.Cluster, error) {
	var clusters []models.Cluster

	if analysisCtx.EnrichedData == nil || analysisCtx.EnrichedData.CustomerProfile == nil {
		return clusters, nil
	}

	profile := analysisCtx.EnrichedData.CustomerProfile

	// Analyze address and phone sharing patterns
	systemPrompt := `You are a household clustering expert. Identify potential household or family groupings based on shared attributes.

Consider:
- Same or nearby addresses
- Shared phone numbers
- Similar email domains (family domains)
- Beneficiary relationships
- Similar surnames`

	userPrompt := fmt.Sprintf(`Identify household clusters for:

Primary Entity: %s
Address: %v
Phone: %s
Email: %s
Related Entities: %v

Policy Beneficiaries: %v

Respond with JSON:
{
  "clusters": [
    {
      "cluster_type": "household|family|business",
      "members": ["entity1", "entity2"],
      "strength": 0.0-1.0,
      "evidence": ["reason"]
    }
  ]
}`,
		profile.ID, profile.Address, profile.Phone, profile.Email,
		profile.RelatedEntities,
		a.extractBeneficiaries(analysisCtx))

	response, _, err := a.GenerateWithRAG(ctx, systemPrompt, userPrompt, []string{
		"household clustering",
		"family relationship detection",
	})
	if err != nil {
		return clusters, nil
	}

	var result struct {
		Clusters []struct {
			ClusterType string   `json:"cluster_type"`
			Members     []string `json:"members"`
			Strength    float64  `json:"strength"`
			Evidence    []string `json:"evidence"`
		} `json:"clusters"`
	}

	if err := a.ParseJSONResponse(response, &result); err != nil {
		return clusters, nil
	}

	for i, c := range result.Clusters {
		clusters = append(clusters, models.Cluster{
			ID:       fmt.Sprintf("cluster_%d", i),
			Type:     c.ClusterType,
			Members:  c.Members,
			Strength: c.Strength,
		})
	}

	return clusters, nil
}

// detectMovementPatterns identifies address/location changes
func (a *EntityResolutionAgent) detectMovementPatterns(ctx context.Context, analysisCtx *AnalysisContext) ([]models.Movement, error) {
	var movements []models.Movement

	// In production, this would query historical address data
	// For now, return based on available data

	return movements, nil
}

// extractBeneficiaries extracts beneficiary information
func (a *EntityResolutionAgent) extractBeneficiaries(analysisCtx *AnalysisContext) []models.Beneficiary {
	if analysisCtx.EnrichedData != nil && analysisCtx.EnrichedData.PolicyDetails != nil {
		return analysisCtx.EnrichedData.PolicyDetails.Beneficiaries
	}
	return nil
}

// analyzeForFraud checks entity links for fraud indicators
func (a *EntityResolutionAgent) analyzeForFraud(ctx context.Context, links *models.EntityLinks) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	// Check for suspicious patterns

	// Multiple identities (same person, different records)
	if len(links.DirectMatches) > 3 {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "ENT001",
			Description: "Multiple linked identities detected",
			Severity:    "high",
			Score:       0.8,
			Category:    "identity",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "entity_analysis",
			Description: fmt.Sprintf("%d direct identity matches found", len(links.DirectMatches)),
			Source:      "entity_resolution_agent",
			Timestamp:   time.Now(),
			Confidence:  0.85,
		})
	}

	// Fraud ring pattern (large transitive network)
	if len(links.TransitiveLinks) > 5 {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "ENT002",
			Description: "Potential fraud ring network detected",
			Severity:    "critical",
			Score:       0.9,
			Category:    "network",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "network_analysis",
			Description: fmt.Sprintf("Connected to %d entities through intermediaries", len(links.TransitiveLinks)),
			Source:      "entity_resolution_agent",
			Timestamp:   time.Now(),
			Confidence:  0.75,
		})
	}

	// Suspicious household size
	for _, cluster := range links.HouseholdClusters {
		if len(cluster.Members) > 10 {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "ENT003",
				Description: "Unusually large household cluster",
				Severity:    "medium",
				Score:       0.6,
				Category:    "household",
			})
		}
	}

	// Recent address changes (movement pattern)
	recentMoves := 0
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	for _, move := range links.MovementPatterns {
		if move.MovedAt.After(sixMonthsAgo) {
			recentMoves++
		}
	}
	if recentMoves > 2 {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "ENT004",
			Description: "Frequent address changes detected",
			Severity:    "medium",
			Score:       0.5,
			Category:    "movement",
		})
	}

	return indicators, evidence
}

// calculateConfidence computes overall confidence in entity resolution
func (a *EntityResolutionAgent) calculateConfidence(links *models.EntityLinks) float64 {
	if links == nil {
		return 0.5
	}

	totalConfidence := 0.0
	count := 0

	for _, match := range links.DirectMatches {
		totalConfidence += match.Confidence
		count++
	}
	for _, link := range links.TransitiveLinks {
		totalConfidence += link.Confidence * 0.8 // Discount transitive
		count++
	}
	for _, cluster := range links.HouseholdClusters {
		totalConfidence += cluster.Strength
		count++
	}

	if count == 0 {
		return 0.9 // High confidence when no links found (clean)
	}

	return totalConfidence / float64(count)
}

// ExactMatchStrategy performs exact attribute matching
type ExactMatchStrategy struct{}

// Name returns the strategy name
func (s *ExactMatchStrategy) Name() string {
	return "exact_match"
}

// Match checks for exact matches
func (s *ExactMatchStrategy) Match(ctx context.Context, entity1, entity2 interface{}) (*MatchResult, error) {
	// Implementation would compare specific fields exactly
	return &MatchResult{
		IsMatch:    false,
		Confidence: 0,
		MatchType:  "exact",
	}, nil
}

// FuzzyMatchStrategy performs fuzzy string matching
type FuzzyMatchStrategy struct {
	threshold float64
}

// Name returns the strategy name
func (s *FuzzyMatchStrategy) Name() string {
	return "fuzzy_match"
}

// Match checks for fuzzy matches
func (s *FuzzyMatchStrategy) Match(ctx context.Context, entity1, entity2 interface{}) (*MatchResult, error) {
	// Would use Levenshtein distance or similar
	return &MatchResult{
		IsMatch:    false,
		Confidence: 0,
		MatchType:  "fuzzy",
	}, nil
}

// AddressMatchStrategy matches addresses with normalization
type AddressMatchStrategy struct{}

// Name returns the strategy name
func (s *AddressMatchStrategy) Name() string {
	return "address_match"
}

// Match checks for address matches
func (s *AddressMatchStrategy) Match(ctx context.Context, entity1, entity2 interface{}) (*MatchResult, error) {
	// Would normalize and compare addresses
	return &MatchResult{
		IsMatch:    false,
		Confidence: 0,
		MatchType:  "address",
	}, nil
}

// PhoneMatchStrategy matches phone numbers with normalization
type PhoneMatchStrategy struct{}

// Name returns the strategy name
func (s *PhoneMatchStrategy) Name() string {
	return "phone_match"
}

// Match checks for phone number matches
func (s *PhoneMatchStrategy) Match(ctx context.Context, entity1, entity2 interface{}) (*MatchResult, error) {
	// Would normalize and compare phone numbers
	return &MatchResult{
		IsMatch:    false,
		Confidence: 0,
		MatchType:  "phone",
	}, nil
}

// NormalizePhone normalizes a phone number for comparison
func NormalizePhone(phone string) string {
	// Remove all non-digit characters
	var normalized strings.Builder
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			normalized.WriteRune(c)
		}
	}
	return normalized.String()
}

// NormalizeAddress normalizes an address for comparison
func NormalizeAddress(addr *models.Address) string {
	if addr == nil {
		return ""
	}

	parts := []string{
		strings.ToLower(addr.Street),
		strings.ToLower(addr.City),
		strings.ToLower(addr.State),
		addr.PostalCode,
	}

	return strings.Join(parts, "|")
}

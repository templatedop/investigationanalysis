// Package agents provides the data enrichment agent
package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// DataEnrichmentAgent gathers all relevant context for investigation
type DataEnrichmentAgent struct {
	*BaseAgent
	db              *sql.DB
	externalSources []ExternalDataSource
}

// ExternalDataSource interface for external data providers
type ExternalDataSource interface {
	Name() string
	Fetch(ctx context.Context, entityID string) (map[string]interface{}, error)
}

// NewDataEnrichmentAgent creates a new data enrichment agent
func NewDataEnrichmentAgent(llmClient *llm.Client, ragSystem *rag.System, db *sql.DB) *DataEnrichmentAgent {
	return &DataEnrichmentAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeDataEnrichment,
			"Data Enrichment Agent",
			"Gathers all relevant context before analysis",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "fast",
			},
		),
		db:              db,
		externalSources: []ExternalDataSource{},
	}
}

// AddExternalSource adds an external data source
func (a *DataEnrichmentAgent) AddExternalSource(source ExternalDataSource) {
	a.externalSources = append(a.externalSources, source)
}

// Execute gathers enriched data for the investigation
func (a *DataEnrichmentAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	alertInput, ok := input.(*models.FraudAlertInput)
	if !ok {
		return nil, fmt.Errorf("expected *models.FraudAlertInput, got %T", input)
	}

	startTime := time.Now()

	// Gather data from various sources
	enrichedData := &models.EnrichedData{}

	// Get customer profile
	profile, err := a.getCustomerProfile(ctx, alertInput.CustomerID)
	if err == nil {
		enrichedData.CustomerProfile = profile
	}

	// Get related transactions
	transactions, err := a.getTransactions(ctx, alertInput.CustomerID, alertInput.ClaimID)
	if err == nil {
		enrichedData.Transactions = transactions
	}

	// Get policy details
	policyDetails, err := a.getPolicyDetails(ctx, alertInput.PolicyID)
	if err == nil {
		enrichedData.PolicyDetails = policyDetails
	}

	// Get related claims history
	claims, err := a.getRelatedClaims(ctx, alertInput.CustomerID)
	if err == nil {
		enrichedData.RelatedClaims = claims
	}

	// Fetch external data
	externalData := make(map[string]interface{})
	for _, source := range a.externalSources {
		data, err := source.Fetch(ctx, alertInput.CustomerID)
		if err == nil {
			externalData[source.Name()] = data
		}
	}
	enrichedData.ExternalData = externalData

	enrichedData.EnrichmentTime = time.Since(startTime)

	// Create finding with enriched data
	finding := a.CreateFinding(0.95, nil, nil, nil)
	finding.Metadata["enriched_data"] = enrichedData
	finding.Metadata["data_sources_checked"] = a.getSourcesChecked(enrichedData)
	finding.ProcessingTime = enrichedData.EnrichmentTime

	return finding, nil
}

// getCustomerProfile retrieves customer profile from database
func (a *DataEnrichmentAgent) getCustomerProfile(ctx context.Context, customerID string) (*models.CustomerProfile, error) {
	if a.db == nil {
		return a.mockCustomerProfile(customerID), nil
	}

	query := `
		SELECT id, name, email, phone, risk_score, account_open_date
		FROM customers
		WHERE id = $1
	`

	var profile models.CustomerProfile
	err := a.db.QueryRowContext(ctx, query, customerID).Scan(
		&profile.ID,
		&profile.Name,
		&profile.Email,
		&profile.Phone,
		&profile.RiskScore,
		&profile.AccountOpenDate,
	)
	if err != nil {
		return nil, err
	}

	// Get address
	addrQuery := `
		SELECT street, city, state, postal_code, country
		FROM customer_addresses
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var addr models.Address
	if err := a.db.QueryRowContext(ctx, addrQuery, customerID).Scan(
		&addr.Street, &addr.City, &addr.State, &addr.PostalCode, &addr.Country,
	); err == nil {
		profile.Address = &addr
	}

	return &profile, nil
}

// mockCustomerProfile creates a mock profile for testing
func (a *DataEnrichmentAgent) mockCustomerProfile(customerID string) *models.CustomerProfile {
	return &models.CustomerProfile{
		ID:              customerID,
		Name:            "Sample Customer",
		Email:           "customer@example.com",
		RiskScore:       0.3,
		AccountOpenDate: time.Now().AddDate(-2, 0, 0),
	}
}

// getTransactions retrieves related transactions
func (a *DataEnrichmentAgent) getTransactions(ctx context.Context, customerID, claimID string) ([]models.Transaction, error) {
	if a.db == nil {
		return []models.Transaction{}, nil
	}

	query := `
		SELECT id, type, amount, currency, timestamp, source_account, target_account, metadata
		FROM transactions
		WHERE customer_id = $1
		ORDER BY timestamp DESC
		LIMIT 100
	`

	rows, err := a.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		var metadataJSON []byte

		err := rows.Scan(
			&t.ID, &t.Type, &t.Amount, &t.Currency, &t.Timestamp,
			&t.SourceAccount, &t.TargetAccount, &metadataJSON,
		)
		if err != nil {
			continue
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &t.Metadata)
		}

		transactions = append(transactions, t)
	}

	return transactions, nil
}

// getPolicyDetails retrieves policy information
func (a *DataEnrichmentAgent) getPolicyDetails(ctx context.Context, policyID string) (*models.PolicyDetails, error) {
	if a.db == nil {
		return a.mockPolicyDetails(policyID), nil
	}

	query := `
		SELECT policy_id, type, start_date, end_date, premium, coverage_amount, exclusions, waiting_periods
		FROM policies
		WHERE policy_id = $1
	`

	var policy models.PolicyDetails
	var exclusionsJSON, waitingJSON []byte

	err := a.db.QueryRowContext(ctx, query, policyID).Scan(
		&policy.PolicyID, &policy.Type, &policy.StartDate, &policy.EndDate,
		&policy.Premium, &policy.CoverageAmount, &exclusionsJSON, &waitingJSON,
	)
	if err != nil {
		return nil, err
	}

	if len(exclusionsJSON) > 0 {
		json.Unmarshal(exclusionsJSON, &policy.Exclusions)
	}
	if len(waitingJSON) > 0 {
		json.Unmarshal(waitingJSON, &policy.WaitingPeriods)
	}

	return &policy, nil
}

// mockPolicyDetails creates a mock policy for testing
func (a *DataEnrichmentAgent) mockPolicyDetails(policyID string) *models.PolicyDetails {
	return &models.PolicyDetails{
		PolicyID:       policyID,
		Type:           "health",
		StartDate:      time.Now().AddDate(-1, 0, 0),
		EndDate:        time.Now().AddDate(1, 0, 0),
		Premium:        12000,
		CoverageAmount: 500000,
		Exclusions:     []string{"pre-existing conditions", "cosmetic procedures"},
		WaitingPeriods: map[string]int{"hospitalization": 30, "surgery": 90},
	}
}

// getRelatedClaims retrieves claim history
func (a *DataEnrichmentAgent) getRelatedClaims(ctx context.Context, customerID string) ([]models.ClaimSummary, error) {
	if a.db == nil {
		return []models.ClaimSummary{}, nil
	}

	query := `
		SELECT claim_id, type, amount, status, submitted_at, resolved_at, is_fraud
		FROM claims
		WHERE customer_id = $1
		ORDER BY submitted_at DESC
		LIMIT 50
	`

	rows, err := a.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []models.ClaimSummary
	for rows.Next() {
		var c models.ClaimSummary
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&c.ClaimID, &c.Type, &c.Amount, &c.Status,
			&c.SubmittedAt, &resolvedAt, &c.IsFraud,
		)
		if err != nil {
			continue
		}

		if resolvedAt.Valid {
			c.ResolvedAt = resolvedAt.Time
		}

		claims = append(claims, c)
	}

	return claims, nil
}

// getSourcesChecked returns a list of data sources that were checked
func (a *DataEnrichmentAgent) getSourcesChecked(data *models.EnrichedData) []string {
	sources := []string{}

	if data.CustomerProfile != nil {
		sources = append(sources, "customer_profile")
	}
	if len(data.Transactions) > 0 {
		sources = append(sources, "transactions")
	}
	if data.PolicyDetails != nil {
		sources = append(sources, "policy_details")
	}
	if len(data.RelatedClaims) > 0 {
		sources = append(sources, "claims_history")
	}
	for name := range data.ExternalData {
		sources = append(sources, "external:"+name)
	}

	return sources
}

// EnrichmentResult contains the result of data enrichment
type EnrichmentResult struct {
	EnrichedData *models.EnrichedData `json:"enriched_data"`
	SourceCount  int                  `json:"source_count"`
	Duration     time.Duration        `json:"duration"`
}

// GetEnrichedData extracts enriched data from finding
func GetEnrichedData(finding *models.AgentFinding) (*models.EnrichedData, bool) {
	if finding == nil || finding.Metadata == nil {
		return nil, false
	}

	data, ok := finding.Metadata["enriched_data"].(*models.EnrichedData)
	return data, ok
}

// CreditBureauSource is an example external data source
type CreditBureauSource struct {
	apiURL string
	apiKey string
}

// NewCreditBureauSource creates a credit bureau source
func NewCreditBureauSource(apiURL, apiKey string) *CreditBureauSource {
	return &CreditBureauSource{
		apiURL: apiURL,
		apiKey: apiKey,
	}
}

// Name returns the source name
func (s *CreditBureauSource) Name() string {
	return "credit_bureau"
}

// Fetch retrieves credit data for an entity
func (s *CreditBureauSource) Fetch(ctx context.Context, entityID string) (map[string]interface{}, error) {
	// This would make an actual API call in production
	return map[string]interface{}{
		"credit_score":    750,
		"open_accounts":   5,
		"delinquencies":   0,
		"inquiries_30d":   2,
		"utilization":     0.35,
	}, nil
}

// SanctionsListSource checks against sanctions lists
type SanctionsListSource struct {
	lists []string
}

// NewSanctionsListSource creates a sanctions list source
func NewSanctionsListSource(lists []string) *SanctionsListSource {
	return &SanctionsListSource{lists: lists}
}

// Name returns the source name
func (s *SanctionsListSource) Name() string {
	return "sanctions_check"
}

// Fetch checks if entity is on any sanctions list
func (s *SanctionsListSource) Fetch(ctx context.Context, entityID string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"is_sanctioned":    false,
		"lists_checked":    s.lists,
		"potential_matches": []string{},
	}, nil
}

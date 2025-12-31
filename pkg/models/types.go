// Package models defines core data types for the fraud investigation RAG framework
package models

import (
	"time"

	"github.com/google/uuid"
)

// AgentType represents different specialized agent types in the system
type AgentType string

const (
	AgentTypeOrchestrator     AgentType = "orchestrator"
	AgentTypeDataEnrichment   AgentType = "data_enrichment"
	AgentTypeEntityResolution AgentType = "entity_resolution"
	AgentTypePatternAnalysis  AgentType = "pattern_analysis"
	AgentTypeDocumentAnalysis AgentType = "document_analysis"
	AgentTypePolicyCompliance AgentType = "policy_compliance"
	AgentTypeNetworkAnalysis  AgentType = "network_analysis"
	AgentTypeTemporalAnalysis AgentType = "temporal_analysis"
	AgentTypeRiskAssessment   AgentType = "risk_assessment"
	AgentTypeEvidence         AgentType = "evidence_compilation"
	AgentTypeExplanation      AgentType = "explanation"
)

// StoreType represents different RAG knowledge store types
type StoreType string

const (
	StorePolicyDocs       StoreType = "policy_docs"
	StoreFraudCases       StoreType = "fraud_cases"
	StoreEntityKnowledge  StoreType = "entity_knowledge"
	StorePlaybooks        StoreType = "playbooks"
	StoreExternalKnowledge StoreType = "external_knowledge"
)

// InvestigationStatus represents the current status of an investigation
type InvestigationStatus string

const (
	StatusPending    InvestigationStatus = "pending"
	StatusInProgress InvestigationStatus = "in_progress"
	StatusReview     InvestigationStatus = "human_review"
	StatusCompleted  InvestigationStatus = "completed"
	StatusEscalated  InvestigationStatus = "escalated"
)

// RiskLevel represents fraud risk classification
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// FraudAlertInput represents incoming fraud alert for investigation
type FraudAlertInput struct {
	ID              string                 `json:"id"`
	ClaimID         string                 `json:"claim_id"`
	PolicyID        string                 `json:"policy_id"`
	CustomerID      string                 `json:"customer_id"`
	ClaimType       string                 `json:"claim_type"`
	Amount          float64                `json:"amount"`
	Description     string                 `json:"description"`
	SubmittedAt     time.Time              `json:"submitted_at"`
	AlertTriggers   []string               `json:"alert_triggers"`
	Priority        int                    `json:"priority"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// Transaction represents a financial transaction for analysis
type Transaction struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	Timestamp       time.Time              `json:"timestamp"`
	SourceAccount   string                 `json:"source_account"`
	TargetAccount   string                 `json:"target_account"`
	Location        *GeoLocation           `json:"location,omitempty"`
	DeviceInfo      *DeviceInfo            `json:"device_info,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// GeoLocation represents geographic coordinates
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	IPAddress string  `json:"ip_address,omitempty"`
}

// DeviceInfo represents device fingerprint information
type DeviceInfo struct {
	DeviceID    string `json:"device_id"`
	DeviceType  string `json:"device_type"`
	OS          string `json:"os"`
	Browser     string `json:"browser,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// CustomerProfile represents customer baseline information
type CustomerProfile struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Email              string                 `json:"email"`
	Phone              string                 `json:"phone"`
	Address            *Address               `json:"address"`
	AccountOpenDate    time.Time              `json:"account_open_date"`
	RiskScore          float64                `json:"risk_score"`
	AverageTransaction float64                `json:"average_transaction"`
	TransactionCount   int                    `json:"transaction_count"`
	ClaimHistory       []ClaimSummary         `json:"claim_history"`
	RelatedEntities    []string               `json:"related_entities"`
	Metadata           map[string]interface{} `json:"metadata"`
}

// Address represents a physical address
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// ClaimSummary represents a summary of a previous claim
type ClaimSummary struct {
	ClaimID     string    `json:"claim_id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
	IsFraud     bool      `json:"is_fraud"`
}

// EnrichedData contains all gathered context for investigation
type EnrichedData struct {
	CustomerProfile  *CustomerProfile       `json:"customer_profile"`
	Transactions     []Transaction          `json:"transactions"`
	RelatedClaims    []ClaimSummary         `json:"related_claims"`
	PolicyDetails    *PolicyDetails         `json:"policy_details"`
	ExternalData     map[string]interface{} `json:"external_data"`
	EnrichmentTime   time.Duration          `json:"enrichment_time"`
}

// PolicyDetails represents insurance policy information
type PolicyDetails struct {
	PolicyID        string                 `json:"policy_id"`
	Type            string                 `json:"type"`
	StartDate       time.Time              `json:"start_date"`
	EndDate         time.Time              `json:"end_date"`
	Premium         float64                `json:"premium"`
	CoverageAmount  float64                `json:"coverage_amount"`
	Exclusions      []string               `json:"exclusions"`
	WaitingPeriods  map[string]int         `json:"waiting_periods"`
	Beneficiaries   []Beneficiary          `json:"beneficiaries"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// Beneficiary represents a policy beneficiary
type Beneficiary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Relationship string  `json:"relationship"`
	Percentage   float64 `json:"percentage"`
}

// EntityLink represents a connection between entities
type EntityLink struct {
	SourceID     string                 `json:"source_id"`
	TargetID     string                 `json:"target_id"`
	LinkType     string                 `json:"link_type"`
	Confidence   float64                `json:"confidence"`
	Evidence     []string               `json:"evidence"`
	DiscoveredAt time.Time              `json:"discovered_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// EntityLinks contains all discovered entity relationships
type EntityLinks struct {
	DirectMatches     []EntityLink `json:"direct_matches"`
	TransitiveLinks   []EntityLink `json:"transitive_links"`
	HouseholdClusters []Cluster    `json:"household_clusters"`
	MovementPatterns  []Movement   `json:"movement_patterns"`
}

// Cluster represents a group of related entities
type Cluster struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Members  []string `json:"members"`
	Strength float64  `json:"strength"`
}

// Movement represents entity movement/relocation pattern
type Movement struct {
	EntityID    string    `json:"entity_id"`
	FromAddress *Address  `json:"from_address"`
	ToAddress   *Address  `json:"to_address"`
	MovedAt     time.Time `json:"moved_at"`
	IsRecent    bool      `json:"is_recent"`
}

// AgentFinding represents findings from a single agent
type AgentFinding struct {
	AgentType      AgentType              `json:"agent_type"`
	Confidence     float64                `json:"confidence"`
	RiskIndicators []RiskIndicator        `json:"risk_indicators"`
	Evidence       []Evidence             `json:"evidence"`
	Recommendations []string              `json:"recommendations"`
	RAGSources     []RAGSource            `json:"rag_sources"`
	ProcessingTime time.Duration          `json:"processing_time"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// RiskIndicator represents a specific fraud risk signal
type RiskIndicator struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	Score       float64 `json:"score"`
	Category    string  `json:"category"`
}

// Evidence represents supporting evidence for findings
type Evidence struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Timestamp   time.Time              `json:"timestamp"`
	Confidence  float64                `json:"confidence"`
	Data        map[string]interface{} `json:"data"`
}

// RAGSource represents a source document from RAG retrieval
type RAGSource struct {
	StoreType  StoreType `json:"store_type"`
	DocumentID string    `json:"document_id"`
	Chunk      string    `json:"chunk"`
	Similarity float64   `json:"similarity"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// AgentFindings aggregates findings from all agents
type AgentFindings struct {
	Pattern   *AgentFinding `json:"pattern,omitempty"`
	Network   *AgentFinding `json:"network,omitempty"`
	Temporal  *AgentFinding `json:"temporal,omitempty"`
	Document  *AgentFinding `json:"document,omitempty"`
	Policy    *AgentFinding `json:"policy,omitempty"`
	Entity    *AgentFinding `json:"entity,omitempty"`
}

// RiskAssessment contains the final risk evaluation
type RiskAssessment struct {
	Score              float64               `json:"score"`
	Level              RiskLevel             `json:"level"`
	Confidence         float64               `json:"confidence"`
	ContributingFactors []ContributingFactor `json:"contributing_factors"`
	RecommendedAction  string                `json:"recommended_action"`
	RequiresHumanReview bool                 `json:"requires_human_review"`
	Explanation        string                `json:"explanation"`
}

// ContributingFactor represents a factor influencing the risk score
type ContributingFactor struct {
	Factor      string  `json:"factor"`
	Weight      float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	Description string  `json:"description"`
}

// HumanDecision represents a human reviewer's decision
type HumanDecision struct {
	ReviewerID   string                 `json:"reviewer_id"`
	Action       string                 `json:"action"`
	Reason       string                 `json:"reason"`
	DecidedAt    time.Time              `json:"decided_at"`
	Notes        string                 `json:"notes"`
	Adjustments  map[string]interface{} `json:"adjustments"`
}

// InvestigationState tracks the complete state of an investigation
type InvestigationState struct {
	ID              uuid.UUID              `json:"id"`
	AlertInput      *FraudAlertInput       `json:"alert_input"`
	Status          InvestigationStatus    `json:"status"`
	EnrichedData    *EnrichedData          `json:"enriched_data,omitempty"`
	EntityLinks     *EntityLinks           `json:"entity_links,omitempty"`
	Findings        *AgentFindings         `json:"findings,omitempty"`
	RiskAssessment  *RiskAssessment        `json:"risk_assessment,omitempty"`
	HumanDecision   *HumanDecision         `json:"human_decision,omitempty"`
	FinalDecision   string                 `json:"final_decision"`
	AuditTrail      []AuditEntry           `json:"audit_trail"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
}

// AuditEntry represents an entry in the audit trail
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Action      string                 `json:"action"`
	AgentType   AgentType              `json:"agent_type,omitempty"`
	Details     string                 `json:"details"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// InvestigationReport is the final compiled report
type InvestigationReport struct {
	ID               string                 `json:"id"`
	InvestigationID  uuid.UUID              `json:"investigation_id"`
	Summary          string                 `json:"summary"`
	Timeline         []TimelineEvent        `json:"timeline"`
	EntityDiagram    *EntityDiagram         `json:"entity_diagram,omitempty"`
	EvidencePackage  []Evidence             `json:"evidence_package"`
	AgentSummaries   map[AgentType]string   `json:"agent_summaries"`
	RiskAssessment   *RiskAssessment        `json:"risk_assessment"`
	Recommendations  []Recommendation       `json:"recommendations"`
	GeneratedAt      time.Time              `json:"generated_at"`
}

// TimelineEvent represents an event in the investigation timeline
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Event       string    `json:"event"`
	Description string    `json:"description"`
	Significance string   `json:"significance"`
}

// EntityDiagram represents relationships for visualization
type EntityDiagram struct {
	Nodes []EntityNode `json:"nodes"`
	Edges []EntityEdge `json:"edges"`
}

// EntityNode represents a node in the entity graph
type EntityNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Risk  string `json:"risk,omitempty"`
}

// EntityEdge represents an edge in the entity graph
type EntityEdge struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight,omitempty"`
}

// Recommendation represents an action recommendation
type Recommendation struct {
	Action    string `json:"action"`
	Priority  int    `json:"priority"`
	Rationale string `json:"rationale"`
}

// InvestigationResult is the final output of the investigation workflow
type InvestigationResult struct {
	InvestigationID uuid.UUID            `json:"investigation_id"`
	RiskScore       float64              `json:"risk_score"`
	RiskLevel       RiskLevel            `json:"risk_level"`
	Decision        string               `json:"decision"`
	Report          *InvestigationReport `json:"report"`
	AuditTrail      []AuditEntry         `json:"audit_trail"`
	ProcessingTime  time.Duration        `json:"processing_time"`
}

// NewInvestigationState creates a new investigation state from alert input
func NewInvestigationState(input *FraudAlertInput) *InvestigationState {
	return &InvestigationState{
		ID:         uuid.New(),
		AlertInput: input,
		Status:     StatusPending,
		AuditTrail: []AuditEntry{
			{
				Timestamp: time.Now(),
				Action:    "investigation_created",
				Details:   "Investigation initiated from fraud alert",
			},
		},
		StartedAt: time.Now(),
	}
}

// AddAuditEntry adds an entry to the audit trail
func (s *InvestigationState) AddAuditEntry(action string, agentType AgentType, details string) {
	s.AuditTrail = append(s.AuditTrail, AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		AgentType: agentType,
		Details:   details,
	})
}

// UpdateStatus updates the investigation status
func (s *InvestigationState) UpdateStatus(status InvestigationStatus) {
	s.Status = status
	s.AddAuditEntry("status_changed", "", "Status changed to "+string(status))
}

// Complete marks the investigation as completed
func (s *InvestigationState) Complete(decision string) {
	now := time.Now()
	s.CompletedAt = &now
	s.FinalDecision = decision
	s.Status = StatusCompleted
	s.AddAuditEntry("investigation_completed", "", "Final decision: "+decision)
}

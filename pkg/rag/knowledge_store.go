// Package rag provides knowledge store management for RAG
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/vectorstore"
)

// KnowledgeStoreManager manages RAG knowledge stores
type KnowledgeStoreManager struct {
	stores     *vectorstore.PgVectorMultiStore
	embedder   vectorstore.EmbeddingProvider
	processors map[models.StoreType]vectorstore.DocumentProcessor
}

// NewKnowledgeStoreManager creates a new knowledge store manager
func NewKnowledgeStoreManager(stores *vectorstore.PgVectorMultiStore, embedder vectorstore.EmbeddingProvider) *KnowledgeStoreManager {
	mgr := &KnowledgeStoreManager{
		stores:     stores,
		embedder:   embedder,
		processors: make(map[models.StoreType]vectorstore.DocumentProcessor),
	}

	// Set up default processors
	textProcessor := vectorstore.NewTextProcessor(embedder, nil)
	mgr.processors[models.StoreFraudCases] = vectorstore.NewFraudCaseProcessor(embedder)
	mgr.processors[models.StorePolicyDocs] = vectorstore.NewPolicyDocumentProcessor(embedder)
	mgr.processors[models.StoreEntityKnowledge] = textProcessor
	mgr.processors[models.StorePlaybooks] = textProcessor
	mgr.processors[models.StoreExternalKnowledge] = textProcessor

	return mgr
}

// SetProcessor sets a custom processor for a store type
func (m *KnowledgeStoreManager) SetProcessor(storeType models.StoreType, processor vectorstore.DocumentProcessor) {
	m.processors[storeType] = processor
}

// IndexDocument indexes a single document
func (m *KnowledgeStoreManager) IndexDocument(ctx context.Context, storeType models.StoreType, content string, metadata map[string]interface{}) error {
	doc := &vectorstore.Document{
		ID:        uuid.New().String(),
		Content:   content,
		Metadata:  metadata,
		StoreType: storeType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	processor, ok := m.processors[storeType]
	if !ok {
		processor = vectorstore.NewTextProcessor(m.embedder, nil)
	}

	chunks, err := processor.Process(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to process document: %w", err)
	}

	store, err := m.stores.GetStore(storeType)
	if err != nil {
		return fmt.Errorf("failed to get store: %w", err)
	}

	return store.Insert(ctx, chunks)
}

// IndexFile indexes a file from disk
func (m *KnowledgeStoreManager) IndexFile(ctx context.Context, storeType models.StoreType, filePath string, metadata map[string]interface{}) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["source_file"] = filepath.Base(filePath)
	metadata["file_path"] = filePath

	return m.IndexDocument(ctx, storeType, string(content), metadata)
}

// IndexDirectory indexes all files in a directory
func (m *KnowledgeStoreManager) IndexDirectory(ctx context.Context, storeType models.StoreType, dirPath string, extensions []string) (int, error) {
	count := 0

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(path))
		if len(extensions) > 0 && !containsExt(extensions, ext) {
			return nil
		}

		if err := m.IndexFile(ctx, storeType, path, nil); err != nil {
			return err
		}

		count++
		return nil
	})

	return count, err
}

// containsExt checks if extension is in list
func containsExt(extensions []string, ext string) bool {
	for _, e := range extensions {
		if e == ext {
			return true
		}
	}
	return false
}

// IndexFraudCase indexes a fraud case from structured data
func (m *KnowledgeStoreManager) IndexFraudCase(ctx context.Context, fraudCase *FraudCaseDocument) error {
	content := fmt.Sprintf(`Fraud Case: %s
Type: %s
Date: %s

Description:
%s

Modus Operandi:
%s

Indicators:
%s

Resolution:
%s`,
		fraudCase.CaseID,
		fraudCase.FraudType,
		fraudCase.Date,
		fraudCase.Description,
		fraudCase.ModusOperandi,
		strings.Join(fraudCase.Indicators, "\n- "),
		fraudCase.Resolution,
	)

	metadata := map[string]interface{}{
		"case_id":     fraudCase.CaseID,
		"fraud_type":  fraudCase.FraudType,
		"date":        fraudCase.Date,
		"severity":    fraudCase.Severity,
		"industry":    fraudCase.Industry,
	}

	return m.IndexDocument(ctx, models.StoreFraudCases, content, metadata)
}

// FraudCaseDocument represents a fraud case for indexing
type FraudCaseDocument struct {
	CaseID        string   `json:"case_id"`
	FraudType     string   `json:"fraud_type"`
	Date          string   `json:"date"`
	Description   string   `json:"description"`
	ModusOperandi string   `json:"modus_operandi"`
	Indicators    []string `json:"indicators"`
	Resolution    string   `json:"resolution"`
	Severity      string   `json:"severity"`
	Industry      string   `json:"industry"`
}

// IndexPolicy indexes a policy document
func (m *KnowledgeStoreManager) IndexPolicy(ctx context.Context, policy *PolicyDocument) error {
	content := fmt.Sprintf(`Policy: %s
Type: %s
Version: %s

Terms and Conditions:
%s

Coverage:
%s

Exclusions:
%s

Waiting Periods:
%s`,
		policy.PolicyName,
		policy.PolicyType,
		policy.Version,
		policy.TermsAndConditions,
		policy.Coverage,
		strings.Join(policy.Exclusions, "\n- "),
		formatWaitingPeriods(policy.WaitingPeriods),
	)

	metadata := map[string]interface{}{
		"policy_name": policy.PolicyName,
		"policy_type": policy.PolicyType,
		"version":     policy.Version,
		"effective":   policy.EffectiveDate,
	}

	return m.IndexDocument(ctx, models.StorePolicyDocs, content, metadata)
}

// PolicyDocument represents a policy document for indexing
type PolicyDocument struct {
	PolicyName         string         `json:"policy_name"`
	PolicyType         string         `json:"policy_type"`
	Version            string         `json:"version"`
	EffectiveDate      string         `json:"effective_date"`
	TermsAndConditions string         `json:"terms_and_conditions"`
	Coverage           string         `json:"coverage"`
	Exclusions         []string       `json:"exclusions"`
	WaitingPeriods     map[string]int `json:"waiting_periods"`
}

// formatWaitingPeriods formats waiting periods for display
func formatWaitingPeriods(periods map[string]int) string {
	var lines []string
	for k, v := range periods {
		lines = append(lines, fmt.Sprintf("- %s: %d days", k, v))
	}
	return strings.Join(lines, "\n")
}

// IndexPlaybook indexes an investigation playbook
func (m *KnowledgeStoreManager) IndexPlaybook(ctx context.Context, playbook *PlaybookDocument) error {
	content := fmt.Sprintf(`Playbook: %s
Category: %s
Version: %s

Purpose:
%s

Steps:
%s

Questions to Ask:
%s

Evidence to Collect:
%s

Red Flags:
%s`,
		playbook.Name,
		playbook.Category,
		playbook.Version,
		playbook.Purpose,
		formatSteps(playbook.Steps),
		strings.Join(playbook.Questions, "\n- "),
		strings.Join(playbook.EvidenceToCollect, "\n- "),
		strings.Join(playbook.RedFlags, "\n- "),
	)

	metadata := map[string]interface{}{
		"playbook_name": playbook.Name,
		"category":      playbook.Category,
		"version":       playbook.Version,
	}

	return m.IndexDocument(ctx, models.StorePlaybooks, content, metadata)
}

// PlaybookDocument represents an investigation playbook
type PlaybookDocument struct {
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	Version           string   `json:"version"`
	Purpose           string   `json:"purpose"`
	Steps             []string `json:"steps"`
	Questions         []string `json:"questions"`
	EvidenceToCollect []string `json:"evidence_to_collect"`
	RedFlags          []string `json:"red_flags"`
}

// formatSteps formats playbook steps
func formatSteps(steps []string) string {
	var lines []string
	for i, step := range steps {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
	}
	return strings.Join(lines, "\n")
}

// IndexEntityKnowledge indexes entity knowledge
func (m *KnowledgeStoreManager) IndexEntityKnowledge(ctx context.Context, entity *EntityKnowledgeDocument) error {
	content := fmt.Sprintf(`Entity: %s
Type: %s

Description:
%s

Known Associations:
%s

Risk Indicators:
%s

Notes:
%s`,
		entity.EntityID,
		entity.EntityType,
		entity.Description,
		strings.Join(entity.Associations, "\n- "),
		strings.Join(entity.RiskIndicators, "\n- "),
		entity.Notes,
	)

	metadata := map[string]interface{}{
		"entity_id":   entity.EntityID,
		"entity_type": entity.EntityType,
		"risk_level":  entity.RiskLevel,
	}

	return m.IndexDocument(ctx, models.StoreEntityKnowledge, content, metadata)
}

// EntityKnowledgeDocument represents entity knowledge
type EntityKnowledgeDocument struct {
	EntityID       string   `json:"entity_id"`
	EntityType     string   `json:"entity_type"`
	Description    string   `json:"description"`
	Associations   []string `json:"associations"`
	RiskIndicators []string `json:"risk_indicators"`
	RiskLevel      string   `json:"risk_level"`
	Notes          string   `json:"notes"`
}

// BulkIndexFromJSON indexes documents from a JSON file
func (m *KnowledgeStoreManager) BulkIndexFromJSON(ctx context.Context, storeType models.StoreType, reader io.Reader) (int, error) {
	var documents []struct {
		Content  string                 `json:"content"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(reader).Decode(&documents); err != nil {
		return 0, err
	}

	count := 0
	for _, doc := range documents {
		if err := m.IndexDocument(ctx, storeType, doc.Content, doc.Metadata); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// DeleteDocument removes a document by ID
func (m *KnowledgeStoreManager) DeleteDocument(ctx context.Context, storeType models.StoreType, documentID string) error {
	store, err := m.stores.GetStore(storeType)
	if err != nil {
		return err
	}

	return store.Delete(ctx, []string{documentID})
}

// Search performs a search across specified stores
func (m *KnowledgeStoreManager) Search(ctx context.Context, query string, storeTypes []models.StoreType, limit int) ([]*vectorstore.SearchResult, error) {
	queryEmbedding, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	return m.stores.SearchAcrossStores(ctx, queryEmbedding, storeTypes, &vectorstore.SearchOptions{
		Limit:    limit,
		MinScore: 0.6,
	})
}

// GetStoreStats returns statistics for a store
func (m *KnowledgeStoreManager) GetStoreStats(ctx context.Context, storeType models.StoreType) (*StoreStats, error) {
	// This would query the database for counts
	return &StoreStats{
		StoreType:     storeType,
		DocumentCount: 0, // Would query
		LastUpdated:   time.Now(),
	}, nil
}

// StoreStats contains store statistics
type StoreStats struct {
	StoreType     models.StoreType `json:"store_type"`
	DocumentCount int              `json:"document_count"`
	LastUpdated   time.Time        `json:"last_updated"`
}

// SyncFromConfluence syncs knowledge from Confluence (placeholder)
func (m *KnowledgeStoreManager) SyncFromConfluence(ctx context.Context, config *ConfluenceConfig) error {
	// Implementation would:
	// 1. Connect to Confluence API
	// 2. Fetch pages from specified spaces
	// 3. Convert to markdown
	// 4. Index documents
	return nil
}

// ConfluenceConfig configures Confluence sync
type ConfluenceConfig struct {
	BaseURL  string
	Username string
	APIToken string
	Spaces   []string
}

// UpdateFeedback incorporates feedback into knowledge
func (m *KnowledgeStoreManager) UpdateFeedback(ctx context.Context, feedback *InvestigationFeedback) error {
	// Use feedback to improve RAG
	if feedback.WasCorrect {
		// Strengthen patterns that led to correct decision
		return nil
	}

	// Learn from incorrect decisions
	content := fmt.Sprintf(`Feedback: Investigation %s
Original Decision: %s
Correct Decision: %s
Reason: %s

Lessons Learned:
%s`,
		feedback.InvestigationID,
		feedback.OriginalDecision,
		feedback.CorrectDecision,
		feedback.Reason,
		strings.Join(feedback.LessonsLearned, "\n- "),
	)

	return m.IndexDocument(ctx, models.StoreFraudCases, content, map[string]interface{}{
		"type":     "feedback",
		"feedback": true,
	})
}

// InvestigationFeedback contains feedback on investigation outcomes
type InvestigationFeedback struct {
	InvestigationID  string   `json:"investigation_id"`
	WasCorrect       bool     `json:"was_correct"`
	OriginalDecision string   `json:"original_decision"`
	CorrectDecision  string   `json:"correct_decision"`
	Reason           string   `json:"reason"`
	LessonsLearned   []string `json:"lessons_learned"`
}

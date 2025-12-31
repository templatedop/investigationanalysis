// Package agents provides the document analysis agent
package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// DocumentAnalysisAgent analyzes supporting documents for authenticity
type DocumentAnalysisAgent struct {
	*BaseAgent
	ocrProcessor    OCRProcessor
	forgeryDetector ForgeryDetector
}

// OCRProcessor interface for OCR operations
type OCRProcessor interface {
	ExtractText(ctx context.Context, documentPath string) (*OCRResult, error)
	ExtractMetadata(ctx context.Context, documentPath string) (*DocumentMetadata, error)
}

// ForgeryDetector interface for forgery detection
type ForgeryDetector interface {
	Analyze(ctx context.Context, documentPath string) (*ForgeryAnalysis, error)
}

// OCRResult contains OCR extraction results
type OCRResult struct {
	Text       string            `json:"text"`
	Confidence float64           `json:"confidence"`
	Fields     map[string]string `json:"fields"`
	Language   string            `json:"language"`
}

// DocumentMetadata contains document metadata
type DocumentMetadata struct {
	CreationDate   time.Time         `json:"creation_date"`
	ModifiedDate   time.Time         `json:"modified_date"`
	Author         string            `json:"author"`
	Creator        string            `json:"creator"`
	Producer       string            `json:"producer"`
	PageCount      int               `json:"page_count"`
	FileSize       int64             `json:"file_size"`
	FileType       string            `json:"file_type"`
	CustomMetadata map[string]string `json:"custom_metadata"`
}

// ForgeryAnalysis contains forgery detection results
type ForgeryAnalysis struct {
	IsSuspicious     bool                   `json:"is_suspicious"`
	Confidence       float64                `json:"confidence"`
	Indicators       []ForgeryIndicator     `json:"indicators"`
	TemplateMatch    *TemplateMatch         `json:"template_match,omitempty"`
}

// ForgeryIndicator represents a forgery indicator
type ForgeryIndicator struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	Location    string  `json:"location,omitempty"`
}

// TemplateMatch indicates if document matches known templates
type TemplateMatch struct {
	TemplateName string  `json:"template_name"`
	Similarity   float64 `json:"similarity"`
	IsKnownForged bool   `json:"is_known_forged"`
}

// NewDocumentAnalysisAgent creates a new document analysis agent
func NewDocumentAnalysisAgent(llmClient *llm.Client, ragSystem *rag.System) *DocumentAnalysisAgent {
	return &DocumentAnalysisAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeDocumentAnalysis,
			"Document Analysis Agent",
			"Analyzes supporting documents for authenticity and consistency",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "medium", // Could use vision model
			},
		),
	}
}

// SetOCRProcessor sets the OCR processor
func (a *DocumentAnalysisAgent) SetOCRProcessor(processor OCRProcessor) {
	a.ocrProcessor = processor
}

// SetForgeryDetector sets the forgery detector
func (a *DocumentAnalysisAgent) SetForgeryDetector(detector ForgeryDetector) {
	a.forgeryDetector = detector
}

// Execute performs document analysis
func (a *DocumentAnalysisAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	// Get document paths from context
	documents := a.extractDocumentReferences(analysisCtx)

	// Analyze documents
	analysis := &DocumentAnalysisResult{
		Documents: make([]DocumentResult, 0),
	}

	for _, doc := range documents {
		result := a.analyzeDocument(ctx, doc)
		analysis.Documents = append(analysis.Documents, result)
	}

	// Cross-document analysis
	analysis.CrossDocumentIssues = a.analyzeCrossDocument(analysis.Documents)

	// Get RAG context for known document patterns
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"document forgery patterns",
		"medical report verification",
		"invoice fraud detection",
	})

	// Generate findings
	indicators, evidence := a.generateDocumentFindings(analysis)

	finding := a.CreateFinding(
		a.calculateDocumentConfidence(analysis),
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["document_analysis"] = analysis
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// DocumentAnalysisResult contains overall document analysis
type DocumentAnalysisResult struct {
	Documents           []DocumentResult     `json:"documents"`
	CrossDocumentIssues []CrossDocumentIssue `json:"cross_document_issues"`
}

// DocumentResult contains single document analysis
type DocumentResult struct {
	DocumentID      string            `json:"document_id"`
	DocumentType    string            `json:"document_type"`
	Provider        string            `json:"provider,omitempty"`
	OCRResult       *OCRResult        `json:"ocr_result,omitempty"`
	Metadata        *DocumentMetadata `json:"metadata,omitempty"`
	ForgeryAnalysis *ForgeryAnalysis  `json:"forgery_analysis,omitempty"`
	ContentIssues   []ContentIssue    `json:"content_issues"`
	OverallRisk     float64           `json:"overall_risk"`
}

// ContentIssue represents an issue with document content
type ContentIssue struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	Field       string  `json:"field,omitempty"`
	Expected    string  `json:"expected,omitempty"`
	Found       string  `json:"found,omitempty"`
}

// CrossDocumentIssue represents inconsistencies across documents
type CrossDocumentIssue struct {
	DocumentIDs []string `json:"document_ids"`
	IssueType   string   `json:"issue_type"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
}

// DocumentReference contains document path and type
type DocumentReference struct {
	Path string
	Type string
}

// extractDocumentReferences extracts document references from context
func (a *DocumentAnalysisAgent) extractDocumentReferences(analysisCtx *AnalysisContext) []DocumentReference {
	// In production, this would extract document paths from the alert/enriched data
	// For now, return empty list
	return []DocumentReference{}
}

// analyzeDocument performs comprehensive document analysis
func (a *DocumentAnalysisAgent) analyzeDocument(ctx context.Context, doc DocumentReference) DocumentResult {
	result := DocumentResult{
		DocumentID:   doc.Path,
		DocumentType: doc.Type,
		ContentIssues: []ContentIssue{},
	}

	// Run OCR if available
	if a.ocrProcessor != nil {
		ocrResult, err := a.ocrProcessor.ExtractText(ctx, doc.Path)
		if err == nil {
			result.OCRResult = ocrResult
		}

		metadata, err := a.ocrProcessor.ExtractMetadata(ctx, doc.Path)
		if err == nil {
			result.Metadata = metadata

			// Check metadata issues
			issues := a.checkMetadataIssues(metadata)
			result.ContentIssues = append(result.ContentIssues, issues...)
		}
	}

	// Run forgery detection if available
	if a.forgeryDetector != nil {
		forgeryResult, err := a.forgeryDetector.Analyze(ctx, doc.Path)
		if err == nil {
			result.ForgeryAnalysis = forgeryResult
		}
	}

	// Calculate overall risk
	result.OverallRisk = a.calculateDocumentRisk(result)

	return result
}

// checkMetadataIssues analyzes document metadata for issues
func (a *DocumentAnalysisAgent) checkMetadataIssues(metadata *DocumentMetadata) []ContentIssue {
	var issues []ContentIssue

	// Check creation date vs modified date
	if metadata.ModifiedDate.Before(metadata.CreationDate) {
		issues = append(issues, ContentIssue{
			Type:        "metadata_inconsistency",
			Description: "Modified date is before creation date",
			Severity:    "high",
			Field:       "dates",
		})
	}

	// Check for future dates
	if metadata.CreationDate.After(time.Now()) {
		issues = append(issues, ContentIssue{
			Type:        "future_date",
			Description: "Document has future creation date",
			Severity:    "critical",
			Field:       "creation_date",
		})
	}

	// Check for suspicious creator/producer combinations
	suspiciousCreators := []string{"libreoffice", "online converter", "pdf editor"}
	for _, s := range suspiciousCreators {
		if containsIgnoreCase(metadata.Creator, s) || containsIgnoreCase(metadata.Producer, s) {
			issues = append(issues, ContentIssue{
				Type:        "suspicious_creator",
				Description: fmt.Sprintf("Document created with potentially suspicious tool: %s", metadata.Creator),
				Severity:    "medium",
				Field:       "creator",
			})
			break
		}
	}

	return issues
}

// analyzeCrossDocument checks for inconsistencies across documents
func (a *DocumentAnalysisAgent) analyzeCrossDocument(documents []DocumentResult) []CrossDocumentIssue {
	var issues []CrossDocumentIssue

	// Check for template reuse (same document from "different" providers)
	for i := 0; i < len(documents); i++ {
		for j := i + 1; j < len(documents); j++ {
			if a.areDocumentsSuspiciouslySimilar(documents[i], documents[j]) {
				issues = append(issues, CrossDocumentIssue{
					DocumentIDs: []string{documents[i].DocumentID, documents[j].DocumentID},
					IssueType:   "template_reuse",
					Description: "Documents appear to use same template despite different providers",
					Severity:    "high",
				})
			}
		}
	}

	// Check for date inconsistencies across documents
	dateIssues := a.checkCrossDocumentDates(documents)
	issues = append(issues, dateIssues...)

	return issues
}

// areDocumentsSuspiciouslySimilar checks if two documents are too similar
func (a *DocumentAnalysisAgent) areDocumentsSuspiciouslySimilar(doc1, doc2 DocumentResult) bool {
	// Check metadata similarity
	if doc1.Metadata != nil && doc2.Metadata != nil {
		if doc1.Metadata.Creator == doc2.Metadata.Creator &&
			doc1.Metadata.Producer == doc2.Metadata.Producer &&
			doc1.Provider != doc2.Provider {
			return true
		}
	}

	// Check forgery template matches
	if doc1.ForgeryAnalysis != nil && doc2.ForgeryAnalysis != nil {
		if doc1.ForgeryAnalysis.TemplateMatch != nil && doc2.ForgeryAnalysis.TemplateMatch != nil {
			if doc1.ForgeryAnalysis.TemplateMatch.TemplateName == doc2.ForgeryAnalysis.TemplateMatch.TemplateName {
				return true
			}
		}
	}

	return false
}

// checkCrossDocumentDates checks for date inconsistencies
func (a *DocumentAnalysisAgent) checkCrossDocumentDates(documents []DocumentResult) []CrossDocumentIssue {
	var issues []CrossDocumentIssue

	// Extract dates from OCR results and compare
	// This would need document-type specific parsing

	return issues
}

// calculateDocumentRisk computes risk score for a document
func (a *DocumentAnalysisAgent) calculateDocumentRisk(result DocumentResult) float64 {
	score := 0.0
	count := 0

	// Weight content issues
	for _, issue := range result.ContentIssues {
		switch issue.Severity {
		case "critical":
			score += 0.95
		case "high":
			score += 0.8
		case "medium":
			score += 0.5
		case "low":
			score += 0.3
		}
		count++
	}

	// Weight forgery analysis
	if result.ForgeryAnalysis != nil {
		if result.ForgeryAnalysis.IsSuspicious {
			score += result.ForgeryAnalysis.Confidence
			count++
		}

		for _, indicator := range result.ForgeryAnalysis.Indicators {
			score += indicator.Confidence
			count++
		}
	}

	if count == 0 {
		return 0.1 // Low risk if no issues
	}

	return score / float64(count)
}

// generateDocumentFindings creates indicators and evidence
func (a *DocumentAnalysisAgent) generateDocumentFindings(analysis *DocumentAnalysisResult) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	for _, doc := range analysis.Documents {
		// Add content issues
		for _, issue := range doc.ContentIssues {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "DOC001",
				Description: issue.Description,
				Severity:    issue.Severity,
				Score:       a.severityToScore(issue.Severity),
				Category:    "document",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "document_issue",
				Description: fmt.Sprintf("Document %s: %s", doc.DocumentID, issue.Description),
				Source:      "document_analysis_agent",
				Timestamp:   time.Now(),
				Confidence:  0.85,
			})
		}

		// Add forgery indicators
		if doc.ForgeryAnalysis != nil && doc.ForgeryAnalysis.IsSuspicious {
			indicators = append(indicators, models.RiskIndicator{
				Code:        "DOC002",
				Description: "Document shows signs of manipulation",
				Severity:    "high",
				Score:       doc.ForgeryAnalysis.Confidence,
				Category:    "forgery",
			})
			evidence = append(evidence, models.Evidence{
				Type:        "forgery_detection",
				Description: fmt.Sprintf("Forgery indicators found in %s", doc.DocumentID),
				Source:      "document_analysis_agent",
				Timestamp:   time.Now(),
				Confidence:  doc.ForgeryAnalysis.Confidence,
			})
		}
	}

	// Add cross-document issues
	for _, issue := range analysis.CrossDocumentIssues {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "DOC003",
			Description: issue.Description,
			Severity:    issue.Severity,
			Score:       a.severityToScore(issue.Severity),
			Category:    "document_consistency",
		})
		evidence = append(evidence, models.Evidence{
			Type:        "cross_document_issue",
			Description: issue.Description,
			Source:      "document_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  0.9,
		})
	}

	return indicators, evidence
}

// severityToScore converts severity to numeric score
func (a *DocumentAnalysisAgent) severityToScore(severity string) float64 {
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

// calculateDocumentConfidence computes overall confidence
func (a *DocumentAnalysisAgent) calculateDocumentConfidence(analysis *DocumentAnalysisResult) float64 {
	if len(analysis.Documents) == 0 {
		return 0.5 // Medium confidence with no documents
	}

	totalConfidence := 0.0
	for _, doc := range analysis.Documents {
		if doc.OCRResult != nil {
			totalConfidence += doc.OCRResult.Confidence
		} else {
			totalConfidence += 0.7 // Default confidence
		}
	}

	return totalConfidence / float64(len(analysis.Documents))
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	// Simple implementation - in production use strings.Contains with ToLower
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalIgnoreCase compares strings case-insensitively
func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if toLower(a[i]) != toLower(b[i]) {
			return false
		}
	}
	return true
}

// toLower converts a byte to lowercase
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

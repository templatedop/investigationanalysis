// Package vectorstore provides document processing utilities
package vectorstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

// DefaultChunkSize is the default chunk size in characters
const DefaultChunkSize = 512

// DefaultChunkOverlap is the default overlap between chunks
const DefaultChunkOverlap = 50

// TextProcessor implements DocumentProcessor for text documents
type TextProcessor struct {
	embedder   EmbeddingProvider
	chunkSize  int
	overlap    int
}

// TextProcessorConfig configures the text processor
type TextProcessorConfig struct {
	ChunkSize int
	Overlap   int
}

// NewTextProcessor creates a new text processor
func NewTextProcessor(embedder EmbeddingProvider, cfg *TextProcessorConfig) *TextProcessor {
	if cfg == nil {
		cfg = &TextProcessorConfig{
			ChunkSize: DefaultChunkSize,
			Overlap:   DefaultChunkOverlap,
		}
	}

	return &TextProcessor{
		embedder:  embedder,
		chunkSize: cfg.ChunkSize,
		overlap:   cfg.Overlap,
	}
}

// Process splits and prepares document for indexing
func (p *TextProcessor) Process(ctx context.Context, doc *Document) ([]*Document, error) {
	// Clean and normalize text
	cleanedText := p.cleanText(doc.Content)

	// Split into chunks
	chunks := p.Chunk(cleanedText, p.chunkSize, p.overlap)

	if len(chunks) == 0 {
		return nil, nil
	}

	// Generate embeddings for all chunks
	embeddings, err := p.embedder.EmbedBatch(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create document for each chunk
	var docs []*Document
	for i, chunk := range chunks {
		chunkID := p.generateChunkID(doc.ID, i)

		chunkDoc := &Document{
			ID:        chunkID,
			Content:   chunk,
			Embedding: embeddings[i],
			Metadata:  p.enrichMetadata(doc.Metadata, i, len(chunks)),
			StoreType: doc.StoreType,
		}

		docs = append(docs, chunkDoc)
	}

	return docs, nil
}

// Chunk splits text into overlapping chunks
func (p *TextProcessor) Chunk(text string, chunkSize, overlap int) []string {
	if len(text) == 0 {
		return nil
	}

	// Try to split on sentence boundaries
	sentences := p.splitSentences(text)

	var chunks []string
	var currentChunk strings.Builder
	var currentSize int

	for _, sentence := range sentences {
		sentenceLen := len(sentence)

		if currentSize+sentenceLen > chunkSize && currentSize > 0 {
			// Save current chunk
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))

			// Start new chunk with overlap
			overlapText := p.getOverlapText(currentChunk.String(), overlap)
			currentChunk.Reset()
			currentChunk.WriteString(overlapText)
			currentSize = len(overlapText)
		}

		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")
		currentSize += sentenceLen + 1
	}

	// Add remaining text
	if currentSize > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// ChunkByParagraph splits text by paragraphs
func (p *TextProcessor) ChunkByParagraph(text string) []string {
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)

	var chunks []string
	for _, para := range paragraphs {
		cleaned := strings.TrimSpace(para)
		if len(cleaned) > 0 {
			chunks = append(chunks, cleaned)
		}
	}

	return chunks
}

// ChunkWithHeaders preserves document structure
func (p *TextProcessor) ChunkWithHeaders(text string) []string {
	headerPattern := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	lines := strings.Split(text, "\n")

	var chunks []string
	var currentChunk strings.Builder
	var currentHeader string

	for _, line := range lines {
		if headerPattern.MatchString(line) {
			// Save previous chunk
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			}

			// Start new chunk with header
			currentChunk.Reset()
			currentHeader = line
			currentChunk.WriteString(currentHeader)
			currentChunk.WriteString("\n")
		} else {
			currentChunk.WriteString(line)
			currentChunk.WriteString("\n")
		}
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// cleanText normalizes and cleans text
func (p *TextProcessor) cleanText(text string) string {
	// Replace multiple whitespace with single space
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove control characters
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)

	return strings.TrimSpace(text)
}

// splitSentences splits text into sentences
func (p *TextProcessor) splitSentences(text string) []string {
	// Simple sentence splitter
	sentenceEnders := regexp.MustCompile(`([.!?])\s+`)
	parts := sentenceEnders.Split(text, -1)

	var sentences []string
	matches := sentenceEnders.FindAllStringSubmatch(text, -1)

	for i, part := range parts {
		if len(part) == 0 {
			continue
		}

		sentence := part
		if i < len(matches) {
			sentence += matches[i][1]
		}

		sentences = append(sentences, strings.TrimSpace(sentence))
	}

	return sentences
}

// getOverlapText returns the last N characters for overlap
func (p *TextProcessor) getOverlapText(text string, overlap int) string {
	if len(text) <= overlap {
		return text
	}

	// Try to break at word boundary
	overlapText := text[len(text)-overlap:]
	spaceIdx := strings.Index(overlapText, " ")
	if spaceIdx > 0 {
		overlapText = overlapText[spaceIdx+1:]
	}

	return overlapText
}

// generateChunkID creates a unique ID for a chunk
func (p *TextProcessor) generateChunkID(parentID string, chunkIndex int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", parentID, chunkIndex)))
	return hex.EncodeToString(hash[:8])
}

// enrichMetadata adds chunk-specific metadata
func (p *TextProcessor) enrichMetadata(original map[string]interface{}, chunkIndex, totalChunks int) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Copy original metadata
	for k, v := range original {
		metadata[k] = v
	}

	// Add chunk info
	metadata["chunk_index"] = chunkIndex
	metadata["total_chunks"] = totalChunks
	metadata["is_first_chunk"] = chunkIndex == 0
	metadata["is_last_chunk"] = chunkIndex == totalChunks-1

	return metadata
}

// PolicyDocumentProcessor handles policy documents
type PolicyDocumentProcessor struct {
	*TextProcessor
}

// NewPolicyDocumentProcessor creates a processor for policy documents
func NewPolicyDocumentProcessor(embedder EmbeddingProvider) *PolicyDocumentProcessor {
	return &PolicyDocumentProcessor{
		TextProcessor: NewTextProcessor(embedder, &TextProcessorConfig{
			ChunkSize: 800,
			Overlap:   100,
		}),
	}
}

// Process handles policy-specific processing
func (p *PolicyDocumentProcessor) Process(ctx context.Context, doc *Document) ([]*Document, error) {
	// Extract policy sections
	sections := p.extractPolicySections(doc.Content)

	var allDocs []*Document
	for sectionName, sectionContent := range sections {
		sectionDoc := &Document{
			ID:        fmt.Sprintf("%s-%s", doc.ID, sectionName),
			Content:   sectionContent,
			Metadata:  mergeMaps(doc.Metadata, map[string]interface{}{"section": sectionName}),
			StoreType: doc.StoreType,
		}

		chunks, err := p.TextProcessor.Process(ctx, sectionDoc)
		if err != nil {
			return nil, err
		}

		allDocs = append(allDocs, chunks...)
	}

	return allDocs, nil
}

// extractPolicySections extracts named sections from policy documents
func (p *PolicyDocumentProcessor) extractPolicySections(content string) map[string]string {
	sections := make(map[string]string)

	// Common policy section patterns
	sectionPatterns := []string{
		"coverage", "exclusions", "definitions", "conditions",
		"claims", "premiums", "limitations", "endorsements",
	}

	currentSection := "general"
	var currentContent strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)

		found := false
		for _, pattern := range sectionPatterns {
			if strings.Contains(lineLower, pattern) && len(line) < 100 {
				// Save previous section
				if currentContent.Len() > 0 {
					sections[currentSection] = currentContent.String()
				}
				currentSection = pattern
				currentContent.Reset()
				found = true
				break
			}
		}

		if !found {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last section
	if currentContent.Len() > 0 {
		sections[currentSection] = currentContent.String()
	}

	return sections
}

// FraudCaseProcessor handles fraud case documents
type FraudCaseProcessor struct {
	*TextProcessor
}

// NewFraudCaseProcessor creates a processor for fraud cases
func NewFraudCaseProcessor(embedder EmbeddingProvider) *FraudCaseProcessor {
	return &FraudCaseProcessor{
		TextProcessor: NewTextProcessor(embedder, &TextProcessorConfig{
			ChunkSize: 600,
			Overlap:   75,
		}),
	}
}

// Process handles fraud case-specific processing
func (p *FraudCaseProcessor) Process(ctx context.Context, doc *Document) ([]*Document, error) {
	// Add fraud-specific metadata extraction
	indicators := p.extractFraudIndicators(doc.Content)
	doc.Metadata["fraud_indicators"] = indicators
	doc.Metadata["fraud_type"] = p.classifyFraudType(doc.Content)

	return p.TextProcessor.Process(ctx, doc)
}

// extractFraudIndicators extracts fraud indicators from case text
func (p *FraudCaseProcessor) extractFraudIndicators(content string) []string {
	indicators := []string{}
	contentLower := strings.ToLower(content)

	patterns := map[string]string{
		"staged accident":     "staged_accident",
		"false claim":         "false_claim",
		"identity theft":      "identity_theft",
		"collusion":           "collusion",
		"forged document":     "document_forgery",
		"inflated":            "inflated_claim",
		"phantom":             "phantom_claim",
		"organized ring":      "fraud_ring",
		"repeat offender":     "repeat_offender",
		"suspicious timing":   "timing_anomaly",
	}

	for pattern, indicator := range patterns {
		if strings.Contains(contentLower, pattern) {
			indicators = append(indicators, indicator)
		}
	}

	return indicators
}

// classifyFraudType determines the fraud type from content
func (p *FraudCaseProcessor) classifyFraudType(content string) string {
	contentLower := strings.ToLower(content)

	types := map[string][]string{
		"health":   {"medical", "hospital", "treatment", "diagnosis", "prescription"},
		"auto":     {"vehicle", "car", "accident", "collision", "repair"},
		"property": {"home", "fire", "theft", "burglary", "damage"},
		"life":     {"death", "beneficiary", "mortality", "deceased"},
		"workers":  {"workplace", "injury", "disability", "compensation"},
	}

	for fraudType, keywords := range types {
		for _, keyword := range keywords {
			if strings.Contains(contentLower, keyword) {
				return fraudType
			}
		}
	}

	return "general"
}

// BatchProcessor processes documents in batches
type BatchProcessor struct {
	processor  DocumentProcessor
	batchSize  int
}

// NewBatchProcessor creates a batch processor
func NewBatchProcessor(processor DocumentProcessor, batchSize int) *BatchProcessor {
	return &BatchProcessor{
		processor: processor,
		batchSize: batchSize,
	}
}

// ProcessBatch processes a batch of documents
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, docs []*Document) ([]*Document, error) {
	var allProcessed []*Document

	for i := 0; i < len(docs); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := docs[i:end]
		for _, doc := range batch {
			processed, err := bp.processor.Process(ctx, doc)
			if err != nil {
				return nil, fmt.Errorf("failed to process document %s: %w", doc.ID, err)
			}
			allProcessed = append(allProcessed, processed...)
		}
	}

	return allProcessed, nil
}

// CreateDocumentFromText creates a document from raw text
func CreateDocumentFromText(content string, storeType models.StoreType, metadata map[string]interface{}) *Document {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	return &Document{
		ID:        uuid.New().String(),
		Content:   content,
		Metadata:  metadata,
		StoreType: storeType,
	}
}

// mergeMaps merges two maps
func mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}

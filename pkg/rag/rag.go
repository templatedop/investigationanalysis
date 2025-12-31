// Package rag provides the core RAG (Retrieval-Augmented Generation) system
package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/embeddings"
	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/vectorstore"
)

// System is the main RAG system that coordinates retrieval and generation
type System struct {
	stores     *vectorstore.PgVectorMultiStore
	embedder   *embeddings.Service
	llmClient  *llm.Client
	config     *Config
}

// Config configures the RAG system
type Config struct {
	DefaultTopK       int     `json:"default_top_k"`
	DefaultMinScore   float64 `json:"default_min_score"`
	MaxContextLength  int     `json:"max_context_length"`
	RetrievalTimeout  time.Duration `json:"retrieval_timeout"`
}

// DefaultConfig returns default RAG configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultTopK:      5,
		DefaultMinScore:  0.7,
		MaxContextLength: 8000,
		RetrievalTimeout: 30 * time.Second,
	}
}

// NewSystem creates a new RAG system
func NewSystem(stores *vectorstore.PgVectorMultiStore, embedder *embeddings.Service, llmClient *llm.Client, cfg *Config) *System {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &System{
		stores:    stores,
		embedder:  embedder,
		llmClient: llmClient,
		config:    cfg,
	}
}

// Query performs a RAG query and returns the generated response
func (s *System) Query(ctx context.Context, query string, opts *QueryOptions) (*QueryResult, error) {
	startTime := time.Now()

	if opts == nil {
		opts = DefaultQueryOptions()
	}

	// Generate query embedding
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Retrieve relevant documents
	searchOpts := &vectorstore.SearchOptions{
		Limit:    opts.TopK,
		MinScore: opts.MinScore,
	}

	results, err := s.stores.SearchAcrossStores(ctx, queryEmbedding, opts.StoreTypes, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Build context from retrieved documents
	context := s.buildContext(results, opts.MaxContextLength)

	// Generate response
	response, err := s.generate(ctx, query, context, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate: %w", err)
	}

	return &QueryResult{
		Query:          query,
		Response:       response,
		Sources:        s.extractSources(results),
		RetrievedDocs:  len(results),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// QueryOptions configures a RAG query
type QueryOptions struct {
	TopK             int                `json:"top_k"`
	MinScore         float64            `json:"min_score"`
	MaxContextLength int                `json:"max_context_length"`
	StoreTypes       []models.StoreType `json:"store_types"`
	SystemPrompt     string             `json:"system_prompt,omitempty"`
	Temperature      float64            `json:"temperature"`
	Model            string             `json:"model,omitempty"`
}

// DefaultQueryOptions returns default query options
func DefaultQueryOptions() *QueryOptions {
	return &QueryOptions{
		TopK:             5,
		MinScore:         0.7,
		MaxContextLength: 8000,
		StoreTypes: []models.StoreType{
			models.StorePolicyDocs,
			models.StoreFraudCases,
			models.StoreEntityKnowledge,
			models.StorePlaybooks,
		},
		Temperature: 0.1,
	}
}

// QueryResult contains the result of a RAG query
type QueryResult struct {
	Query          string                `json:"query"`
	Response       string                `json:"response"`
	Sources        []models.RAGSource    `json:"sources"`
	RetrievedDocs  int                   `json:"retrieved_docs"`
	ProcessingTime time.Duration         `json:"processing_time"`
}

// buildContext creates a context string from retrieved documents
func (s *System) buildContext(results []*vectorstore.SearchResult, maxLength int) string {
	var builder strings.Builder
	currentLength := 0

	for i, result := range results {
		docContext := fmt.Sprintf("\n\n[Document %d - %s (Score: %.2f)]\n%s",
			i+1, result.Source, result.Similarity, result.Document.Content)

		if currentLength+len(docContext) > maxLength {
			break
		}

		builder.WriteString(docContext)
		currentLength += len(docContext)
	}

	return builder.String()
}

// generate creates a response using the LLM
func (s *System) generate(ctx context.Context, query, context string, opts *QueryOptions) (string, error) {
	systemPrompt := opts.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	userPrompt := fmt.Sprintf(`Based on the following context, answer the question.

## Context
%s

## Question
%s

Provide a detailed, accurate response based solely on the provided context. If the context doesn't contain enough information to answer the question, state that clearly.`, context, query)

	req := &llm.ChatRequest{
		Model: opts.Model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: opts.Temperature,
	}

	resp, err := s.llmClient.ChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}

// extractSources converts search results to RAG sources
func (s *System) extractSources(results []*vectorstore.SearchResult) []models.RAGSource {
	sources := make([]models.RAGSource, len(results))
	for i, r := range results {
		sources[i] = models.RAGSource{
			StoreType:  r.Source,
			DocumentID: r.Document.ID,
			Chunk:      r.Document.Content,
			Similarity: r.Similarity,
			Metadata:   r.Document.Metadata,
		}
	}
	return sources
}

const defaultSystemPrompt = `You are an expert fraud investigation analyst. Your role is to analyze evidence, identify patterns, and provide accurate assessments based on the available information.

Guidelines:
- Be precise and factual in your analysis
- Cite specific evidence from the provided context
- Clearly distinguish between facts and inferences
- If information is insufficient, acknowledge the limitation
- Consider multiple perspectives and potential explanations
- Provide actionable recommendations when appropriate`

// SearchForAgent performs a search tailored to a specific agent type
func (s *System) SearchForAgent(ctx context.Context, agentType models.AgentType, query string) ([]*vectorstore.SearchResult, error) {
	// Map agents to relevant stores
	storeMap := map[models.AgentType][]models.StoreType{
		models.AgentTypePatternAnalysis:  {models.StoreFraudCases, models.StoreExternalKnowledge},
		models.AgentTypePolicyCompliance: {models.StorePolicyDocs, models.StorePlaybooks},
		models.AgentTypeEntityResolution: {models.StoreEntityKnowledge, models.StoreFraudCases},
		models.AgentTypeDocumentAnalysis: {models.StorePlaybooks, models.StoreFraudCases},
		models.AgentTypeNetworkAnalysis:  {models.StoreEntityKnowledge, models.StoreFraudCases},
		models.AgentTypeTemporalAnalysis: {models.StoreFraudCases, models.StoreExternalKnowledge},
		models.AgentTypeExplanation:      {models.StorePlaybooks, models.StorePolicyDocs},
		models.AgentTypeRiskAssessment:   {models.StoreFraudCases, models.StorePolicyDocs},
		models.AgentTypeDataEnrichment:   {models.StorePlaybooks, models.StoreEntityKnowledge},
		models.AgentTypeEvidence:         {models.StorePlaybooks, models.StoreFraudCases},
	}

	stores, ok := storeMap[agentType]
	if !ok {
		stores = []models.StoreType{models.StoreFraudCases, models.StorePolicyDocs}
	}

	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	return s.stores.SearchAcrossStores(ctx, queryEmbedding, stores, &vectorstore.SearchOptions{
		Limit:    5,
		MinScore: 0.65,
	})
}

// HybridQuery combines vector and keyword search
func (s *System) HybridQuery(ctx context.Context, query string, keywords string, opts *QueryOptions) (*QueryResult, error) {
	startTime := time.Now()

	if opts == nil {
		opts = DefaultQueryOptions()
	}

	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	var allResults []*vectorstore.SearchResult

	for _, storeType := range opts.StoreTypes {
		store, err := s.stores.GetStore(storeType)
		if err != nil {
			continue
		}

		results, err := store.HybridSearch(ctx, queryEmbedding, keywords, &vectorstore.SearchOptions{
			Limit:    opts.TopK,
			MinScore: opts.MinScore,
		})
		if err != nil {
			continue
		}

		allResults = append(allResults, results...)
	}

	// Re-rank and limit
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Similarity > allResults[j].Similarity
	})

	if len(allResults) > opts.TopK {
		allResults = allResults[:opts.TopK]
	}

	context := s.buildContext(allResults, opts.MaxContextLength)
	response, err := s.generate(ctx, query, context, opts)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		Query:          query,
		Response:       response,
		Sources:        s.extractSources(allResults),
		RetrievedDocs:  len(allResults),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// MultiStepQuery performs a multi-step RAG query with iterative refinement
func (s *System) MultiStepQuery(ctx context.Context, query string, steps int, opts *QueryOptions) (*QueryResult, error) {
	if opts == nil {
		opts = DefaultQueryOptions()
	}

	currentQuery := query
	var allSources []models.RAGSource
	var finalResponse string

	for i := 0; i < steps; i++ {
		result, err := s.Query(ctx, currentQuery, opts)
		if err != nil {
			return nil, fmt.Errorf("step %d failed: %w", i+1, err)
		}

		allSources = append(allSources, result.Sources...)
		finalResponse = result.Response

		// Generate follow-up query based on response
		if i < steps-1 {
			followUpQuery, err := s.generateFollowUp(ctx, query, result.Response)
			if err != nil {
				break // Continue with current results
			}
			currentQuery = followUpQuery
		}
	}

	return &QueryResult{
		Query:         query,
		Response:      finalResponse,
		Sources:       allSources,
		RetrievedDocs: len(allSources),
	}, nil
}

// generateFollowUp creates a follow-up query based on the current response
func (s *System) generateFollowUp(ctx context.Context, originalQuery, currentResponse string) (string, error) {
	prompt := fmt.Sprintf(`Based on the original question and the current answer, generate a follow-up question that would help gather more relevant information.

Original Question: %s

Current Answer: %s

Generate a concise follow-up question that addresses any gaps or seeks additional context. Return only the question.`,
		originalQuery, currentResponse)

	return s.llmClient.Complete(ctx, prompt)
}

// IndexDocument adds a document to the appropriate store
func (s *System) IndexDocument(ctx context.Context, doc *vectorstore.Document) error {
	// Generate embedding
	embedding, err := s.embedder.Embed(ctx, doc.Content)
	if err != nil {
		return fmt.Errorf("failed to embed document: %w", err)
	}
	doc.Embedding = embedding

	// Get the appropriate store
	store, err := s.stores.GetStore(doc.StoreType)
	if err != nil {
		return fmt.Errorf("failed to get store: %w", err)
	}

	return store.Insert(ctx, []*vectorstore.Document{doc})
}

// IndexBatch adds multiple documents to stores
func (s *System) IndexBatch(ctx context.Context, docs []*vectorstore.Document) error {
	// Group by store type
	docsByStore := make(map[models.StoreType][]*vectorstore.Document)
	for _, doc := range docs {
		docsByStore[doc.StoreType] = append(docsByStore[doc.StoreType], doc)
	}

	// Generate embeddings for all documents
	var texts []string
	for _, doc := range docs {
		texts = append(texts, doc.Content)
	}

	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	for i, doc := range docs {
		doc.Embedding = embeddings[i]
	}

	// Insert into respective stores
	for storeType, storeDocs := range docsByStore {
		store, err := s.stores.GetStore(storeType)
		if err != nil {
			continue
		}

		if err := store.Insert(ctx, storeDocs); err != nil {
			return fmt.Errorf("failed to insert into store %s: %w", storeType, err)
		}
	}

	return nil
}

// ContextBuilder helps build context for agents
type ContextBuilder struct {
	rag       *System
	maxLength int
}

// NewContextBuilder creates a context builder
func NewContextBuilder(rag *System, maxLength int) *ContextBuilder {
	return &ContextBuilder{
		rag:       rag,
		maxLength: maxLength,
	}
}

// BuildAgentContext builds context for a specific agent
func (cb *ContextBuilder) BuildAgentContext(ctx context.Context, agentType models.AgentType, queries []string) (string, []models.RAGSource, error) {
	var allResults []*vectorstore.SearchResult

	for _, query := range queries {
		results, err := cb.rag.SearchForAgent(ctx, agentType, query)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	// Deduplicate by document ID
	seen := make(map[string]bool)
	var unique []*vectorstore.SearchResult
	for _, r := range allResults {
		if !seen[r.Document.ID] {
			seen[r.Document.ID] = true
			unique = append(unique, r)
		}
	}

	// Sort by similarity
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Similarity > unique[j].Similarity
	})

	context := cb.rag.buildContext(unique, cb.maxLength)
	sources := cb.rag.extractSources(unique)

	return context, sources, nil
}

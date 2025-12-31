// Package embeddings provides text embedding generation services
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/vectorstore"
)

// Provider represents an embedding provider type
type Provider string

const (
	ProviderOllama   Provider = "ollama"
	ProviderOpenAI   Provider = "openai"
	ProviderVLLM     Provider = "vllm"
	ProviderLocal    Provider = "local"
	ProviderHuggingFace Provider = "huggingface"
)

// Config configures the embedding service
type Config struct {
	Provider    Provider `json:"provider"`
	BaseURL     string   `json:"base_url"`
	APIKey      string   `json:"api_key,omitempty"`
	Model       string   `json:"model"`
	Dimension   int      `json:"dimension"`
	BatchSize   int      `json:"batch_size"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int      `json:"max_retries"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Provider:   ProviderOllama,
		BaseURL:    "http://localhost:11434",
		Model:      "nomic-embed-text",
		Dimension:  768,
		BatchSize:  32,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// Service provides embedding generation functionality
type Service struct {
	config   *Config
	client   *http.Client
	provider EmbedderImpl
	cache    *EmbeddingCache
}

// EmbedderImpl is the interface for embedding implementations
type EmbedderImpl interface {
	Embed(ctx context.Context, text string) (vectorstore.Vector, error)
	EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error)
}

// NewService creates a new embedding service
func NewService(cfg *Config) (*Service, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	var provider EmbedderImpl
	var err error

	switch cfg.Provider {
	case ProviderOllama:
		provider = NewOllamaEmbedder(cfg, client)
	case ProviderOpenAI:
		provider = NewOpenAIEmbedder(cfg, client)
	case ProviderVLLM:
		provider = NewVLLMEmbedder(cfg, client)
	case ProviderHuggingFace:
		provider = NewHuggingFaceEmbedder(cfg, client)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, err
	}

	return &Service{
		config:   cfg,
		client:   client,
		provider: provider,
		cache:    NewEmbeddingCache(10000, 1*time.Hour),
	}, nil
}

// Embed generates embedding for a single text
func (s *Service) Embed(ctx context.Context, text string) (vectorstore.Vector, error) {
	// Check cache first
	if cached, ok := s.cache.Get(text); ok {
		return cached, nil
	}

	embedding, err := s.provider.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.Set(text, embedding)

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (s *Service) EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error) {
	results := make([]vectorstore.Vector, len(texts))
	uncached := make([]int, 0)
	uncachedTexts := make([]string, 0)

	// Check cache for each text
	for i, text := range texts {
		if cached, ok := s.cache.Get(text); ok {
			results[i] = cached
		} else {
			uncached = append(uncached, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}

	// Generate embeddings for uncached texts
	if len(uncachedTexts) > 0 {
		embeddings, err := s.provider.EmbedBatch(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		for i, idx := range uncached {
			results[idx] = embeddings[i]
			s.cache.Set(texts[idx], embeddings[i])
		}
	}

	return results, nil
}

// Dimension returns the embedding dimension
func (s *Service) Dimension() int {
	return s.config.Dimension
}

// OllamaEmbedder implements embeddings using Ollama
type OllamaEmbedder struct {
	config *Config
	client *http.Client
}

// NewOllamaEmbedder creates an Ollama embedder
func NewOllamaEmbedder(cfg *Config, client *http.Client) *OllamaEmbedder {
	return &OllamaEmbedder{
		config: cfg,
		client: client,
	}
}

// ollamaEmbedRequest is the request structure for Ollama embeddings
type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbedResponse is the response structure for Ollama embeddings
type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates a single embedding using Ollama
func (o *OllamaEmbedder) Embed(ctx context.Context, text string) (vectorstore.Vector, error) {
	reqBody := ollamaEmbedRequest{
		Model:  o.config.Model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/embeddings", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: %s", string(body))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts using Ollama
func (o *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error) {
	embeddings := make([]vectorstore.Vector, len(texts))

	// Ollama doesn't support batch, so we do parallel requests
	var wg sync.WaitGroup
	errChan := make(chan error, len(texts))
	sem := make(chan struct{}, o.config.BatchSize) // Limit concurrency

	for i, text := range texts {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			embedding, err := o.Embed(ctx, t)
			if err != nil {
				errChan <- err
				return
			}
			embeddings[idx] = embedding
		}(i, text)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	return embeddings, nil
}

// OpenAIEmbedder implements embeddings using OpenAI API
type OpenAIEmbedder struct {
	config *Config
	client *http.Client
}

// NewOpenAIEmbedder creates an OpenAI embedder
func NewOpenAIEmbedder(cfg *Config, client *http.Client) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		config: cfg,
		client: client,
	}
}

// openaiEmbedRequest is the request structure for OpenAI embeddings
type openaiEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// openaiEmbedResponse is the response structure for OpenAI embeddings
type openaiEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed generates a single embedding using OpenAI
func (o *OpenAIEmbedder) Embed(ctx context.Context, text string) (vectorstore.Vector, error) {
	embeddings, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts using OpenAI
func (o *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error) {
	reqBody := openaiEmbedRequest{
		Input: texts,
		Model: o.config.Model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/embeddings", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error: %s", string(body))
	}

	var result openaiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([]vectorstore.Vector, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// VLLMEmbedder implements embeddings using vLLM server
type VLLMEmbedder struct {
	config *Config
	client *http.Client
}

// NewVLLMEmbedder creates a vLLM embedder
func NewVLLMEmbedder(cfg *Config, client *http.Client) *VLLMEmbedder {
	return &VLLMEmbedder{
		config: cfg,
		client: client,
	}
}

// Embed generates a single embedding using vLLM
func (v *VLLMEmbedder) Embed(ctx context.Context, text string) (vectorstore.Vector, error) {
	embeddings, err := v.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// vllmEmbedRequest is the request structure for vLLM embeddings
type vllmEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// vllmEmbedResponse is the response structure for vLLM embeddings
type vllmEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// EmbedBatch generates embeddings for multiple texts using vLLM
func (v *VLLMEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error) {
	reqBody := vllmEmbedRequest{
		Input: texts,
		Model: v.config.Model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/embeddings", v.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if v.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+v.config.APIKey)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vllm error: %s", string(body))
	}

	var result vllmEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([]vectorstore.Vector, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// HuggingFaceEmbedder implements embeddings using HuggingFace Inference API
type HuggingFaceEmbedder struct {
	config *Config
	client *http.Client
}

// NewHuggingFaceEmbedder creates a HuggingFace embedder
func NewHuggingFaceEmbedder(cfg *Config, client *http.Client) *HuggingFaceEmbedder {
	return &HuggingFaceEmbedder{
		config: cfg,
		client: client,
	}
}

// Embed generates a single embedding using HuggingFace
func (h *HuggingFaceEmbedder) Embed(ctx context.Context, text string) (vectorstore.Vector, error) {
	embeddings, err := h.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts using HuggingFace
func (h *HuggingFaceEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]vectorstore.Vector, error) {
	reqBody := map[string]interface{}{
		"inputs": texts,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/models/%s", h.config.BaseURL, h.config.Model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.config.APIKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("huggingface error: %s", string(body))
	}

	var result [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([]vectorstore.Vector, len(result))
	for i, emb := range result {
		embeddings[i] = emb
	}

	return embeddings, nil
}

// EmbeddingCache provides caching for embeddings
type EmbeddingCache struct {
	cache   map[string]cacheEntry
	mutex   sync.RWMutex
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	embedding vectorstore.Vector
	timestamp time.Time
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(maxSize int, ttl time.Duration) *EmbeddingCache {
	cache := &EmbeddingCache{
		cache:   make(map[string]cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves an embedding from cache
func (c *EmbeddingCache) Get(text string) (vectorstore.Vector, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, ok := c.cache[text]
	if !ok {
		return nil, false
	}

	if time.Since(entry.timestamp) > c.ttl {
		return nil, false
	}

	return entry.embedding, true
}

// Set stores an embedding in cache
func (c *EmbeddingCache) Set(text string, embedding vectorstore.Vector) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Simple eviction if at capacity
	if len(c.cache) >= c.maxSize {
		// Remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.cache {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		delete(c.cache, oldestKey)
	}

	c.cache[text] = cacheEntry{
		embedding: embedding,
		timestamp: time.Now(),
	}
}

// cleanup periodically removes expired entries
func (c *EmbeddingCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for k, v := range c.cache {
			if now.Sub(v.timestamp) > c.ttl {
				delete(c.cache, k)
			}
		}
		c.mutex.Unlock()
	}
}

// ComputeCosineSimilarity calculates cosine similarity between two vectors
func ComputeCosineSimilarity(a, b vectorstore.Vector) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt calculates square root
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

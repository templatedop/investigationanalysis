// Package vectorstore provides vector database abstractions for RAG
package vectorstore

import (
	"context"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

// Vector represents an embedding vector
type Vector []float32

// Document represents a document stored in the vector store
type Document struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Embedding  Vector                 `json:"embedding,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	StoreType  models.StoreType       `json:"store_type"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Document   *Document          `json:"document"`
	Similarity float64            `json:"similarity"`
	Source     models.StoreType   `json:"source"`
}

// SearchOptions configures search behavior
type SearchOptions struct {
	Limit      int                    `json:"limit"`
	MinScore   float64                `json:"min_score"`
	Filters    map[string]interface{} `json:"filters"`
	StoreTypes []models.StoreType     `json:"store_types"`
}

// DefaultSearchOptions returns default search configuration
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		Limit:    10,
		MinScore: 0.7,
	}
}

// VectorStore defines the interface for vector database operations
type VectorStore interface {
	// Insert adds documents to the store
	Insert(ctx context.Context, docs []*Document) error

	// Update updates existing documents
	Update(ctx context.Context, docs []*Document) error

	// Delete removes documents by IDs
	Delete(ctx context.Context, ids []string) error

	// Get retrieves documents by IDs
	Get(ctx context.Context, ids []string) ([]*Document, error)

	// SimilaritySearch performs vector similarity search
	SimilaritySearch(ctx context.Context, query Vector, opts *SearchOptions) ([]*SearchResult, error)

	// HybridSearch combines vector and keyword search
	HybridSearch(ctx context.Context, query Vector, keywords string, opts *SearchOptions) ([]*SearchResult, error)

	// Close closes the connection
	Close() error
}

// MultiStoreManager manages multiple vector stores
type MultiStoreManager interface {
	// GetStore returns a store by type
	GetStore(storeType models.StoreType) (VectorStore, error)

	// SearchAcrossStores searches multiple stores
	SearchAcrossStores(ctx context.Context, query Vector, storeTypes []models.StoreType, opts *SearchOptions) ([]*SearchResult, error)

	// Close closes all stores
	Close() error
}

// EmbeddingProvider generates embeddings for text
type EmbeddingProvider interface {
	// Embed generates embedding for a single text
	Embed(ctx context.Context, text string) (Vector, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([]Vector, error)

	// Dimension returns the embedding dimension
	Dimension() int
}

// DocumentProcessor processes documents for indexing
type DocumentProcessor interface {
	// Process splits and prepares document for indexing
	Process(ctx context.Context, doc *Document) ([]*Document, error)

	// Chunk splits text into chunks
	Chunk(text string, chunkSize, overlap int) []string
}

// Package vectorstore provides pgvector implementation
package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

// PgVectorStore implements VectorStore using PostgreSQL with pgvector
type PgVectorStore struct {
	db        *sql.DB
	tableName string
	storeType models.StoreType
	dimension int
}

// PgVectorConfig configures the pgvector store
type PgVectorConfig struct {
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	SSLMode   string
	Dimension int
}

// NewPgVectorStore creates a new pgvector store instance
func NewPgVectorStore(db *sql.DB, storeType models.StoreType, dimension int) (*PgVectorStore, error) {
	tableName := fmt.Sprintf("vectors_%s", storeType)

	store := &PgVectorStore{
		db:        db,
		tableName: tableName,
		storeType: storeType,
		dimension: dimension,
	}

	if err := store.ensureTable(); err != nil {
		return nil, fmt.Errorf("failed to ensure table: %w", err)
	}

	return store, nil
}

// ensureTable creates the vector table if it doesn't exist
func (s *PgVectorStore) ensureTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			embedding vector(%d),
			metadata JSONB DEFAULT '{}',
			store_type TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, s.tableName, s.dimension)

	if _, err := s.db.Exec(query); err != nil {
		return err
	}

	// Create index for similarity search
	indexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx
		ON %s USING ivfflat (embedding vector_cosine_ops)
		WITH (lists = 100)
	`, s.tableName, s.tableName)

	if _, err := s.db.Exec(indexQuery); err != nil {
		// Index creation may fail if not enough rows, ignore
		_ = err
	}

	// Create GIN index for metadata
	metadataIndexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_metadata_idx
		ON %s USING GIN (metadata)
	`, s.tableName, s.tableName)

	_, _ = s.db.Exec(metadataIndexQuery)

	return nil
}

// Insert adds documents to the store
func (s *PgVectorStore) Insert(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, content, embedding, metadata, store_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, s.tableName))
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return err
		}

		vec := pgvector.NewVector(doc.Embedding)
		_, err = stmt.ExecContext(ctx,
			doc.ID,
			doc.Content,
			vec,
			metadataJSON,
			s.storeType,
			now,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Update updates existing documents
func (s *PgVectorStore) Update(ctx context.Context, docs []*Document) error {
	return s.Insert(ctx, docs) // Upsert handles updates
}

// Delete removes documents by IDs
func (s *PgVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1)`, s.tableName)
	_, err := s.db.ExecContext(ctx, query, pq.Array(ids))
	return err
}

// Get retrieves documents by IDs
func (s *PgVectorStore) Get(ctx context.Context, ids []string) ([]*Document, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := fmt.Sprintf(`
		SELECT id, content, embedding, metadata, store_type, created_at, updated_at
		FROM %s
		WHERE id = ANY($1)
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}

// SimilaritySearch performs vector similarity search
func (s *PgVectorStore) SimilaritySearch(ctx context.Context, query Vector, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	vec := pgvector.NewVector(query)

	sqlQuery := fmt.Sprintf(`
		SELECT id, content, embedding, metadata, store_type, created_at, updated_at,
			   1 - (embedding <=> $1) as similarity
		FROM %s
		WHERE 1 - (embedding <=> $1) >= $2
		ORDER BY embedding <=> $1
		LIMIT $3
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, sqlQuery, vec, opts.MinScore, opts.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanResults(rows)
}

// HybridSearch combines vector and keyword search
func (s *PgVectorStore) HybridSearch(ctx context.Context, query Vector, keywords string, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	vec := pgvector.NewVector(query)

	// Combine vector similarity with full-text search
	sqlQuery := fmt.Sprintf(`
		WITH vector_search AS (
			SELECT id, content, embedding, metadata, store_type, created_at, updated_at,
				   1 - (embedding <=> $1) as vector_score
			FROM %s
			WHERE 1 - (embedding <=> $1) >= $2
		),
		text_search AS (
			SELECT id, ts_rank(to_tsvector('english', content), plainto_tsquery('english', $3)) as text_score
			FROM %s
			WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $3)
		)
		SELECT v.id, v.content, v.embedding, v.metadata, v.store_type, v.created_at, v.updated_at,
			   COALESCE(v.vector_score * 0.7 + COALESCE(t.text_score, 0) * 0.3, v.vector_score) as similarity
		FROM vector_search v
		LEFT JOIN text_search t ON v.id = t.id
		ORDER BY similarity DESC
		LIMIT $4
	`, s.tableName, s.tableName)

	rows, err := s.db.QueryContext(ctx, sqlQuery, vec, opts.MinScore, keywords, opts.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanResults(rows)
}

// Close closes the connection
func (s *PgVectorStore) Close() error {
	return nil // DB is managed externally
}

// scanDocuments scans rows into documents
func (s *PgVectorStore) scanDocuments(rows *sql.Rows) ([]*Document, error) {
	var docs []*Document

	for rows.Next() {
		var doc Document
		var vec pgvector.Vector
		var metadataJSON []byte
		var storeType string

		err := rows.Scan(
			&doc.ID,
			&doc.Content,
			&vec,
			&metadataJSON,
			&storeType,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		doc.Embedding = vec.Slice()
		doc.StoreType = models.StoreType(storeType)

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]interface{})
		}

		docs = append(docs, &doc)
	}

	return docs, rows.Err()
}

// scanResults scans rows into search results
func (s *PgVectorStore) scanResults(rows *sql.Rows) ([]*SearchResult, error) {
	var results []*SearchResult

	for rows.Next() {
		var doc Document
		var vec pgvector.Vector
		var metadataJSON []byte
		var storeType string
		var similarity float64

		err := rows.Scan(
			&doc.ID,
			&doc.Content,
			&vec,
			&metadataJSON,
			&storeType,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&similarity,
		)
		if err != nil {
			return nil, err
		}

		doc.Embedding = vec.Slice()
		doc.StoreType = models.StoreType(storeType)

		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]interface{})
		}

		results = append(results, &SearchResult{
			Document:   &doc,
			Similarity: similarity,
			Source:     doc.StoreType,
		})
	}

	return results, rows.Err()
}

// PgVectorMultiStore manages multiple pgvector stores
type PgVectorMultiStore struct {
	db        *sql.DB
	stores    map[models.StoreType]*PgVectorStore
	dimension int
}

// NewPgVectorMultiStore creates a multi-store manager
func NewPgVectorMultiStore(db *sql.DB, dimension int, storeTypes []models.StoreType) (*PgVectorMultiStore, error) {
	ms := &PgVectorMultiStore{
		db:        db,
		stores:    make(map[models.StoreType]*PgVectorStore),
		dimension: dimension,
	}

	for _, st := range storeTypes {
		store, err := NewPgVectorStore(db, st, dimension)
		if err != nil {
			return nil, fmt.Errorf("failed to create store %s: %w", st, err)
		}
		ms.stores[st] = store
	}

	return ms, nil
}

// GetStore returns a store by type
func (ms *PgVectorMultiStore) GetStore(storeType models.StoreType) (VectorStore, error) {
	store, ok := ms.stores[storeType]
	if !ok {
		return nil, fmt.Errorf("store not found: %s", storeType)
	}
	return store, nil
}

// SearchAcrossStores searches multiple stores
func (ms *PgVectorMultiStore) SearchAcrossStores(ctx context.Context, query Vector, storeTypes []models.StoreType, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	var allResults []*SearchResult

	for _, st := range storeTypes {
		store, ok := ms.stores[st]
		if !ok {
			continue
		}

		results, err := store.SimilaritySearch(ctx, query, opts)
		if err != nil {
			continue
		}

		allResults = append(allResults, results...)
	}

	// Sort by similarity and limit
	sortResultsBySimilarity(allResults)

	if len(allResults) > opts.Limit {
		allResults = allResults[:opts.Limit]
	}

	return allResults, nil
}

// Close closes all stores
func (ms *PgVectorMultiStore) Close() error {
	return ms.db.Close()
}

// sortResultsBySimilarity sorts results by similarity score descending
func sortResultsBySimilarity(results []*SearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// ConnectPgVector creates a connection to PostgreSQL with pgvector
func ConnectPgVector(cfg *PgVectorConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable pgvector extension
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return nil, fmt.Errorf("failed to create vector extension: %w", err)
	}

	return db, nil
}

// BuildFilterQuery constructs SQL filter from metadata filters
func BuildFilterQuery(filters map[string]interface{}) (string, []interface{}) {
	if len(filters) == 0 {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIndex := 1

	for key, value := range filters {
		conditions = append(conditions, fmt.Sprintf("metadata->>'%s' = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}

	return " AND " + strings.Join(conditions, " AND "), args
}

// Package workflows provides Temporal worker configuration
package workflows

import (
	"context"
	"database/sql"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/fraudinvestigation/rag-framework/pkg/agents"
	"github.com/fraudinvestigation/rag-framework/pkg/embeddings"
	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
	"github.com/fraudinvestigation/rag-framework/pkg/vectorstore"
)

// WorkerConfig configures the Temporal worker
type WorkerConfig struct {
	TemporalHost      string
	TemporalNamespace string
	TaskQueue         string
	WorkerCount       int

	// Database configuration
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// LLM configuration
	LLMProvider string
	LLMBaseURL  string
	LLMModel    string

	// Embedding configuration
	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingModel    string
	EmbeddingDimension int
}

// DefaultWorkerConfig returns default configuration
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		TemporalHost:       "localhost:7233",
		TemporalNamespace:  "default",
		TaskQueue:          TaskQueueName,
		WorkerCount:        4,
		DBHost:             "localhost",
		DBPort:             5432,
		DBUser:             "postgres",
		DBPassword:         "postgres",
		DBName:             "fraud_investigation",
		LLMProvider:        "ollama",
		LLMBaseURL:         "http://localhost:11434",
		LLMModel:           "deepseek-r1:32b",
		EmbeddingProvider:  "ollama",
		EmbeddingBaseURL:   "http://localhost:11434",
		EmbeddingModel:     "nomic-embed-text",
		EmbeddingDimension: 768,
	}
}

// Worker manages the Temporal worker and its dependencies
type Worker struct {
	config       *WorkerConfig
	temporalClient client.Client
	worker       worker.Worker
	db           *sql.DB
	llmClient    *llm.Client
	embedder     *embeddings.Service
	ragSystem    *rag.System
	activities   *Activities
}

// NewWorker creates a new worker instance
func NewWorker(cfg *WorkerConfig) (*Worker, error) {
	if cfg == nil {
		cfg = DefaultWorkerConfig()
	}

	w := &Worker{
		config: cfg,
	}

	if err := w.initialize(); err != nil {
		return nil, err
	}

	return w, nil
}

// initialize sets up all dependencies
func (w *Worker) initialize() error {
	var err error

	// Connect to Temporal
	w.temporalClient, err = client.Dial(client.Options{
		HostPort:  w.config.TemporalHost,
		Namespace: w.config.TemporalNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Temporal: %w", err)
	}

	// Connect to database
	w.db, err = vectorstore.ConnectPgVector(&vectorstore.PgVectorConfig{
		Host:      w.config.DBHost,
		Port:      w.config.DBPort,
		User:      w.config.DBUser,
		Password:  w.config.DBPassword,
		Database:  w.config.DBName,
		SSLMode:   "disable",
		Dimension: w.config.EmbeddingDimension,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize LLM client
	w.llmClient, err = llm.NewClient(&llm.Config{
		Provider:     llm.Provider(w.config.LLMProvider),
		BaseURL:      w.config.LLMBaseURL,
		DefaultModel: w.config.LLMModel,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Initialize embeddings service
	w.embedder, err = embeddings.NewService(&embeddings.Config{
		Provider:  embeddings.Provider(w.config.EmbeddingProvider),
		BaseURL:   w.config.EmbeddingBaseURL,
		Model:     w.config.EmbeddingModel,
		Dimension: w.config.EmbeddingDimension,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize embeddings service: %w", err)
	}

	// Initialize vector stores
	storeTypes := []models.StoreType{
		models.StorePolicyDocs,
		models.StoreFraudCases,
		models.StoreEntityKnowledge,
		models.StorePlaybooks,
		models.StoreExternalKnowledge,
	}

	multiStore, err := vectorstore.NewPgVectorMultiStore(w.db, w.config.EmbeddingDimension, storeTypes)
	if err != nil {
		return fmt.Errorf("failed to initialize vector stores: %w", err)
	}

	// Initialize RAG system
	w.ragSystem = rag.NewSystem(multiStore, w.embedder, w.llmClient, nil)

	// Initialize agents
	w.initializeAgents()

	// Create Temporal worker
	w.worker = worker.New(w.temporalClient, w.config.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: w.config.WorkerCount,
	})

	// Register workflows and activities
	w.registerWorkflows()
	w.registerActivities()

	return nil
}

// initializeAgents creates all agent instances
func (w *Worker) initializeAgents() {
	dataEnrichment := agents.NewDataEnrichmentAgent(w.llmClient, w.ragSystem, w.db)
	entityResolution := agents.NewEntityResolutionAgent(w.llmClient, w.ragSystem)
	patternAnalysis := agents.NewPatternAnalysisAgent(w.llmClient, w.ragSystem)
	networkAnalysis := agents.NewNetworkAnalysisAgent(w.llmClient, w.ragSystem, nil)
	temporalAnalysis := agents.NewTemporalAnalysisAgent(w.llmClient, w.ragSystem)
	documentAnalysis := agents.NewDocumentAnalysisAgent(w.llmClient, w.ragSystem)
	policyCompliance := agents.NewPolicyComplianceAgent(w.llmClient, w.ragSystem)
	riskAssessment := agents.NewRiskAssessmentAgent(w.llmClient, w.ragSystem)
	evidence := agents.NewEvidenceCompilationAgent(w.llmClient, w.ragSystem)
	explanation := agents.NewExplanationAgent(w.llmClient, w.ragSystem)

	w.activities = NewActivities(
		dataEnrichment,
		entityResolution,
		patternAnalysis,
		networkAnalysis,
		temporalAnalysis,
		documentAnalysis,
		policyCompliance,
		riskAssessment,
		evidence,
		explanation,
	)
}

// registerWorkflows registers all workflows
func (w *Worker) registerWorkflows() {
	w.worker.RegisterWorkflow(FraudInvestigationWorkflow)
	w.worker.RegisterWorkflow(QuickInvestigationWorkflow)
	w.worker.RegisterWorkflow(BatchInvestigationWorkflow)
}

// registerActivities registers all activities
func (w *Worker) registerActivities() {
	w.worker.RegisterActivity(w.activities.DataEnrichmentActivity)
	w.worker.RegisterActivity(w.activities.EntityResolutionActivity)
	w.worker.RegisterActivity(w.activities.PatternAnalysisActivity)
	w.worker.RegisterActivity(w.activities.NetworkAnalysisActivity)
	w.worker.RegisterActivity(w.activities.TemporalAnalysisActivity)
	w.worker.RegisterActivity(w.activities.DocumentAnalysisActivity)
	w.worker.RegisterActivity(w.activities.PolicyComplianceActivity)
	w.worker.RegisterActivity(w.activities.RiskAssessmentActivity)
	w.worker.RegisterActivity(w.activities.EvidenceCompilationActivity)
	w.worker.RegisterActivity(w.activities.ExplanationActivity)
	w.worker.RegisterActivity(w.activities.ExecuteFraudActionActivity)
}

// Start starts the worker
func (w *Worker) Start() error {
	return w.worker.Start()
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.worker.Stop()
	if w.temporalClient != nil {
		w.temporalClient.Close()
	}
	if w.db != nil {
		w.db.Close()
	}
}

// Run starts the worker and blocks until interrupted
func (w *Worker) Run(ctx context.Context) error {
	if err := w.Start(); err != nil {
		return err
	}

	<-ctx.Done()
	w.Stop()
	return nil
}

// GetClient returns the Temporal client for starting workflows
func (w *Worker) GetClient() client.Client {
	return w.temporalClient
}

// GetRAGSystem returns the RAG system for knowledge management
func (w *Worker) GetRAGSystem() *rag.System {
	return w.ragSystem
}

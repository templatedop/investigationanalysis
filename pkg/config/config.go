// Package config provides configuration management for the RAG framework
package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds all configuration for the fraud investigation system
type Config struct {
	Server       ServerConfig       `json:"server"`
	Temporal     TemporalConfig     `json:"temporal"`
	Database     DatabaseConfig     `json:"database"`
	LLM          LLMConfig          `json:"llm"`
	Embedding    EmbeddingConfig    `json:"embedding"`
	RAG          RAGConfig          `json:"rag"`
	Agents       AgentsConfig       `json:"agents"`
	Graph        GraphConfig        `json:"graph"`
	Observability ObservabilityConfig `json:"observability"`
}

// ServerConfig configures the HTTP server
type ServerConfig struct {
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	EnableCORS   bool          `json:"enable_cors"`
}

// TemporalConfig configures Temporal connection
type TemporalConfig struct {
	Host      string `json:"host"`
	Namespace string `json:"namespace"`
	TaskQueue string `json:"task_queue"`
}

// DatabaseConfig configures database connection
type DatabaseConfig struct {
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	User            string        `json:"user"`
	Password        string        `json:"password"`
	Database        string        `json:"database"`
	SSLMode         string        `json:"ssl_mode"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// LLMConfig configures LLM providers
type LLMConfig struct {
	Provider     string        `json:"provider"`
	BaseURL      string        `json:"base_url"`
	APIKey       string        `json:"api_key,omitempty"`
	DefaultModel string        `json:"default_model"`
	Timeout      time.Duration `json:"timeout"`
	Models       ModelsConfig  `json:"models"`
}

// ModelsConfig maps model types to specific models
type ModelsConfig struct {
	Fast      string `json:"fast"`
	Medium    string `json:"medium"`
	Reasoning string `json:"reasoning"`
	Vision    string `json:"vision"`
}

// EmbeddingConfig configures embedding generation
type EmbeddingConfig struct {
	Provider  string        `json:"provider"`
	BaseURL   string        `json:"base_url"`
	APIKey    string        `json:"api_key,omitempty"`
	Model     string        `json:"model"`
	Dimension int           `json:"dimension"`
	BatchSize int           `json:"batch_size"`
	Timeout   time.Duration `json:"timeout"`
}

// RAGConfig configures the RAG system
type RAGConfig struct {
	DefaultTopK      int           `json:"default_top_k"`
	DefaultMinScore  float64       `json:"default_min_score"`
	MaxContextLength int           `json:"max_context_length"`
	ChunkSize        int           `json:"chunk_size"`
	ChunkOverlap     int           `json:"chunk_overlap"`
	RetrievalTimeout time.Duration `json:"retrieval_timeout"`
}

// AgentsConfig configures agent behavior
type AgentsConfig struct {
	EnabledAgents      []string          `json:"enabled_agents"`
	RiskThresholds     RiskThresholds    `json:"risk_thresholds"`
	EscalationRules    EscalationRules   `json:"escalation_rules"`
	HumanReviewTimeout time.Duration     `json:"human_review_timeout"`
}

// RiskThresholds defines risk level boundaries
type RiskThresholds struct {
	LowMax    float64 `json:"low_max"`
	MediumMax float64 `json:"medium_max"`
	HighMax   float64 `json:"high_max"`
}

// EscalationRules defines escalation behavior
type EscalationRules struct {
	AutoEscalateThreshold float64 `json:"auto_escalate_threshold"`
	HumanReviewThreshold  float64 `json:"human_review_threshold"`
	MaxAutoDecisionAmount float64 `json:"max_auto_decision_amount"`
	RequiredConfidence    float64 `json:"required_confidence"`
}

// GraphConfig configures the LangGraph-style execution
type GraphConfig struct {
	Enabled           bool          `json:"enabled"`
	MaxIterations     int           `json:"max_iterations"`
	EnableCheckpoints bool          `json:"enable_checkpoints"`
	CheckpointTTL     time.Duration `json:"checkpoint_ttl"`
	MaxRecursionDepth int           `json:"max_recursion_depth"`
	DetectLoops       bool          `json:"detect_loops"`
	LoopThreshold     int           `json:"loop_threshold"`
	ParallelExecution bool          `json:"parallel_execution"`
	StreamUpdates     bool          `json:"stream_updates"`
}

// ObservabilityConfig configures logging, metrics, and health checks
type ObservabilityConfig struct {
	Logging LoggingConfig `json:"logging"`
	Metrics MetricsConfig `json:"metrics"`
	Health  HealthConfig  `json:"health"`
	Tracing TracingConfig `json:"tracing"`
}

// LoggingConfig configures structured logging
type LoggingConfig struct {
	Enabled     bool   `json:"enabled"`
	Level       string `json:"level"` // debug, info, warn, error, fatal
	Format      string `json:"format"` // json, text
	Output      string `json:"output"` // stdout, stderr, file path
	EnableColor bool   `json:"enable_color"`
	AddCaller   bool   `json:"add_caller"`
	AddTime     bool   `json:"add_time"`
}

// MetricsConfig configures Prometheus metrics
type MetricsConfig struct {
	Enabled            bool   `json:"enabled"`
	Endpoint           string `json:"endpoint"`
	Namespace          string `json:"namespace"`
	Subsystem          string `json:"subsystem"`
	EnableGoMetrics    bool   `json:"enable_go_metrics"`
	EnableProcessMetrics bool `json:"enable_process_metrics"`
}

// HealthConfig configures health checks
type HealthConfig struct {
	Enabled          bool          `json:"enabled"`
	LivenessPath     string        `json:"liveness_path"`
	ReadinessPath    string        `json:"readiness_path"`
	CheckTimeout     time.Duration `json:"check_timeout"`
	CheckInterval    time.Duration `json:"check_interval"`
	FailureThreshold int           `json:"failure_threshold"`
}

// TracingConfig configures distributed tracing
type TracingConfig struct {
	Enabled     bool    `json:"enabled"`
	Provider    string  `json:"provider"` // jaeger, zipkin, otlp
	Endpoint    string  `json:"endpoint"`
	ServiceName string  `json:"service_name"`
	SampleRate  float64 `json:"sample_rate"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			EnableCORS:   true,
		},
		Temporal: TemporalConfig{
			Host:      "localhost:7233",
			Namespace: "default",
			TaskQueue: "fraud-investigation",
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			Database:        "fraud_investigation",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		LLM: LLMConfig{
			Provider:     "ollama",
			BaseURL:      "http://localhost:11434",
			DefaultModel: "deepseek-r1:32b",
			Timeout:      120 * time.Second,
			Models: ModelsConfig{
				Fast:      "llama3.2:7b",
				Medium:    "llama3.2:14b",
				Reasoning: "deepseek-r1:32b",
				Vision:    "llava:13b",
			},
		},
		Embedding: EmbeddingConfig{
			Provider:  "ollama",
			BaseURL:   "http://localhost:11434",
			Model:     "nomic-embed-text",
			Dimension: 768,
			BatchSize: 32,
			Timeout:   30 * time.Second,
		},
		RAG: RAGConfig{
			DefaultTopK:      5,
			DefaultMinScore:  0.7,
			MaxContextLength: 8000,
			ChunkSize:        512,
			ChunkOverlap:     50,
			RetrievalTimeout: 30 * time.Second,
		},
		Agents: AgentsConfig{
			EnabledAgents: []string{
				"data_enrichment",
				"entity_resolution",
				"pattern_analysis",
				"network_analysis",
				"temporal_analysis",
				"document_analysis",
				"policy_compliance",
				"risk_assessment",
				"evidence_compilation",
				"explanation",
			},
			RiskThresholds: RiskThresholds{
				LowMax:    0.3,
				MediumMax: 0.6,
				HighMax:   0.8,
			},
			EscalationRules: EscalationRules{
				AutoEscalateThreshold: 0.9,
				HumanReviewThreshold:  0.7,
				MaxAutoDecisionAmount: 100000,
				RequiredConfidence:    0.85,
			},
			HumanReviewTimeout: 24 * time.Hour,
		},
		Graph: GraphConfig{
			Enabled:           false, // Disabled by default, use traditional Temporal workflows
			MaxIterations:     100,
			EnableCheckpoints: true,
			CheckpointTTL:     24 * time.Hour,
			MaxRecursionDepth: 10,
			DetectLoops:       true,
			LoopThreshold:     3,
			ParallelExecution: true,
			StreamUpdates:     true,
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Enabled:     true,
				Level:       "info",
				Format:      "json",
				Output:      "stdout",
				EnableColor: true,
				AddCaller:   true,
				AddTime:     true,
			},
			Metrics: MetricsConfig{
				Enabled:              true,
				Endpoint:             "/metrics",
				Namespace:            "fraud_investigation",
				Subsystem:            "",
				EnableGoMetrics:      true,
				EnableProcessMetrics: true,
			},
			Health: HealthConfig{
				Enabled:          true,
				LivenessPath:     "/health/live",
				ReadinessPath:    "/health/ready",
				CheckTimeout:     5 * time.Second,
				CheckInterval:    10 * time.Second,
				FailureThreshold: 3,
			},
			Tracing: TracingConfig{
				Enabled:     false,
				Provider:    "otlp",
				Endpoint:    "localhost:4317",
				ServiceName: "fraud-investigation",
				SampleRate:  0.1,
			},
		},
	}
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override from environment
	cfg.overrideFromEnv()

	return cfg, nil
}

// LoadConfigOrDefault loads config from file or returns default
func LoadConfigOrDefault(path string) *Config {
	cfg, err := LoadConfig(path)
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// overrideFromEnv overrides config from environment variables
func (c *Config) overrideFromEnv() {
	if v := os.Getenv("PORT"); v != "" {
		c.Server.Port = parseInt(v, c.Server.Port)
	}
	if v := os.Getenv("TEMPORAL_HOST"); v != "" {
		c.Temporal.Host = v
	}
	if v := os.Getenv("TEMPORAL_NAMESPACE"); v != "" {
		c.Temporal.Namespace = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		c.Database.Port = parseInt(v, c.Database.Port)
	}
	if v := os.Getenv("DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		c.Database.Database = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		c.LLM.Provider = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		c.LLM.BaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLM.DefaultModel = v
	}
	if v := os.Getenv("EMBEDDING_PROVIDER"); v != "" {
		c.Embedding.Provider = v
	}
	if v := os.Getenv("EMBEDDING_BASE_URL"); v != "" {
		c.Embedding.BaseURL = v
	}
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		c.Embedding.Model = v
	}

	// Graph configuration
	if v := os.Getenv("GRAPH_ENABLED"); v != "" {
		c.Graph.Enabled = parseBool(v, c.Graph.Enabled)
	}

	// Observability configuration
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.Observability.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		c.Observability.Logging.Format = v
	}
	if v := os.Getenv("METRICS_ENABLED"); v != "" {
		c.Observability.Metrics.Enabled = parseBool(v, c.Observability.Metrics.Enabled)
	}
	if v := os.Getenv("HEALTH_ENABLED"); v != "" {
		c.Observability.Health.Enabled = parseBool(v, c.Observability.Health.Enabled)
	}
	if v := os.Getenv("TRACING_ENABLED"); v != "" {
		c.Observability.Tracing.Enabled = parseBool(v, c.Observability.Tracing.Enabled)
	}
	if v := os.Getenv("TRACING_ENDPOINT"); v != "" {
		c.Observability.Tracing.Endpoint = v
	}
}

func parseInt(s string, defaultVal int) int {
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultVal
	}
	return v
}

func parseBool(s string, defaultVal bool) bool {
	switch s {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return defaultVal
	}
}

var fmt = struct {
	Sscanf func(string, string, ...interface{}) (int, error)
}{
	Sscanf: func(s, format string, a ...interface{}) (int, error) {
		// Simple integer parsing
		val := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				val = val*10 + int(c-'0')
			}
		}
		if len(a) > 0 {
			if p, ok := a[0].(*int); ok {
				*p = val
			}
		}
		return 1, nil
	},
}

// IsGraphEnabled returns true if LangGraph mode is enabled
func (c *Config) IsGraphEnabled() bool {
	return c.Graph.Enabled
}

// GetLogLevel returns the configured log level
func (c *Config) GetLogLevel() string {
	return c.Observability.Logging.Level
}

// IsMetricsEnabled returns true if metrics are enabled
func (c *Config) IsMetricsEnabled() bool {
	return c.Observability.Metrics.Enabled
}

// IsHealthEnabled returns true if health checks are enabled
func (c *Config) IsHealthEnabled() bool {
	return c.Observability.Health.Enabled
}

// IsTracingEnabled returns true if tracing is enabled
func (c *Config) IsTracingEnabled() bool {
	return c.Observability.Tracing.Enabled
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Add validation logic here
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return &ValidationError{Field: "server.port", Message: "port must be between 1 and 65535"}
	}
	if c.Database.Host == "" {
		return &ValidationError{Field: "database.host", Message: "database host is required"}
	}
	if c.LLM.BaseURL == "" {
		return &ValidationError{Field: "llm.base_url", Message: "LLM base URL is required"}
	}
	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "config validation failed for " + e.Field + ": " + e.Message
}

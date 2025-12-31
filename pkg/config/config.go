// Package config provides configuration management for the RAG framework
package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds all configuration for the fraud investigation system
type Config struct {
	Server    ServerConfig    `json:"server"`
	Temporal  TemporalConfig  `json:"temporal"`
	Database  DatabaseConfig  `json:"database"`
	LLM       LLMConfig       `json:"llm"`
	Embedding EmbeddingConfig `json:"embedding"`
	RAG       RAGConfig       `json:"rag"`
	Agents    AgentsConfig    `json:"agents"`
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
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
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
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "postgres",
			Database: "fraud_investigation",
			SSLMode:  "disable",
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
}

func parseInt(s string, defaultVal int) int {
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultVal
	}
	return v
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

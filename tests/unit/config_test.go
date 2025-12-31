// Package unit contains unit tests for the configuration package
package unit

import (
	"os"
	"testing"
	"time"

	"github.com/investigationanalysis/pkg/config"
)

// TestConfig tests the configuration package
func TestConfig(t *testing.T) {
	t.Run("DefaultConfig returns valid configuration", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if cfg == nil {
			t.Fatal("DefaultConfig returned nil")
		}

		// Check server defaults
		if cfg.Server.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", cfg.Server.Port)
		}
		if cfg.Server.ReadTimeout != 30*time.Second {
			t.Errorf("Expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
		}

		// Check database defaults
		if cfg.Database.Host != "localhost" {
			t.Errorf("Expected database host 'localhost', got '%s'", cfg.Database.Host)
		}
		if cfg.Database.Port != 5432 {
			t.Errorf("Expected database port 5432, got %d", cfg.Database.Port)
		}

		// Check LLM defaults
		if cfg.LLM.Provider != "ollama" {
			t.Errorf("Expected LLM provider 'ollama', got '%s'", cfg.LLM.Provider)
		}

		// Check graph defaults
		if cfg.Graph.Enabled {
			t.Error("Graph should be disabled by default")
		}
		if cfg.Graph.MaxIterations != 100 {
			t.Errorf("Expected max iterations 100, got %d", cfg.Graph.MaxIterations)
		}

		// Check observability defaults
		if !cfg.Observability.Logging.Enabled {
			t.Error("Logging should be enabled by default")
		}
		if cfg.Observability.Logging.Level != "info" {
			t.Errorf("Expected log level 'info', got '%s'", cfg.Observability.Logging.Level)
		}
		if !cfg.Observability.Metrics.Enabled {
			t.Error("Metrics should be enabled by default")
		}
		if !cfg.Observability.Health.Enabled {
			t.Error("Health checks should be enabled by default")
		}
	})

	t.Run("Config has all agent types", func(t *testing.T) {
		cfg := config.DefaultConfig()
		expectedAgents := []string{
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
		}

		if len(cfg.Agents.EnabledAgents) != len(expectedAgents) {
			t.Errorf("Expected %d agents, got %d", len(expectedAgents), len(cfg.Agents.EnabledAgents))
		}

		agentMap := make(map[string]bool)
		for _, a := range cfg.Agents.EnabledAgents {
			agentMap[a] = true
		}

		for _, expected := range expectedAgents {
			if !agentMap[expected] {
				t.Errorf("Missing expected agent: %s", expected)
			}
		}
	})

	t.Run("IsGraphEnabled returns correct value", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if cfg.IsGraphEnabled() {
			t.Error("Graph should be disabled by default")
		}

		cfg.Graph.Enabled = true
		if !cfg.IsGraphEnabled() {
			t.Error("Graph should be enabled after setting")
		}
	})

	t.Run("GetLogLevel returns correct value", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if cfg.GetLogLevel() != "info" {
			t.Errorf("Expected log level 'info', got '%s'", cfg.GetLogLevel())
		}

		cfg.Observability.Logging.Level = "debug"
		if cfg.GetLogLevel() != "debug" {
			t.Errorf("Expected log level 'debug', got '%s'", cfg.GetLogLevel())
		}
	})

	t.Run("IsMetricsEnabled returns correct value", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if !cfg.IsMetricsEnabled() {
			t.Error("Metrics should be enabled by default")
		}

		cfg.Observability.Metrics.Enabled = false
		if cfg.IsMetricsEnabled() {
			t.Error("Metrics should be disabled after setting")
		}
	})

	t.Run("IsHealthEnabled returns correct value", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if !cfg.IsHealthEnabled() {
			t.Error("Health should be enabled by default")
		}

		cfg.Observability.Health.Enabled = false
		if cfg.IsHealthEnabled() {
			t.Error("Health should be disabled after setting")
		}
	})

	t.Run("IsTracingEnabled returns correct value", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if cfg.IsTracingEnabled() {
			t.Error("Tracing should be disabled by default")
		}

		cfg.Observability.Tracing.Enabled = true
		if !cfg.IsTracingEnabled() {
			t.Error("Tracing should be enabled after setting")
		}
	})

	t.Run("Validate catches invalid port", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Server.Port = 0

		err := cfg.Validate()
		if err == nil {
			t.Error("Should fail validation with invalid port")
		}

		cfg.Server.Port = 70000
		err = cfg.Validate()
		if err == nil {
			t.Error("Should fail validation with port > 65535")
		}
	})

	t.Run("Validate catches missing database host", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Database.Host = ""

		err := cfg.Validate()
		if err == nil {
			t.Error("Should fail validation with empty database host")
		}
	})

	t.Run("Validate catches missing LLM base URL", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.LLM.BaseURL = ""

		err := cfg.Validate()
		if err == nil {
			t.Error("Should fail validation with empty LLM base URL")
		}
	})

	t.Run("ValidationError formats correctly", func(t *testing.T) {
		err := &config.ValidationError{
			Field:   "server.port",
			Message: "port must be between 1 and 65535",
		}

		expected := "config validation failed for server.port: port must be between 1 and 65535"
		if err.Error() != expected {
			t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("LoadConfigOrDefault returns default on missing file", func(t *testing.T) {
		cfg := config.LoadConfigOrDefault("/nonexistent/path/config.json")
		if cfg == nil {
			t.Fatal("LoadConfigOrDefault returned nil")
		}
		// Should be equal to default config
		if cfg.Server.Port != 8080 {
			t.Error("Should return default config on missing file")
		}
	})

	t.Run("Environment overrides work", func(t *testing.T) {
		// Set environment variables
		os.Setenv("PORT", "9000")
		os.Setenv("TEMPORAL_HOST", "temporal:7233")
		os.Setenv("DB_HOST", "postgres")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("GRAPH_ENABLED", "true")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("METRICS_ENABLED", "false")
		os.Setenv("HEALTH_ENABLED", "false")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("TEMPORAL_HOST")
			os.Unsetenv("DB_HOST")
			os.Unsetenv("DB_PORT")
			os.Unsetenv("GRAPH_ENABLED")
			os.Unsetenv("LOG_LEVEL")
			os.Unsetenv("METRICS_ENABLED")
			os.Unsetenv("HEALTH_ENABLED")
		}()

		// Create a temp config file
		tmpFile, err := os.CreateTemp("", "config*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString("{}")
		tmpFile.Close()

		cfg, err := config.LoadConfig(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.Server.Port != 9000 {
			t.Errorf("Expected port 9000 from env, got %d", cfg.Server.Port)
		}
		if cfg.Temporal.Host != "temporal:7233" {
			t.Errorf("Expected temporal host 'temporal:7233' from env, got '%s'", cfg.Temporal.Host)
		}
		if cfg.Database.Host != "postgres" {
			t.Errorf("Expected db host 'postgres' from env, got '%s'", cfg.Database.Host)
		}
		if cfg.Database.Port != 5433 {
			t.Errorf("Expected db port 5433 from env, got %d", cfg.Database.Port)
		}
		if !cfg.Graph.Enabled {
			t.Error("Expected graph enabled from env")
		}
		if cfg.Observability.Logging.Level != "debug" {
			t.Errorf("Expected log level 'debug' from env, got '%s'", cfg.Observability.Logging.Level)
		}
		if cfg.Observability.Metrics.Enabled {
			t.Error("Expected metrics disabled from env")
		}
		if cfg.Observability.Health.Enabled {
			t.Error("Expected health disabled from env")
		}
	})

	t.Run("Risk thresholds are properly configured", func(t *testing.T) {
		cfg := config.DefaultConfig()

		if cfg.Agents.RiskThresholds.LowMax != 0.3 {
			t.Errorf("Expected low max 0.3, got %f", cfg.Agents.RiskThresholds.LowMax)
		}
		if cfg.Agents.RiskThresholds.MediumMax != 0.6 {
			t.Errorf("Expected medium max 0.6, got %f", cfg.Agents.RiskThresholds.MediumMax)
		}
		if cfg.Agents.RiskThresholds.HighMax != 0.8 {
			t.Errorf("Expected high max 0.8, got %f", cfg.Agents.RiskThresholds.HighMax)
		}
	})

	t.Run("Escalation rules are properly configured", func(t *testing.T) {
		cfg := config.DefaultConfig()

		if cfg.Agents.EscalationRules.AutoEscalateThreshold != 0.9 {
			t.Errorf("Expected auto escalate threshold 0.9, got %f", cfg.Agents.EscalationRules.AutoEscalateThreshold)
		}
		if cfg.Agents.EscalationRules.HumanReviewThreshold != 0.7 {
			t.Errorf("Expected human review threshold 0.7, got %f", cfg.Agents.EscalationRules.HumanReviewThreshold)
		}
		if cfg.Agents.EscalationRules.MaxAutoDecisionAmount != 100000 {
			t.Errorf("Expected max auto decision amount 100000, got %f", cfg.Agents.EscalationRules.MaxAutoDecisionAmount)
		}
		if cfg.Agents.EscalationRules.RequiredConfidence != 0.85 {
			t.Errorf("Expected required confidence 0.85, got %f", cfg.Agents.EscalationRules.RequiredConfidence)
		}
	})

	t.Run("Graph configuration is complete", func(t *testing.T) {
		cfg := config.DefaultConfig()

		if cfg.Graph.MaxIterations <= 0 {
			t.Error("Max iterations should be positive")
		}
		if !cfg.Graph.EnableCheckpoints {
			t.Error("Checkpoints should be enabled by default")
		}
		if cfg.Graph.CheckpointTTL <= 0 {
			t.Error("Checkpoint TTL should be positive")
		}
		if cfg.Graph.MaxRecursionDepth <= 0 {
			t.Error("Max recursion depth should be positive")
		}
		if !cfg.Graph.DetectLoops {
			t.Error("Loop detection should be enabled by default")
		}
		if cfg.Graph.LoopThreshold <= 0 {
			t.Error("Loop threshold should be positive")
		}
	})

	t.Run("Observability configuration is complete", func(t *testing.T) {
		cfg := config.DefaultConfig()

		// Logging
		if cfg.Observability.Logging.Format != "json" {
			t.Errorf("Expected log format 'json', got '%s'", cfg.Observability.Logging.Format)
		}
		if cfg.Observability.Logging.Output != "stdout" {
			t.Errorf("Expected log output 'stdout', got '%s'", cfg.Observability.Logging.Output)
		}

		// Metrics
		if cfg.Observability.Metrics.Endpoint != "/metrics" {
			t.Errorf("Expected metrics endpoint '/metrics', got '%s'", cfg.Observability.Metrics.Endpoint)
		}
		if cfg.Observability.Metrics.Namespace != "fraud_investigation" {
			t.Errorf("Expected metrics namespace 'fraud_investigation', got '%s'", cfg.Observability.Metrics.Namespace)
		}

		// Health
		if cfg.Observability.Health.LivenessPath != "/health/live" {
			t.Errorf("Expected liveness path '/health/live', got '%s'", cfg.Observability.Health.LivenessPath)
		}
		if cfg.Observability.Health.ReadinessPath != "/health/ready" {
			t.Errorf("Expected readiness path '/health/ready', got '%s'", cfg.Observability.Health.ReadinessPath)
		}
		if cfg.Observability.Health.CheckTimeout <= 0 {
			t.Error("Check timeout should be positive")
		}
		if cfg.Observability.Health.FailureThreshold <= 0 {
			t.Error("Failure threshold should be positive")
		}

		// Tracing
		if cfg.Observability.Tracing.Provider != "otlp" {
			t.Errorf("Expected tracing provider 'otlp', got '%s'", cfg.Observability.Tracing.Provider)
		}
		if cfg.Observability.Tracing.ServiceName != "fraud-investigation" {
			t.Errorf("Expected service name 'fraud-investigation', got '%s'", cfg.Observability.Tracing.ServiceName)
		}
	})
}

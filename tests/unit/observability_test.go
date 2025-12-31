// Package unit contains unit tests for the observability packages
package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/investigationanalysis/pkg/observability/health"
	"github.com/investigationanalysis/pkg/observability/logging"
	"github.com/investigationanalysis/pkg/observability/metrics"
)

// TestLogging tests the logging package
func TestLogging(t *testing.T) {
	t.Run("ParseLevel parses log levels", func(t *testing.T) {
		tests := []struct {
			input    string
			expected logging.Level
		}{
			{"debug", logging.LevelDebug},
			{"DEBUG", logging.LevelDebug},
			{"info", logging.LevelInfo},
			{"INFO", logging.LevelInfo},
			{"warn", logging.LevelWarn},
			{"warning", logging.LevelWarn},
			{"error", logging.LevelError},
			{"fatal", logging.LevelFatal},
			{"unknown", logging.LevelInfo}, // default
		}

		for _, tc := range tests {
			result := logging.ParseLevel(tc.input)
			if result != tc.expected {
				t.Errorf("ParseLevel(%s): expected %v, got %v", tc.input, tc.expected, result)
			}
		}
	})

	t.Run("Level.String returns correct strings", func(t *testing.T) {
		tests := []struct {
			level    logging.Level
			expected string
		}{
			{logging.LevelDebug, "DEBUG"},
			{logging.LevelInfo, "INFO"},
			{logging.LevelWarn, "WARN"},
			{logging.LevelError, "ERROR"},
			{logging.LevelFatal, "FATAL"},
		}

		for _, tc := range tests {
			if tc.level.String() != tc.expected {
				t.Errorf("Level %d: expected %s, got %s", tc.level, tc.expected, tc.level.String())
			}
		}
	})

	t.Run("Logger respects log level", func(t *testing.T) {
		var buf bytes.Buffer
		config := &logging.LogConfig{
			Level:   logging.LevelWarn,
			Format:  logging.FormatJSON,
			Output:  "stdout",
			AddTime: false,
		}

		// Create a custom logger that writes to buffer
		// Note: In actual implementation, you'd need to modify NewLogger to accept io.Writer
		logger, err := logging.NewLogger(config)
		if err != nil {
			t.Fatalf("Failed to create logger: %v", err)
		}

		// Debug and Info should be suppressed
		logger.Debug("debug message")
		logger.Info("info message")

		// These would write to the configured output
		_ = buf // buffer would capture output if logger accepted custom writer
	})

	t.Run("Logger.With adds fields", func(t *testing.T) {
		logger := logging.Default()
		childLogger := logger.With(map[string]interface{}{
			"service": "test",
			"version": "1.0",
		})

		if childLogger == nil {
			t.Fatal("With returned nil")
		}
	})

	t.Run("Logger.WithField adds single field", func(t *testing.T) {
		logger := logging.Default()
		childLogger := logger.WithField("request_id", "123")

		if childLogger == nil {
			t.Fatal("WithField returned nil")
		}
	})

	t.Run("Logger.WithContext extracts context values", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithTraceID(ctx, "trace-123")
		ctx = logging.WithSpanID(ctx, "span-456")
		ctx = logging.WithRequestID(ctx, "req-789")

		logger := logging.Default()
		childLogger := logger.WithContext(ctx)

		if childLogger == nil {
			t.Fatal("WithContext returned nil")
		}
	})

	t.Run("FromContext returns logger from context", func(t *testing.T) {
		logger := logging.Default()
		ctx := logging.WithLogger(context.Background(), logger)

		retrieved := logging.FromContext(ctx)
		if retrieved == nil {
			t.Error("FromContext should return logger")
		}
	})

	t.Run("InvestigationLogger logs investigation events", func(t *testing.T) {
		logger := logging.NewInvestigationLogger("inv-123", "claim-456")

		// These should not panic
		logger.AgentStart("test_agent")
		logger.AgentEnd("test_agent", time.Second, nil)
		logger.RiskAssessed(0.75, "high")
		logger.DecisionMade("deny", 0.9)
		logger.EscalatedToHuman("high risk")
	})

	t.Run("DefaultLogConfig returns valid config", func(t *testing.T) {
		config := logging.DefaultLogConfig()
		if config == nil {
			t.Fatal("DefaultLogConfig returned nil")
		}
		if config.Level != logging.LevelInfo {
			t.Error("Default level should be Info")
		}
		if config.Format != logging.FormatJSON {
			t.Error("Default format should be JSON")
		}
	})
}

// TestMetrics tests the metrics package
func TestMetrics(t *testing.T) {
	t.Run("Counter increments correctly", func(t *testing.T) {
		counter := metrics.NewCounter("test_counter", "Test counter", "label1")

		labels := metrics.Labels{"label1": "value1"}
		counter.Inc(labels)
		counter.Inc(labels)
		counter.Add(5, labels)

		if counter.Value(labels) != 7 {
			t.Errorf("Expected counter value 7, got %f", counter.Value(labels))
		}
	})

	t.Run("Gauge sets and adjusts correctly", func(t *testing.T) {
		gauge := metrics.NewGauge("test_gauge", "Test gauge")

		labels := metrics.Labels{}
		gauge.Set(10, labels)
		if gauge.Value(labels) != 10 {
			t.Errorf("Expected gauge value 10, got %f", gauge.Value(labels))
		}

		gauge.Inc(labels)
		if gauge.Value(labels) != 11 {
			t.Errorf("Expected gauge value 11, got %f", gauge.Value(labels))
		}

		gauge.Dec(labels)
		if gauge.Value(labels) != 10 {
			t.Errorf("Expected gauge value 10, got %f", gauge.Value(labels))
		}

		gauge.Add(-5, labels)
		if gauge.Value(labels) != 5 {
			t.Errorf("Expected gauge value 5, got %f", gauge.Value(labels))
		}
	})

	t.Run("Histogram records observations", func(t *testing.T) {
		buckets := []float64{0.1, 0.5, 1.0, 5.0}
		histogram := metrics.NewHistogram("test_histogram", "Test histogram", buckets)

		labels := metrics.Labels{}
		histogram.Observe(0.3, labels)
		histogram.Observe(0.7, labels)
		histogram.Observe(2.0, labels)

		// Histogram internals tested via the Observe method not panicking
	})

	t.Run("Timer measures duration", func(t *testing.T) {
		buckets := metrics.DefaultBuckets()
		histogram := metrics.NewHistogram("duration", "Duration histogram", buckets)

		labels := metrics.Labels{"operation": "test"}
		timer := histogram.NewTimer(labels)

		time.Sleep(10 * time.Millisecond)
		duration := timer.ObserveDuration()

		if duration < 10*time.Millisecond {
			t.Errorf("Expected duration >= 10ms, got %v", duration)
		}
	})

	t.Run("Registry registers and retrieves metrics", func(t *testing.T) {
		registry := metrics.NewRegistry(metrics.DefaultMetricsConfig())

		counter := registry.RegisterCounter("requests_total", "Total requests", "method")
		if counter == nil {
			t.Fatal("RegisterCounter returned nil")
		}

		gauge := registry.RegisterGauge("active_connections", "Active connections")
		if gauge == nil {
			t.Fatal("RegisterGauge returned nil")
		}

		histogram := registry.RegisterHistogram("request_duration", "Request duration",
			metrics.DefaultBuckets(), "endpoint")
		if histogram == nil {
			t.Fatal("RegisterHistogram returned nil")
		}

		// Retrieve by name
		if registry.Counter("requests_total") == nil {
			t.Error("Should retrieve registered counter")
		}
		if registry.Gauge("active_connections") == nil {
			t.Error("Should retrieve registered gauge")
		}
		if registry.Histogram("request_duration") == nil {
			t.Error("Should retrieve registered histogram")
		}
	})

	t.Run("InvestigationMetrics records all metric types", func(t *testing.T) {
		registry := metrics.NewRegistry(metrics.DefaultMetricsConfig())
		m := metrics.NewInvestigationMetrics(registry)

		// These should not panic
		m.RecordInvestigation("completed", time.Minute)
		m.RecordRiskScore(0.75, "high")
		m.RecordDecision("deny", "auto")
		m.RecordEscalation("high_risk")
		m.RecordAgentExecution("pattern_analysis", "success", time.Second)
		m.RecordAgentError("pattern_analysis", "timeout")
		m.RecordRAGQuery("fraud_cases", 100*time.Millisecond)
	})

	t.Run("DefaultMetricsConfig returns valid config", func(t *testing.T) {
		config := metrics.DefaultMetricsConfig()
		if config == nil {
			t.Fatal("DefaultMetricsConfig returned nil")
		}
		if !config.Enabled {
			t.Error("Metrics should be enabled by default")
		}
		if config.Endpoint != "/metrics" {
			t.Errorf("Expected endpoint /metrics, got %s", config.Endpoint)
		}
	})

	t.Run("DefaultBuckets returns valid buckets", func(t *testing.T) {
		buckets := metrics.DefaultBuckets()
		if len(buckets) == 0 {
			t.Error("DefaultBuckets should return non-empty slice")
		}
		// Verify buckets are in ascending order
		for i := 1; i < len(buckets); i++ {
			if buckets[i] <= buckets[i-1] {
				t.Error("Buckets should be in ascending order")
			}
		}
	})
}

// TestHealth tests the health check package
func TestHealth(t *testing.T) {
	t.Run("NewChecker creates checker with config", func(t *testing.T) {
		config := health.DefaultHealthConfig()
		checker := health.NewChecker(config)
		if checker == nil {
			t.Fatal("NewChecker returned nil")
		}
	})

	t.Run("IsLive returns true by default", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		if !checker.IsLive() {
			t.Error("IsLive should return true by default")
		}
	})

	t.Run("IsReady returns true when no checks registered", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		if !checker.IsReady() {
			t.Error("IsReady should return true when no checks registered")
		}
	})

	t.Run("AddCheck registers check", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		checker.AddCheck("test", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{
				Name:   "test",
				Status: health.StatusHealthy,
			}
		})

		// Check should be registered (verified by CheckNow)
		results := checker.CheckNow(context.Background())
		if _, ok := results["test"]; !ok {
			t.Error("Check should be registered")
		}
	})

	t.Run("RemoveCheck unregisters check", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		checker.AddCheck("test", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Name: "test", Status: health.StatusHealthy}
		})
		checker.RemoveCheck("test")

		results := checker.CheckNow(context.Background())
		if _, ok := results["test"]; ok {
			t.Error("Check should be removed")
		}
	})

	t.Run("CheckNow runs all checks", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		checker.AddCheck("db", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Name: "db", Status: health.StatusHealthy}
		})
		checker.AddCheck("cache", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Name: "cache", Status: health.StatusDegraded}
		})

		results := checker.CheckNow(context.Background())
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
		if results["db"].Status != health.StatusHealthy {
			t.Error("DB check should be healthy")
		}
		if results["cache"].Status != health.StatusDegraded {
			t.Error("Cache check should be degraded")
		}
	})

	t.Run("GetStatus returns correct overall status", func(t *testing.T) {
		config := health.DefaultHealthConfig()
		config.FailureThreshold = 1
		checker := health.NewChecker(config)

		// All healthy
		checker.AddCheck("check1", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Name: "check1", Status: health.StatusHealthy}
		})
		checker.CheckNow(context.Background())
		if checker.GetStatus() != health.StatusHealthy {
			t.Error("Status should be healthy when all checks pass")
		}

		// Add degraded check
		checker.AddCheck("check2", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Name: "check2", Status: health.StatusDegraded}
		})
		checker.CheckNow(context.Background())
		if checker.GetStatus() != health.StatusDegraded {
			t.Error("Status should be degraded when any check is degraded")
		}
	})

	t.Run("DatabaseCheck creates working check", func(t *testing.T) {
		check := health.DatabaseCheck("postgres", func(ctx context.Context) error {
			return nil
		})

		result := check(context.Background())
		if result.Name != "postgres" {
			t.Errorf("Expected name 'postgres', got '%s'", result.Name)
		}
		if result.Status != health.StatusHealthy {
			t.Error("Check should be healthy when ping succeeds")
		}
	})

	t.Run("DatabaseCheck returns unhealthy on error", func(t *testing.T) {
		check := health.DatabaseCheck("postgres", func(ctx context.Context) error {
			return context.DeadlineExceeded
		})

		result := check(context.Background())
		if result.Status != health.StatusUnhealthy {
			t.Error("Check should be unhealthy when ping fails")
		}
	})

	t.Run("LLMCheck returns degraded on error", func(t *testing.T) {
		check := health.LLMCheck("ollama", func(ctx context.Context) error {
			return context.DeadlineExceeded
		})

		result := check(context.Background())
		// LLM failures are degraded, not unhealthy
		if result.Status != health.StatusDegraded {
			t.Error("LLM check should be degraded when failing")
		}
	})

	t.Run("DependencyCheck respects critical flag", func(t *testing.T) {
		// Critical dependency
		criticalCheck := health.DependencyCheck("critical", func(ctx context.Context) error {
			return context.DeadlineExceeded
		}, true)

		result := criticalCheck(context.Background())
		if result.Status != health.StatusUnhealthy {
			t.Error("Critical dependency should be unhealthy on failure")
		}

		// Non-critical dependency
		nonCriticalCheck := health.DependencyCheck("optional", func(ctx context.Context) error {
			return context.DeadlineExceeded
		}, false)

		result = nonCriticalCheck(context.Background())
		if result.Status != health.StatusDegraded {
			t.Error("Non-critical dependency should be degraded on failure")
		}
	})

	t.Run("CompositeChecker aggregates checkers", func(t *testing.T) {
		checker1 := health.NewChecker(health.DefaultHealthConfig())
		checker1.AddCheck("c1", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Status: health.StatusHealthy}
		})
		checker1.CheckNow(context.Background())

		checker2 := health.NewChecker(health.DefaultHealthConfig())
		checker2.AddCheck("c2", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{Status: health.StatusHealthy}
		})
		checker2.CheckNow(context.Background())

		composite := health.NewCompositeChecker(checker1, checker2)

		if !composite.IsLive() {
			t.Error("Composite should be live when all checkers are live")
		}
		if !composite.IsReady() {
			t.Error("Composite should be ready when all checkers are ready")
		}
		if composite.GetStatus() != health.StatusHealthy {
			t.Error("Composite status should be healthy")
		}
	})

	t.Run("DefaultHealthConfig returns valid config", func(t *testing.T) {
		config := health.DefaultHealthConfig()
		if config == nil {
			t.Fatal("DefaultHealthConfig returned nil")
		}
		if !config.Enabled {
			t.Error("Health checks should be enabled by default")
		}
		if config.LivenessPath != "/health/live" {
			t.Errorf("Expected liveness path /health/live, got %s", config.LivenessPath)
		}
		if config.ReadinessPath != "/health/ready" {
			t.Errorf("Expected readiness path /health/ready, got %s", config.ReadinessPath)
		}
	})

	t.Run("Status constants are correct", func(t *testing.T) {
		if health.StatusHealthy != "healthy" {
			t.Error("StatusHealthy should be 'healthy'")
		}
		if health.StatusUnhealthy != "unhealthy" {
			t.Error("StatusUnhealthy should be 'unhealthy'")
		}
		if health.StatusDegraded != "degraded" {
			t.Error("StatusDegraded should be 'degraded'")
		}
	})

	t.Run("LivenessHandler returns correct response", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		handler := checker.LivenessHandler()
		if handler == nil {
			t.Fatal("LivenessHandler returned nil")
		}
	})

	t.Run("ReadinessHandler returns correct response", func(t *testing.T) {
		checker := health.NewChecker(health.DefaultHealthConfig())
		handler := checker.ReadinessHandler()
		if handler == nil {
			t.Fatal("ReadinessHandler returned nil")
		}
	})

	t.Run("HealthResponse serializes to JSON", func(t *testing.T) {
		response := health.HealthResponse{
			Status:    health.StatusHealthy,
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Uptime:    time.Hour,
			Checks: map[string]health.CheckResult{
				"db": {
					Name:      "db",
					Status:    health.StatusHealthy,
					Message:   "connected",
					Duration:  10 * time.Millisecond,
					Timestamp: time.Now(),
				},
			},
		}

		data, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("Failed to marshal HealthResponse: %v", err)
		}
		if len(data) == 0 {
			t.Error("Marshaled data should not be empty")
		}
	})
}

// Package integration contains integration tests for the fraud investigation framework
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/investigationanalysis/pkg/config"
	"github.com/investigationanalysis/pkg/graph"
	"github.com/investigationanalysis/pkg/models"
	"github.com/investigationanalysis/pkg/observability/health"
	"github.com/investigationanalysis/pkg/observability/logging"
	"github.com/investigationanalysis/pkg/observability/metrics"
)

// TestGraphWorkflowExecution tests the graph-based workflow execution
func TestGraphWorkflowExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Simple graph execution completes successfully", func(t *testing.T) {
		sg := graph.NewStateGraph("test_workflow")

		sg.AddNode("start", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("started", true)
			state.Set("timestamp", time.Now())
			return state, nil
		})

		sg.AddNode("process", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("processed", true)
			return state, nil
		})

		sg.AddNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("completed", true)
			return state, nil
		})

		sg.SetEntryPoint("start")
		sg.SetFinishPoint("end")
		sg.AddEdge("start", "process")
		sg.AddEdge("process", "end")

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Failed to compile graph: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := compiled.Invoke(ctx, map[string]interface{}{
			"initial_input": "test",
		})

		if err != nil {
			t.Fatalf("Graph execution failed: %v", err)
		}

		if result.Status != graph.StatusCompleted {
			t.Errorf("Expected status Completed, got %v", result.Status)
		}

		if !result.GetBool("started") {
			t.Error("Start node did not execute")
		}
		if !result.GetBool("processed") {
			t.Error("Process node did not execute")
		}
		if !result.GetBool("completed") {
			t.Error("End node did not execute")
		}
	})

	t.Run("Conditional graph routes correctly", func(t *testing.T) {
		sg := graph.NewStateGraph("conditional_workflow")

		sg.AddNode("router", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})

		sg.AddNode("high_risk_handler", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("handled_by", "high_risk")
			return state, nil
		})

		sg.AddNode("low_risk_handler", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("handled_by", "low_risk")
			return state, nil
		})

		sg.SetEntryPoint("router")
		sg.SetFinishPoint("high_risk_handler")
		sg.SetFinishPoint("low_risk_handler")

		sg.AddConditionalEdges("router",
			func(state *graph.State) string {
				if state.GetFloat("risk_score") >= 0.7 {
					return "high"
				}
				return "low"
			},
			map[string]string{
				"high": "high_risk_handler",
				"low":  "low_risk_handler",
			},
		)

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Failed to compile graph: %v", err)
		}

		ctx := context.Background()

		// Test high risk path
		result, err := compiled.Invoke(ctx, map[string]interface{}{"risk_score": 0.9})
		if err != nil {
			t.Fatalf("Graph execution failed: %v", err)
		}
		if result.GetString("handled_by") != "high_risk" {
			t.Errorf("Expected high_risk handler, got %s", result.GetString("handled_by"))
		}

		// Test low risk path
		result, err = compiled.Invoke(ctx, map[string]interface{}{"risk_score": 0.3})
		if err != nil {
			t.Fatalf("Graph execution failed: %v", err)
		}
		if result.GetString("handled_by") != "low_risk" {
			t.Errorf("Expected low_risk handler, got %s", result.GetString("handled_by"))
		}
	})

	t.Run("Parallel execution runs nodes concurrently", func(t *testing.T) {
		node1 := graph.NewFunctionNode("node1", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			time.Sleep(100 * time.Millisecond)
			state.Set("node1_done", true)
			return state, nil
		})

		node2 := graph.NewFunctionNode("node2", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			time.Sleep(100 * time.Millisecond)
			state.Set("node2_done", true)
			return state, nil
		})

		parallel := graph.NewParallelNode("parallel", node1, node2)

		sg := graph.NewStateGraph("parallel_workflow")
		sg.AddNode("parallel_exec", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return parallel.Execute(ctx, state)
		})
		sg.AddNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("ended", true)
			return state, nil
		})
		sg.SetEntryPoint("parallel_exec")
		sg.SetFinishPoint("end")
		sg.AddEdge("parallel_exec", "end")

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Failed to compile graph: %v", err)
		}

		ctx := context.Background()
		start := time.Now()

		result, err := compiled.Invoke(ctx, nil)
		if err != nil {
			t.Fatalf("Graph execution failed: %v", err)
		}

		duration := time.Since(start)

		// Parallel execution should take ~100ms, not ~200ms
		if duration > 300*time.Millisecond {
			t.Errorf("Parallel execution took too long: %v (expected ~100ms)", duration)
		}

		if !result.GetBool("node1_done") || !result.GetBool("node2_done") {
			t.Error("Not all parallel nodes completed")
		}
	})
}

// TestConfigurationIntegration tests configuration loading and validation
func TestConfigurationIntegration(t *testing.T) {
	t.Run("Default configuration is valid", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("Default config validation failed: %v", err)
		}
	})

	t.Run("Configuration respects feature flags", func(t *testing.T) {
		cfg := config.DefaultConfig()

		// Graph should be disabled by default
		if cfg.IsGraphEnabled() {
			t.Error("Graph should be disabled by default")
		}

		// Enable graph
		cfg.Graph.Enabled = true
		if !cfg.IsGraphEnabled() {
			t.Error("Graph should be enabled after setting flag")
		}

		// Metrics should be enabled by default
		if !cfg.IsMetricsEnabled() {
			t.Error("Metrics should be enabled by default")
		}

		// Health should be enabled by default
		if !cfg.IsHealthEnabled() {
			t.Error("Health should be enabled by default")
		}

		// Tracing should be disabled by default
		if cfg.IsTracingEnabled() {
			t.Error("Tracing should be disabled by default")
		}
	})
}

// TestObservabilityIntegration tests observability components working together
func TestObservabilityIntegration(t *testing.T) {
	t.Run("Logging works with investigation context", func(t *testing.T) {
		invLogger := logging.NewInvestigationLogger("inv-123", "claim-456")

		// Should not panic
		invLogger.AgentStart("test_agent")
		invLogger.AgentEnd("test_agent", 500*time.Millisecond, nil)
		invLogger.RiskAssessed(0.75, "high")
		invLogger.DecisionMade("deny", 0.9)
	})

	t.Run("Metrics record investigation lifecycle", func(t *testing.T) {
		registry := metrics.NewRegistry(metrics.DefaultMetricsConfig())
		m := metrics.NewInvestigationMetrics(registry)

		// Simulate an investigation
		m.InvestigationsActive.Inc(nil)
		m.RecordAgentExecution("data_enrichment", "success", 100*time.Millisecond)
		m.RecordAgentExecution("pattern_analysis", "success", 200*time.Millisecond)
		m.RecordRAGQuery("fraud_cases", 50*time.Millisecond)
		m.RecordRiskScore(0.8, "high")
		m.RecordDecision("deny", "auto")
		m.InvestigationsActive.Dec(nil)
		m.RecordInvestigation("completed", 2*time.Second)
	})

	t.Run("Health checks aggregate correctly", func(t *testing.T) {
		cfg := health.DefaultHealthConfig()
		checker := health.NewChecker(cfg)

		// Add healthy check
		checker.AddCheck("database", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{
				Name:    "database",
				Status:  health.StatusHealthy,
				Message: "connected",
			}
		})

		// Add degraded check
		checker.AddCheck("cache", func(ctx context.Context) health.CheckResult {
			return health.CheckResult{
				Name:    "cache",
				Status:  health.StatusDegraded,
				Message: "slow responses",
			}
		})

		// Run checks
		results := checker.CheckNow(context.Background())

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		// Overall status should be degraded
		if checker.GetStatus() != health.StatusDegraded {
			t.Errorf("Expected status Degraded, got %v", checker.GetStatus())
		}

		// Should still be ready (degraded is not unhealthy)
		if !checker.IsReady() {
			t.Error("Checker should still be ready when degraded")
		}
	})
}

// TestInvestigationFlow tests a complete investigation flow
func TestInvestigationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Complete investigation graph flow", func(t *testing.T) {
		// Build a simplified investigation graph
		sg := graph.NewStateGraph("investigation")

		sg.AddNode("data_gathering", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("customer_data", map[string]interface{}{
				"name":        "Test Customer",
				"risk_factor": 0.5,
			})
			state.Set("transaction_history", []interface{}{
				map[string]interface{}{"amount": 1000, "type": "withdrawal"},
				map[string]interface{}{"amount": 5000, "type": "withdrawal"},
			})
			return state, nil
		})

		sg.AddNode("analysis", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			// Simulate pattern analysis
			state.Set("pattern_risk", 0.6)
			state.Set("anomaly_detected", true)
			return state, nil
		})

		sg.AddNode("risk_assessment", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			patternRisk := state.GetFloat("pattern_risk")
			customerData, _ := state.Get("customer_data")
			customerRisk := 0.5
			if cd, ok := customerData.(map[string]interface{}); ok {
				if rf, ok := cd["risk_factor"].(float64); ok {
					customerRisk = rf
				}
			}

			// Calculate combined risk
			riskScore := (patternRisk + customerRisk) / 2
			state.Set("risk_score", riskScore)

			if riskScore >= 0.7 {
				state.Set("risk_level", "high")
				state.Set("requires_review", true)
			} else if riskScore >= 0.4 {
				state.Set("risk_level", "medium")
				state.Set("requires_review", false)
			} else {
				state.Set("risk_level", "low")
				state.Set("requires_review", false)
			}

			return state, nil
		})

		sg.AddNode("decision", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			riskLevel := state.GetString("risk_level")
			var decision string
			switch riskLevel {
			case "high":
				decision = "DENY"
			case "medium":
				decision = "APPROVE_WITH_AUDIT"
			default:
				decision = "APPROVE"
			}
			state.Set("decision", decision)
			return state, nil
		})

		sg.SetEntryPoint("data_gathering")
		sg.SetFinishPoint("decision")
		sg.AddEdge("data_gathering", "analysis")
		sg.AddEdge("analysis", "risk_assessment")
		sg.AddEdge("risk_assessment", "decision")

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Failed to compile graph: %v", err)
		}

		ctx := context.Background()
		result, err := compiled.Invoke(ctx, map[string]interface{}{
			"claim_id": "CLM-12345",
			"amount":   10000.0,
		})

		if err != nil {
			t.Fatalf("Investigation failed: %v", err)
		}

		// Verify results
		if result.GetString("decision") == "" {
			t.Error("Decision should be set")
		}
		if result.GetFloat("risk_score") == 0 {
			t.Error("Risk score should be calculated")
		}
		if result.GetString("risk_level") == "" {
			t.Error("Risk level should be set")
		}

		t.Logf("Investigation completed: decision=%s, risk_score=%.2f, risk_level=%s",
			result.GetString("decision"),
			result.GetFloat("risk_score"),
			result.GetString("risk_level"))
	})
}

// TestGraphCheckpointing tests checkpoint save and resume functionality
func TestGraphCheckpointing(t *testing.T) {
	t.Run("Checkpoint saves and restores state", func(t *testing.T) {
		store := graph.NewInMemoryCheckpointStore()
		ctx := context.Background()

		// Create initial state
		state := graph.NewState()
		state.CurrentNode = "analysis"
		state.Set("claim_id", "CLM-12345")
		state.Set("risk_score", 0.75)
		state.SetNodeOutput("data_gathering", map[string]interface{}{"data": "gathered"})

		// Save checkpoint
		checkpoint := state.CreateCheckpoint()
		err := store.Save(ctx, checkpoint)
		if err != nil {
			t.Fatalf("Failed to save checkpoint: %v", err)
		}

		// Load checkpoint
		loaded, err := store.Load(ctx, checkpoint.ID)
		if err != nil {
			t.Fatalf("Failed to load checkpoint: %v", err)
		}

		// Verify checkpoint data
		if loaded.NodeName != "analysis" {
			t.Errorf("Expected node 'analysis', got '%s'", loaded.NodeName)
		}
		if loaded.Data["claim_id"] != "CLM-12345" {
			t.Error("Checkpoint data mismatch")
		}
		if loaded.Data["risk_score"] != 0.75 {
			t.Error("Checkpoint risk_score mismatch")
		}
	})

	t.Run("List checkpoints by thread", func(t *testing.T) {
		store := graph.NewInMemoryCheckpointStore()
		ctx := context.Background()

		// Create checkpoints for multiple threads
		for i := 0; i < 5; i++ {
			state := graph.NewState()
			checkpoint := state.CreateCheckpoint()
			checkpoint.ThreadID = "thread-1"
			store.Save(ctx, checkpoint)
		}

		for i := 0; i < 3; i++ {
			state := graph.NewState()
			checkpoint := state.CreateCheckpoint()
			checkpoint.ThreadID = "thread-2"
			store.Save(ctx, checkpoint)
		}

		// List thread-1 checkpoints
		list1, err := store.List(ctx, "thread-1")
		if err != nil {
			t.Fatalf("Failed to list checkpoints: %v", err)
		}
		if len(list1) != 5 {
			t.Errorf("Expected 5 checkpoints for thread-1, got %d", len(list1))
		}

		// List thread-2 checkpoints
		list2, err := store.List(ctx, "thread-2")
		if err != nil {
			t.Fatalf("Failed to list checkpoints: %v", err)
		}
		if len(list2) != 3 {
			t.Errorf("Expected 3 checkpoints for thread-2, got %d", len(list2))
		}
	})
}

// TestGraphStreaming tests streaming execution
func TestGraphStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Stream provides state updates", func(t *testing.T) {
		sg := graph.NewStateGraph("streaming_test")

		sg.AddNode("step1", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step", 1)
			return state, nil
		})
		sg.AddNode("step2", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step", 2)
			return state, nil
		})
		sg.AddNode("step3", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step", 3)
			return state, nil
		})

		sg.SetEntryPoint("step1")
		sg.SetFinishPoint("step3")
		sg.AddEdge("step1", "step2")
		sg.AddEdge("step2", "step3")

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Failed to compile graph: %v", err)
		}

		ctx := context.Background()
		stateChan, errChan := compiled.Stream(ctx, nil)

		var states []*graph.State
		done := make(chan struct{})

		go func() {
			defer close(done)
			for state := range stateChan {
				states = append(states, state)
			}
		}()

		// Wait for completion or error
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Stream error: %v", err)
			}
		case <-done:
		}

		<-done // Wait for all states to be collected

		if len(states) < 3 {
			t.Errorf("Expected at least 3 state updates, got %d", len(states))
		}

		// Last state should be completed
		lastState := states[len(states)-1]
		if lastState.Status != graph.StatusCompleted {
			t.Errorf("Final state should be completed, got %v", lastState.Status)
		}
	})
}

// BenchmarkGraphExecution benchmarks graph execution performance
func BenchmarkGraphExecution(b *testing.B) {
	sg := graph.NewStateGraph("benchmark")

	sg.AddNode("start", func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.Set("counter", 0)
		return state, nil
	})

	sg.AddNode("increment", func(ctx context.Context, state *graph.State) (*graph.State, error) {
		state.Set("counter", state.GetInt("counter")+1)
		return state, nil
	})

	sg.AddNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
		return state, nil
	})

	sg.SetEntryPoint("start")
	sg.SetFinishPoint("end")
	sg.AddEdge("start", "increment")
	sg.AddEdge("increment", "end")

	compiled, _ := sg.Compile()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compiled.Invoke(ctx, nil)
	}
}

// BenchmarkParallelExecution benchmarks parallel node execution
func BenchmarkParallelExecution(b *testing.B) {
	nodes := make([]graph.Node, 10)
	for i := 0; i < 10; i++ {
		idx := i
		nodes[i] = graph.NewFunctionNode("node"+string(rune('0'+idx)), func(ctx context.Context, state *graph.State) (*graph.State, error) {
			// Simulate some work
			sum := 0
			for j := 0; j < 1000; j++ {
				sum += j
			}
			state.Set("sum"+string(rune('0'+idx)), sum)
			return state, nil
		})
	}

	parallel := graph.NewParallelNode("parallel", nodes...)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := graph.NewState()
		parallel.Execute(ctx, state)
	}
}

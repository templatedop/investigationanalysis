// Package unit contains unit tests for the fraud investigation framework
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/investigationanalysis/pkg/graph"
)

// TestState tests the State struct functionality
func TestState(t *testing.T) {
	t.Run("NewState creates empty state", func(t *testing.T) {
		state := graph.NewState()
		if state == nil {
			t.Fatal("NewState returned nil")
		}
		if state.ID == "" {
			t.Error("State ID should not be empty")
		}
		if state.Status != graph.StatusPending {
			t.Errorf("Expected status %v, got %v", graph.StatusPending, state.Status)
		}
	})

	t.Run("Set and Get work correctly", func(t *testing.T) {
		state := graph.NewState()
		state.Set("key1", "value1")
		state.Set("key2", 42)
		state.Set("key3", true)

		val, ok := state.Get("key1")
		if !ok || val != "value1" {
			t.Error("Failed to get string value")
		}

		val, ok = state.Get("key2")
		if !ok || val != 42 {
			t.Error("Failed to get int value")
		}

		val, ok = state.Get("key3")
		if !ok || val != true {
			t.Error("Failed to get bool value")
		}

		_, ok = state.Get("nonexistent")
		if ok {
			t.Error("Should return false for nonexistent key")
		}
	})

	t.Run("GetString works correctly", func(t *testing.T) {
		state := graph.NewState()
		state.Set("string_key", "hello")
		state.Set("int_key", 42)

		if state.GetString("string_key") != "hello" {
			t.Error("Failed to get string")
		}
		if state.GetString("int_key") != "" {
			t.Error("Should return empty string for non-string")
		}
		if state.GetString("nonexistent") != "" {
			t.Error("Should return empty string for nonexistent key")
		}
	})

	t.Run("GetFloat works correctly", func(t *testing.T) {
		state := graph.NewState()
		state.Set("float_key", 3.14)
		state.Set("int_key", 42)

		if state.GetFloat("float_key") != 3.14 {
			t.Error("Failed to get float")
		}
		// Int should also work
		if state.GetFloat("int_key") != 42.0 {
			t.Error("Failed to get int as float")
		}
	})

	t.Run("GetInt works correctly", func(t *testing.T) {
		state := graph.NewState()
		state.Set("int_key", 42)

		if state.GetInt("int_key") != 42 {
			t.Error("Failed to get int")
		}
		if state.GetInt("nonexistent") != 0 {
			t.Error("Should return 0 for nonexistent key")
		}
	})

	t.Run("GetBool works correctly", func(t *testing.T) {
		state := graph.NewState()
		state.Set("bool_key", true)

		if state.GetBool("bool_key") != true {
			t.Error("Failed to get bool")
		}
		if state.GetBool("nonexistent") != false {
			t.Error("Should return false for nonexistent key")
		}
	})

	t.Run("NodeOutputs work correctly", func(t *testing.T) {
		state := graph.NewState()
		output := map[string]interface{}{"result": "success"}
		state.SetNodeOutput("node1", output)

		result, ok := state.GetNodeOutput("node1")
		if !ok {
			t.Error("Failed to get node output")
		}
		if result.(map[string]interface{})["result"] != "success" {
			t.Error("Node output mismatch")
		}
	})

	t.Run("Clone creates independent copy", func(t *testing.T) {
		state := graph.NewState()
		state.Set("key", "value1")

		clone := state.Clone()
		clone.Set("key", "value2")

		if state.GetString("key") != "value1" {
			t.Error("Original state was modified by clone")
		}
		if clone.GetString("key") != "value2" {
			t.Error("Clone should have new value")
		}
	})

	t.Run("RecordTransition tracks history", func(t *testing.T) {
		state := graph.NewState()
		state.RecordTransition("node1", "node2")
		state.RecordTransition("node2", "node3")

		if len(state.History) != 2 {
			t.Errorf("Expected 2 transitions, got %d", len(state.History))
		}
	})

	t.Run("CreateCheckpoint creates checkpoint", func(t *testing.T) {
		state := graph.NewState()
		state.CurrentNode = "test_node"
		state.Set("key", "value")

		checkpoint := state.CreateCheckpoint()
		if checkpoint == nil {
			t.Fatal("Checkpoint is nil")
		}
		if checkpoint.NodeName != "test_node" {
			t.Error("Checkpoint node name mismatch")
		}
	})

	t.Run("AddMessage adds messages", func(t *testing.T) {
		state := graph.NewState()
		state.AddMessage(graph.RoleUser, "Hello")
		state.AddMessage(graph.RoleAssistant, "Hi there!")

		if len(state.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(state.Messages))
		}
		if state.Messages[0].Role != graph.RoleUser {
			t.Error("First message role mismatch")
		}
	})
}

// TestNode tests the Node interface implementations
func TestNode(t *testing.T) {
	t.Run("FunctionNode executes correctly", func(t *testing.T) {
		executed := false
		node := graph.NewFunctionNode("test_func", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			executed = true
			state.Set("executed", true)
			return state, nil
		})

		if node.Name() != "test_func" {
			t.Errorf("Expected name 'test_func', got '%s'", node.Name())
		}
		if node.Type() != graph.NodeTypeFunction {
			t.Errorf("Expected type %v, got %v", graph.NodeTypeFunction, node.Type())
		}

		state := graph.NewState()
		result, err := node.Execute(context.Background(), state)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !executed {
			t.Error("Function was not executed")
		}
		if !result.GetBool("executed") {
			t.Error("State was not modified")
		}
	})

	t.Run("FunctionNode handles errors", func(t *testing.T) {
		expectedErr := errors.New("test error")
		node := graph.NewFunctionNode("error_func", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, expectedErr
		})

		state := graph.NewState()
		_, err := node.Execute(context.Background(), state)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("ConditionNode routes correctly", func(t *testing.T) {
		evaluator := func(state *graph.State) string {
			if state.GetBool("high_risk") {
				return "risk_path"
			}
			return "normal_path"
		}

		node := graph.NewConditionNode("risk_check", evaluator)

		if node.Type() != graph.NodeTypeCondition {
			t.Errorf("Expected type %v, got %v", graph.NodeTypeCondition, node.Type())
		}

		// Test high risk path
		state := graph.NewState()
		state.Set("high_risk", true)
		result, err := node.Execute(context.Background(), state)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.GetString("next_node") != "risk_path" {
			t.Error("Should route to risk_path")
		}

		// Test normal path
		state = graph.NewState()
		state.Set("high_risk", false)
		result, err = node.Execute(context.Background(), state)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.GetString("next_node") != "normal_path" {
			t.Error("Should route to normal_path")
		}
	})

	t.Run("ParallelNode executes nodes in parallel", func(t *testing.T) {
		node1 := graph.NewFunctionNode("node1", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("node1_done", true)
			return state, nil
		})
		node2 := graph.NewFunctionNode("node2", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("node2_done", true)
			return state, nil
		})

		parallel := graph.NewParallelNode("parallel", node1, node2)

		if parallel.Type() != graph.NodeTypeParallel {
			t.Errorf("Expected type %v, got %v", graph.NodeTypeParallel, parallel.Type())
		}

		state := graph.NewState()
		result, err := parallel.Execute(context.Background(), state)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Both nodes should have executed
		if !result.GetBool("node1_done") {
			t.Error("node1 should have executed")
		}
		if !result.GetBool("node2_done") {
			t.Error("node2 should have executed")
		}
	})

	t.Run("HumanNode returns interrupt error", func(t *testing.T) {
		handler := func(state *graph.State) (*graph.State, error) {
			return state, nil
		}
		node := graph.NewHumanNode("human_review", handler)

		if node.Type() != graph.NodeTypeHuman {
			t.Errorf("Expected type %v, got %v", graph.NodeTypeHuman, node.Type())
		}

		state := graph.NewState()
		_, err := node.Execute(context.Background(), state)
		if !errors.Is(err, graph.ErrHumanInterrupt) {
			t.Errorf("Expected ErrHumanInterrupt, got %v", err)
		}
	})
}

// TestEdge tests edge and routing functionality
func TestEdge(t *testing.T) {
	t.Run("NewEdge creates unconditional edge", func(t *testing.T) {
		edge := graph.NewEdge("from", "to")
		if edge.From != "from" || edge.To != "to" {
			t.Error("Edge endpoints mismatch")
		}

		state := graph.NewState()
		if !edge.ShouldFollow(state) {
			t.Error("Unconditional edge should always be followed")
		}
	})

	t.Run("NewConditionalEdge respects condition", func(t *testing.T) {
		condition := func(state *graph.State) bool {
			return state.GetBool("proceed")
		}
		edge := graph.NewConditionalEdge("from", "to", condition)

		state := graph.NewState()
		if edge.ShouldFollow(state) {
			t.Error("Should not follow when condition is false")
		}

		state.Set("proceed", true)
		if !edge.ShouldFollow(state) {
			t.Error("Should follow when condition is true")
		}
	})

	t.Run("ConditionalEdges evaluate correctly", func(t *testing.T) {
		ce := graph.NewConditionalEdges("router")
		ce.AddCondition("high", graph.RiskAboveThreshold(0.7))
		ce.AddCondition("low", graph.RiskBelowThreshold(0.3))
		ce.SetDefault("medium")

		// High risk
		state := graph.NewState()
		state.Set("risk_score", 0.9)
		if ce.Evaluate(state) != "high" {
			t.Error("Should route to high for risk > 0.7")
		}

		// Low risk
		state = graph.NewState()
		state.Set("risk_score", 0.2)
		if ce.Evaluate(state) != "low" {
			t.Error("Should route to low for risk <= 0.3")
		}

		// Medium risk (default)
		state = graph.NewState()
		state.Set("risk_score", 0.5)
		result := ce.Evaluate(state)
		if result != "medium" {
			t.Errorf("Should route to medium (default), got %s", result)
		}
	})

	t.Run("Router returns correct next nodes", func(t *testing.T) {
		router := graph.NewRouter()
		router.AddEdge(graph.NewEdge("a", "b"))
		router.AddEdge(graph.NewEdge("b", "c"))

		state := graph.NewState()
		next := router.GetNextNodes("a", state)
		if len(next) != 1 || next[0] != "b" {
			t.Errorf("Expected [b], got %v", next)
		}
	})

	t.Run("Router respects router function priority", func(t *testing.T) {
		router := graph.NewRouter()
		router.AddEdge(graph.NewEdge("a", "b"))
		router.AddRouterFunc("a", func(state *graph.State) string {
			return "c"
		})

		state := graph.NewState()
		next := router.GetNextNodes("a", state)
		if len(next) != 1 || next[0] != "c" {
			t.Errorf("Router func should take priority, expected [c], got %v", next)
		}
	})
}

// TestEdgeConditions tests built-in edge conditions
func TestEdgeConditions(t *testing.T) {
	t.Run("RiskAboveThreshold", func(t *testing.T) {
		cond := graph.RiskAboveThreshold(0.5)
		state := graph.NewState()

		state.Set("risk_score", 0.6)
		if !cond(state) {
			t.Error("Should return true for risk > threshold")
		}

		state.Set("risk_score", 0.4)
		if cond(state) {
			t.Error("Should return false for risk <= threshold")
		}
	})

	t.Run("HasIndicators", func(t *testing.T) {
		cond := graph.HasIndicators(2)
		state := graph.NewState()

		state.Set("risk_indicators", []interface{}{"indicator1", "indicator2", "indicator3"})
		if !cond(state) {
			t.Error("Should return true when indicators >= minCount")
		}

		state.Set("risk_indicators", []interface{}{"indicator1"})
		if cond(state) {
			t.Error("Should return false when indicators < minCount")
		}
	})

	t.Run("RequiresHumanReview", func(t *testing.T) {
		cond := graph.RequiresHumanReview()
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when not set")
		}

		state.Set("requires_human_review", true)
		if !cond(state) {
			t.Error("Should return true when set to true")
		}
	})

	t.Run("HasError", func(t *testing.T) {
		cond := graph.HasError()
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when no error")
		}

		state.Error = errors.New("test error")
		if !cond(state) {
			t.Error("Should return true when error exists")
		}
	})

	t.Run("NodeCompleted", func(t *testing.T) {
		cond := graph.NodeCompleted("test_node")
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when node not completed")
		}

		state.SetNodeOutput("test_node", "result")
		if !cond(state) {
			t.Error("Should return true when node completed")
		}
	})

	t.Run("AllNodesCompleted", func(t *testing.T) {
		cond := graph.AllNodesCompleted("node1", "node2")
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when no nodes completed")
		}

		state.SetNodeOutput("node1", "result1")
		if cond(state) {
			t.Error("Should return false when only some nodes completed")
		}

		state.SetNodeOutput("node2", "result2")
		if !cond(state) {
			t.Error("Should return true when all nodes completed")
		}
	})

	t.Run("StateHasKey", func(t *testing.T) {
		cond := graph.StateHasKey("my_key")
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when key doesn't exist")
		}

		state.Set("my_key", "value")
		if !cond(state) {
			t.Error("Should return true when key exists")
		}
	})

	t.Run("StateKeyEquals", func(t *testing.T) {
		cond := graph.StateKeyEquals("status", "approved")
		state := graph.NewState()

		if cond(state) {
			t.Error("Should return false when key doesn't exist")
		}

		state.Set("status", "pending")
		if cond(state) {
			t.Error("Should return false when value doesn't match")
		}

		state.Set("status", "approved")
		if !cond(state) {
			t.Error("Should return true when value matches")
		}
	})

	t.Run("And combines conditions", func(t *testing.T) {
		cond1 := graph.StateHasKey("key1")
		cond2 := graph.StateHasKey("key2")
		combined := graph.And(cond1, cond2)

		state := graph.NewState()
		if combined(state) {
			t.Error("Should return false when neither condition met")
		}

		state.Set("key1", true)
		if combined(state) {
			t.Error("Should return false when only one condition met")
		}

		state.Set("key2", true)
		if !combined(state) {
			t.Error("Should return true when all conditions met")
		}
	})

	t.Run("Or combines conditions", func(t *testing.T) {
		cond1 := graph.StateHasKey("key1")
		cond2 := graph.StateHasKey("key2")
		combined := graph.Or(cond1, cond2)

		state := graph.NewState()
		if combined(state) {
			t.Error("Should return false when no condition met")
		}

		state.Set("key1", true)
		if !combined(state) {
			t.Error("Should return true when one condition met")
		}
	})

	t.Run("Not negates condition", func(t *testing.T) {
		cond := graph.Not(graph.StateHasKey("key"))
		state := graph.NewState()

		if !cond(state) {
			t.Error("Should return true when key doesn't exist")
		}

		state.Set("key", true)
		if cond(state) {
			t.Error("Should return false when key exists")
		}
	})
}

// TestGraph tests the Graph struct
func TestGraph(t *testing.T) {
	t.Run("NewGraph creates empty graph", func(t *testing.T) {
		g := graph.NewGraph("test")
		if g.Name() != "test" {
			t.Errorf("Expected name 'test', got '%s'", g.Name())
		}
	})

	t.Run("Graph compilation validates entry point", func(t *testing.T) {
		g := graph.NewGraph("test")
		err := g.Compile()
		if err == nil {
			t.Error("Should fail without entry point")
		}
	})

	t.Run("Graph compilation validates node existence", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.SetEntryPoint("nonexistent")
		err := g.Compile()
		if err == nil {
			t.Error("Should fail when entry point node doesn't exist")
		}
	})

	t.Run("Valid graph compiles successfully", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.AddFunctionNode("start", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.AddFunctionNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.SetEntryPoint("start")
		g.SetEndNode("end")
		g.AddEdge("start", "end")

		err := g.Compile()
		if err != nil {
			t.Errorf("Valid graph should compile: %v", err)
		}
	})

	t.Run("GetNextNodes returns correct nodes", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.AddFunctionNode("a", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.AddFunctionNode("b", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.AddFunctionNode("c", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.SetEntryPoint("a")
		g.SetEndNode("c")
		g.AddEdge("a", "b")
		g.AddEdge("b", "c")
		g.Compile()

		state := graph.NewState()
		next := g.GetNextNodes("a", state)
		if len(next) != 1 || next[0] != "b" {
			t.Errorf("Expected [b], got %v", next)
		}

		// End node should return empty
		next = g.GetNextNodes("c", state)
		if len(next) != 0 {
			t.Errorf("End node should return empty, got %v", next)
		}
	})
}

// TestExecutor tests the graph executor
func TestExecutor(t *testing.T) {
	t.Run("Execute runs graph to completion", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.AddFunctionNode("start", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step1", true)
			return state, nil
		})
		g.AddFunctionNode("middle", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step2", true)
			return state, nil
		})
		g.AddFunctionNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("step3", true)
			return state, nil
		})
		g.SetEntryPoint("start")
		g.SetEndNode("end")
		g.AddEdge("start", "middle")
		g.AddEdge("middle", "end")
		g.Compile()

		executor := graph.NewExecutor(g)
		result, err := executor.Execute(context.Background(), map[string]interface{}{"initial": true})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if result.Status != graph.StatusCompleted {
			t.Errorf("Expected status Completed, got %v", result.Status)
		}
		if !result.GetBool("step1") || !result.GetBool("step2") || !result.GetBool("step3") {
			t.Error("Not all steps executed")
		}
	})

	t.Run("Execute respects context cancellation", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.AddFunctionNode("slow", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			time.Sleep(1 * time.Second)
			return state, nil
		})
		g.AddFunctionNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.SetEntryPoint("slow")
		g.SetEndNode("end")
		g.AddEdge("slow", "end")
		g.Compile()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		executor := graph.NewExecutor(g)
		_, err := executor.Execute(ctx, nil)
		if err == nil {
			t.Error("Should fail on context cancellation")
		}
	})

	t.Run("Execute handles node errors", func(t *testing.T) {
		expectedErr := errors.New("node failed")
		g := graph.NewGraph("test")
		g.AddFunctionNode("failing", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, expectedErr
		})
		g.AddFunctionNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.SetEntryPoint("failing")
		g.SetEndNode("end")
		g.AddEdge("failing", "end")
		g.Compile()

		executor := graph.NewExecutor(g)
		result, err := executor.Execute(context.Background(), nil)
		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
		if result.Status != graph.StatusFailed {
			t.Errorf("Expected status Failed, got %v", result.Status)
		}
	})

	t.Run("Execute detects infinite loops", func(t *testing.T) {
		g := graph.NewGraph("test")
		g.AddFunctionNode("loop", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		g.SetEntryPoint("loop")
		g.SetEndNode("loop") // This creates a loop as we never actually reach end
		g.AddEdge("loop", "loop")
		g.Compile()

		config := graph.DefaultGraphConfig()
		config.Recursion.LoopThreshold = 3
		executor := graph.NewExecutorWithConfig(g, config)
		_, err := executor.Execute(context.Background(), nil)
		if err == nil {
			t.Error("Should detect infinite loop")
		}
	})
}

// TestStateGraph tests the StateGraph builder
func TestStateGraph(t *testing.T) {
	t.Run("StateGraph builds valid graph", func(t *testing.T) {
		sg := graph.NewStateGraph("test")
		sg.AddNode("start", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("started", true)
			return state, nil
		})
		sg.AddNode("end", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("ended", true)
			return state, nil
		})
		sg.SetEntryPoint("start")
		sg.SetFinishPoint("end")
		sg.AddEdge("start", "end")

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Compile failed: %v", err)
		}

		result, err := compiled.Invoke(context.Background(), nil)
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}

		if !result.GetBool("started") || !result.GetBool("ended") {
			t.Error("Graph did not execute correctly")
		}
	})

	t.Run("StateGraph supports conditional edges", func(t *testing.T) {
		sg := graph.NewStateGraph("test")
		sg.AddNode("router", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			return state, nil
		})
		sg.AddNode("path_a", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("path", "a")
			return state, nil
		})
		sg.AddNode("path_b", func(ctx context.Context, state *graph.State) (*graph.State, error) {
			state.Set("path", "b")
			return state, nil
		})
		sg.SetEntryPoint("router")
		sg.SetFinishPoint("path_a")
		sg.SetFinishPoint("path_b")
		sg.AddConditionalEdges("router",
			func(state *graph.State) string {
				if state.GetBool("go_a") {
					return "a"
				}
				return "b"
			},
			map[string]string{
				"a": "path_a",
				"b": "path_b",
			},
		)

		compiled, err := sg.Compile()
		if err != nil {
			t.Fatalf("Compile failed: %v", err)
		}

		// Test path A
		result, err := compiled.Invoke(context.Background(), map[string]interface{}{"go_a": true})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if result.GetString("path") != "a" {
			t.Errorf("Expected path 'a', got '%s'", result.GetString("path"))
		}

		// Test path B
		result, err = compiled.Invoke(context.Background(), map[string]interface{}{"go_a": false})
		if err != nil {
			t.Fatalf("Invoke failed: %v", err)
		}
		if result.GetString("path") != "b" {
			t.Errorf("Expected path 'b', got '%s'", result.GetString("path"))
		}
	})
}

// TestInMemoryCheckpointStore tests checkpoint persistence
func TestInMemoryCheckpointStore(t *testing.T) {
	store := graph.NewInMemoryCheckpointStore()
	ctx := context.Background()

	t.Run("Save and Load checkpoint", func(t *testing.T) {
		checkpoint := &graph.Checkpoint{
			ID:       "cp1",
			ThreadID: "thread1",
			NodeName: "test_node",
			Data:     map[string]interface{}{"key": "value"},
		}

		err := store.Save(ctx, checkpoint)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := store.Load(ctx, "cp1")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if loaded.NodeName != "test_node" {
			t.Error("Loaded checkpoint mismatch")
		}
	})

	t.Run("List checkpoints by thread", func(t *testing.T) {
		store.Save(ctx, &graph.Checkpoint{ID: "cp2", ThreadID: "thread1"})
		store.Save(ctx, &graph.Checkpoint{ID: "cp3", ThreadID: "thread2"})

		list, err := store.List(ctx, "thread1")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Should have cp1 and cp2 from thread1
		if len(list) < 2 {
			t.Errorf("Expected at least 2 checkpoints for thread1, got %d", len(list))
		}
	})

	t.Run("Delete checkpoint", func(t *testing.T) {
		err := store.Delete(ctx, "cp1")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.Load(ctx, "cp1")
		if err == nil {
			t.Error("Should fail to load deleted checkpoint")
		}
	})
}

// TestRouterFunctions tests built-in router functions
func TestRouterFunctions(t *testing.T) {
	t.Run("RiskLevelRouter routes by risk score", func(t *testing.T) {
		router := graph.RiskLevelRouter()

		tests := []struct {
			risk     float64
			expected string
		}{
			{0.95, "critical_path"},
			{0.8, "high_risk_path"},
			{0.6, "medium_risk_path"},
			{0.3, "low_risk_path"},
		}

		for _, tc := range tests {
			state := graph.NewState()
			state.Set("risk_score", tc.risk)
			result := router(state)
			if result != tc.expected {
				t.Errorf("Risk %.2f: expected %s, got %s", tc.risk, tc.expected, result)
			}
		}
	})

	t.Run("DecisionRouter routes by decision", func(t *testing.T) {
		router := graph.DecisionRouter()

		tests := []struct {
			decision string
			expected string
		}{
			{"approve", "approve_path"},
			{"deny", "deny_path"},
			{"escalate", "escalate_path"},
			{"review", "human_review"},
			{"unknown", "default_path"},
		}

		for _, tc := range tests {
			state := graph.NewState()
			state.Set("decision", tc.decision)
			result := router(state)
			if result != tc.expected {
				t.Errorf("Decision %s: expected %s, got %s", tc.decision, tc.expected, result)
			}
		}
	})
}

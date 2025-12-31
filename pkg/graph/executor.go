// Package graph provides graph execution capabilities
package graph

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Executor handles graph execution
type Executor struct {
	graph           *Graph
	config          *GraphConfig
	checkpointStore CheckpointStore
}

// NewExecutor creates a new graph executor
func NewExecutor(graph *Graph) *Executor {
	return &Executor{
		graph:           graph,
		config:          DefaultGraphConfig(),
		checkpointStore: NewInMemoryCheckpointStore(),
	}
}

// NewExecutorWithConfig creates an executor with custom config
func NewExecutorWithConfig(graph *Graph, config *GraphConfig) *Executor {
	e := &Executor{
		graph:  graph,
		config: config,
	}
	if config.CheckpointStore != nil {
		e.checkpointStore = config.CheckpointStore
	} else {
		e.checkpointStore = NewInMemoryCheckpointStore()
	}
	return e
}

// Execute runs the graph to completion
func (e *Executor) Execute(ctx context.Context, input map[string]interface{}) (*State, error) {
	// Initialize state
	state := NewState()
	for k, v := range input {
		state.Set(k, v)
	}

	// Set entry point
	state.CurrentNode = e.graph.EntryPoint()

	// Execute until complete or error
	return e.run(ctx, state)
}

// Stream executes the graph and streams state updates
func (e *Executor) Stream(ctx context.Context, input map[string]interface{}) (<-chan *State, <-chan error) {
	stateChan := make(chan *State, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(stateChan)
		defer close(errChan)

		state := NewState()
		for k, v := range input {
			state.Set(k, v)
		}
		state.CurrentNode = e.graph.EntryPoint()

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			// Send current state
			stateCopy := state.Clone()
			stateChan <- stateCopy

			// Check if at end
			if e.graph.IsEndNode(state.CurrentNode) {
				state.Status = StatusCompleted
				stateChan <- state
				return
			}

			// Execute current node
			newState, err := e.executeNode(ctx, state)
			if err != nil {
				errChan <- err
				return
			}
			state = newState

			// Get next node
			nextNodes := e.graph.GetNextNodes(state.CurrentNode, state)
			if len(nextNodes) == 0 {
				state.Status = StatusCompleted
				stateChan <- state
				return
			}

			// Transition to next node
			state.CurrentNode = nextNodes[0]
		}
	}()

	return stateChan, errChan
}

// ResumeFromCheckpoint resumes execution from a saved checkpoint
func (e *Executor) ResumeFromCheckpoint(ctx context.Context, checkpoint *Checkpoint) (*State, error) {
	state := &State{
		ID:           checkpoint.StateID,
		CurrentNode:  checkpoint.NodeName,
		Data:         checkpoint.Data,
		NodeOutputs:  checkpoint.NodeOutputs,
		Messages:     checkpoint.Messages,
		Status:       StatusRunning,
		Checkpoint:   checkpoint,
		mu:           sync.RWMutex{},
	}

	return e.run(ctx, state)
}

// run executes the graph from current state
func (e *Executor) run(ctx context.Context, state *State) (*State, error) {
	state.Status = StatusRunning
	iterations := 0
	nodeVisits := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			state.Status = StatusFailed
			state.Error = ctx.Err()
			return state, ctx.Err()
		default:
		}

		// Check iteration limit
		iterations++
		if iterations > e.config.MaxIterations {
			err := errors.New("max iterations exceeded")
			state.Status = StatusFailed
			state.Error = err
			return state, err
		}

		// Track node visits for loop detection
		nodeVisits[state.CurrentNode]++
		if e.config.Recursion.DetectLoops && nodeVisits[state.CurrentNode] > e.config.Recursion.LoopThreshold {
			err := fmt.Errorf("potential infinite loop detected at node '%s'", state.CurrentNode)
			state.Status = StatusFailed
			state.Error = err
			return state, err
		}

		// Check if at end node
		if e.graph.IsEndNode(state.CurrentNode) {
			state.Status = StatusCompleted
			return state, nil
		}

		// Execute callback
		if e.graph.onNodeStart != nil {
			e.graph.onNodeStart(state.CurrentNode, state)
		}

		// Execute current node
		prevNode := state.CurrentNode
		newState, err := e.executeNode(ctx, state)

		// Execute callback
		if e.graph.onNodeEnd != nil {
			e.graph.onNodeEnd(prevNode, newState, err)
		}

		if err != nil {
			// Check if it's a human interrupt
			if errors.Is(err, ErrHumanInterrupt) {
				newState.Status = StatusWaitingHuman
				// Save checkpoint for resumption
				if e.config.EnableCheckpoints {
					checkpoint := newState.CreateCheckpoint()
					if saveErr := e.checkpointStore.Save(ctx, checkpoint); saveErr != nil {
						return newState, fmt.Errorf("failed to save checkpoint: %w", saveErr)
					}
				}
				return newState, nil
			}
			newState.Status = StatusFailed
			newState.Error = err
			return newState, err
		}

		state = newState

		// Get next nodes
		nextNodes := e.graph.GetNextNodes(state.CurrentNode, state)
		if len(nextNodes) == 0 {
			// No more nodes, execution complete
			state.Status = StatusCompleted
			return state, nil
		}

		// Handle multiple next nodes (parallel paths)
		if len(nextNodes) > 1 {
			// Execute parallel paths
			parallelState, err := e.executeParallel(ctx, state, nextNodes)
			if err != nil {
				return parallelState, err
			}
			state = parallelState
			continue
		}

		// Single next node
		nextNode := nextNodes[0]

		// Execute transition callback
		if e.graph.onTransition != nil {
			e.graph.onTransition(state.CurrentNode, nextNode, state)
		}

		// Record transition
		state.RecordTransition(state.CurrentNode, nextNode)

		// Move to next node
		state.PreviousNode = state.CurrentNode
		state.CurrentNode = nextNode
	}
}

// executeNode executes a single node
func (e *Executor) executeNode(ctx context.Context, state *State) (*State, error) {
	node, ok := e.graph.GetNode(state.CurrentNode)
	if !ok {
		return state, fmt.Errorf("node '%s' not found", state.CurrentNode)
	}

	return node.Execute(ctx, state)
}

// executeParallel executes multiple nodes in parallel
func (e *Executor) executeParallel(ctx context.Context, state *State, nodeNames []string) (*State, error) {
	var wg sync.WaitGroup
	results := make(chan *parallelResult, len(nodeNames))

	// Execute each path in a goroutine
	for _, nodeName := range nodeNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Clone state for this branch
			branchState := state.Clone()
			branchState.CurrentNode = name

			// Execute the node
			resultState, err := e.executeNode(ctx, branchState)
			results <- &parallelResult{
				nodeName: name,
				state:    resultState,
				err:      err,
			}
		}(nodeName)
	}

	// Wait and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge results
	mergedState := state.Clone()
	var firstError error

	for result := range results {
		if result.err != nil && firstError == nil {
			firstError = result.err
			continue
		}

		// Merge node outputs
		for k, v := range result.state.NodeOutputs {
			mergedState.SetNodeOutput(k, v)
		}

		// Merge data (last write wins)
		for k, v := range result.state.Data {
			mergedState.Set(k, v)
		}

		// Merge messages
		mergedState.Messages = append(mergedState.Messages, result.state.Messages...)
	}

	return mergedState, firstError
}

type parallelResult struct {
	nodeName string
	state    *State
	err      error
}

// ExecutionContext provides context for node execution
type ExecutionContext struct {
	ctx       context.Context
	executor  *Executor
	state     *State
	startTime time.Time
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(ctx context.Context, executor *Executor, state *State) *ExecutionContext {
	return &ExecutionContext{
		ctx:       ctx,
		executor:  executor,
		state:     state,
		startTime: time.Now(),
	}
}

// Context returns the underlying context
func (ec *ExecutionContext) Context() context.Context {
	return ec.ctx
}

// State returns the current state
func (ec *ExecutionContext) State() *State {
	return ec.state
}

// Elapsed returns the elapsed time
func (ec *ExecutionContext) Elapsed() time.Duration {
	return time.Since(ec.startTime)
}

// GraphRunner provides a high-level interface for running graphs
type GraphRunner struct {
	graphs          map[string]*CompiledGraph
	checkpointStore CheckpointStore
	config          *GraphConfig
	mu              sync.RWMutex
}

// NewGraphRunner creates a new graph runner
func NewGraphRunner() *GraphRunner {
	return &GraphRunner{
		graphs:          make(map[string]*CompiledGraph),
		checkpointStore: NewInMemoryCheckpointStore(),
		config:          DefaultGraphConfig(),
	}
}

// RegisterGraph registers a compiled graph
func (r *GraphRunner) RegisterGraph(name string, graph *CompiledGraph) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.graphs[name] = graph
}

// Run executes a named graph
func (r *GraphRunner) Run(ctx context.Context, graphName string, input map[string]interface{}) (*State, error) {
	r.mu.RLock()
	graph, ok := r.graphs[graphName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("graph '%s' not found", graphName)
	}

	return graph.Invoke(ctx, input)
}

// RunWithThread executes a graph with a thread ID for state persistence
func (r *GraphRunner) RunWithThread(ctx context.Context, graphName, threadID string, input map[string]interface{}) (*State, error) {
	r.mu.RLock()
	graph, ok := r.graphs[graphName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("graph '%s' not found", graphName)
	}

	// Check for existing checkpoint
	checkpoints, err := r.checkpointStore.List(ctx, threadID)
	if err == nil && len(checkpoints) > 0 {
		// Resume from latest checkpoint
		latest := checkpoints[len(checkpoints)-1]
		return graph.InvokeWithCheckpoint(ctx, latest)
	}

	// Start fresh
	input["thread_id"] = threadID
	return graph.Invoke(ctx, input)
}

// CreateInvestigationGraph creates the fraud investigation graph
func CreateInvestigationGraph(agents map[string]AgentExecutor) (*CompiledGraph, error) {
	sg := NewStateGraph("fraud_investigation")

	// Add agent nodes
	for name, agent := range agents {
		sg.AddAgentNode(name, agent)
	}

	// Add orchestrator routing
	sg.AddNode("route_decision", func(ctx context.Context, state *State) (*State, error) {
		// Routing logic based on risk level
		riskScore := state.GetFloat("risk_score")
		state.Set("routed_at", time.Now())

		switch {
		case riskScore >= 0.9:
			state.Set("route", "critical")
		case riskScore >= 0.7:
			state.Set("route", "high")
		case riskScore >= 0.5:
			state.Set("route", "medium")
		default:
			state.Set("route", "low")
		}

		return state, nil
	})

	// Set entry point
	sg.SetEntryPoint("orchestrator")

	// Add edges for investigation flow
	sg.AddEdge("orchestrator", "data_enrichment")
	sg.AddEdge("data_enrichment", "entity_resolution")
	sg.AddEdge("entity_resolution", "pattern_analysis")
	sg.AddEdge("pattern_analysis", "network_analysis")
	sg.AddEdge("network_analysis", "temporal_analysis")
	sg.AddEdge("temporal_analysis", "document_analysis")
	sg.AddEdge("document_analysis", "policy_compliance")
	sg.AddEdge("policy_compliance", "risk_assessment")
	sg.AddEdge("risk_assessment", "route_decision")

	// Add conditional edges based on risk
	sg.AddConditionalEdges("route_decision",
		func(state *State) string {
			route := state.GetString("route")
			return route
		},
		map[string]string{
			"critical": "human_review",
			"high":     "evidence_compilation",
			"medium":   "evidence_compilation",
			"low":      "evidence_compilation",
		},
	)

	// Human review loop
	sg.AddEdge("human_review", "evidence_compilation")

	// Final steps
	sg.AddEdge("evidence_compilation", "explanation")
	sg.SetFinishPoint("explanation")

	return sg.Compile()
}

// GraphBuilder provides a fluent API for building graphs
type GraphBuilder struct {
	graph *Graph
	err   error
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder(name string) *GraphBuilder {
	return &GraphBuilder{
		graph: NewGraph(name),
	}
}

// Entry sets the entry point
func (b *GraphBuilder) Entry(name string) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.SetEntryPoint(name)
	return b
}

// End marks a node as an end point
func (b *GraphBuilder) End(name string) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.SetEndNode(name)
	return b
}

// Node adds a function node
func (b *GraphBuilder) Node(name string, fn NodeFunc) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.AddFunctionNode(name, fn)
	return b
}

// Agent adds an agent node
func (b *GraphBuilder) Agent(name string, agent AgentExecutor) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.AddAgentNode(name, agent)
	return b
}

// Edge adds an edge between nodes
func (b *GraphBuilder) Edge(from, to string) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.AddEdge(from, to)
	return b
}

// ConditionalEdge adds a conditional edge
func (b *GraphBuilder) ConditionalEdge(from, to string, condition EdgeCondition) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.AddConditionalEdge(from, to, condition)
	return b
}

// Router adds a router function
func (b *GraphBuilder) Router(from string, fn RouterFunc) *GraphBuilder {
	if b.err != nil {
		return b
	}
	b.graph.AddRouterFunc(from, fn)
	return b
}

// Build compiles and returns the graph
func (b *GraphBuilder) Build() (*CompiledGraph, error) {
	if b.err != nil {
		return nil, b.err
	}
	return &CompiledGraph{graph: b.graph}, b.graph.Compile()
}

// Quick helper to create a simple linear graph
func LinearGraph(name string, nodeNames ...string) (*CompiledGraph, error) {
	if len(nodeNames) == 0 {
		return nil, errors.New("at least one node required")
	}

	builder := NewGraphBuilder(name)
	builder.Entry(nodeNames[0])
	builder.End(nodeNames[len(nodeNames)-1])

	// Add placeholder nodes and edges
	for i, nodeName := range nodeNames {
		builder.Node(nodeName, func(ctx context.Context, state *State) (*State, error) {
			return state, nil
		})
		if i > 0 {
			builder.Edge(nodeNames[i-1], nodeName)
		}
	}

	return builder.Build()
}

// MessageBus for inter-node communication
type MessageBus struct {
	subscribers map[string][]chan Message
	mu          sync.RWMutex
}

// NewMessageBus creates a new message bus
func NewMessageBus() *MessageBus {
	return &MessageBus{
		subscribers: make(map[string][]chan Message),
	}
}

// Subscribe subscribes to messages for a topic
func (mb *MessageBus) Subscribe(topic string) <-chan Message {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	ch := make(chan Message, 10)
	mb.subscribers[topic] = append(mb.subscribers[topic], ch)
	return ch
}

// Publish publishes a message to a topic
func (mb *MessageBus) Publish(topic string, msg Message) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	for _, ch := range mb.subscribers[topic] {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
}

// Close closes all subscriber channels
func (mb *MessageBus) Close() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for _, channels := range mb.subscribers {
		for _, ch := range channels {
			close(ch)
		}
	}
	mb.subscribers = make(map[string][]chan Message)
}

// GenerateID generates a unique ID
func GenerateID() string {
	return uuid.New().String()
}

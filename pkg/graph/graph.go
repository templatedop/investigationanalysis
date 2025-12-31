// Package graph provides LangGraph-style graph execution
package graph

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Graph represents a state graph for agent execution
type Graph struct {
	name       string
	nodes      map[string]Node
	router     *Router
	entryPoint string
	endNodes   map[string]bool
	compiled   bool
	mu         sync.RWMutex

	// Callbacks
	onNodeStart  func(nodeName string, state *State)
	onNodeEnd    func(nodeName string, state *State, err error)
	onTransition func(from, to string, state *State)
}

// NewGraph creates a new graph with the given name
func NewGraph(name string) *Graph {
	return &Graph{
		name:     name,
		nodes:    make(map[string]Node),
		router:   NewRouter(),
		endNodes: make(map[string]bool),
	}
}

// Name returns the graph name
func (g *Graph) Name() string {
	return g.name
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node Node) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.Name()] = node
	g.compiled = false
	return g
}

// AddAgentNode adds an agent as a node
func (g *Graph) AddAgentNode(name string, agent AgentExecutor) *Graph {
	return g.AddNode(NewAgentNode(name, agent))
}

// AddFunctionNode adds a function as a node
func (g *Graph) AddFunctionNode(name string, fn NodeFunc) *Graph {
	return g.AddNode(NewFunctionNode(name, fn))
}

// AddConditionNode adds a conditional branching node
func (g *Graph) AddConditionNode(name string, evaluator ConditionEvaluator) *Graph {
	return g.AddNode(NewConditionNode(name, evaluator))
}

// AddParallelNode adds a parallel execution node
func (g *Graph) AddParallelNode(name string, nodes ...Node) *Graph {
	return g.AddNode(NewParallelNode(name, nodes...))
}

// AddHumanNode adds a human-in-the-loop node
func (g *Graph) AddHumanNode(name string, handler HumanHandler) *Graph {
	return g.AddNode(NewHumanNode(name, handler))
}

// AddSubgraph adds another graph as a subgraph node
func (g *Graph) AddSubgraph(name string, subgraph *Graph) *Graph {
	return g.AddNode(NewSubgraphNode(name, subgraph))
}

// SetEntryPoint sets the starting node
func (g *Graph) SetEntryPoint(nodeName string) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.entryPoint = nodeName
	g.compiled = false
	return g
}

// SetEndNode marks a node as an end node
func (g *Graph) SetEndNode(nodeName string) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.endNodes[nodeName] = true
	g.compiled = false
	return g
}

// AddEdge adds an unconditional edge between nodes
func (g *Graph) AddEdge(from, to string) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.router.AddEdge(NewEdge(from, to))
	g.compiled = false
	return g
}

// AddConditionalEdge adds a conditional edge
func (g *Graph) AddConditionalEdge(from, to string, condition EdgeCondition) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.router.AddEdge(NewConditionalEdge(from, to, condition))
	g.compiled = false
	return g
}

// AddConditionalEdges adds multiple conditional edges from one node
func (g *Graph) AddConditionalEdges(ce *ConditionalEdges) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.router.AddConditionalEdges(ce)
	g.compiled = false
	return g
}

// AddRouterFunc adds a custom routing function
func (g *Graph) AddRouterFunc(from string, fn RouterFunc) *Graph {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.router.AddRouterFunc(from, fn)
	g.compiled = false
	return g
}

// OnNodeStart sets callback for node start
func (g *Graph) OnNodeStart(fn func(nodeName string, state *State)) *Graph {
	g.onNodeStart = fn
	return g
}

// OnNodeEnd sets callback for node end
func (g *Graph) OnNodeEnd(fn func(nodeName string, state *State, err error)) *Graph {
	g.onNodeEnd = fn
	return g
}

// OnTransition sets callback for state transitions
func (g *Graph) OnTransition(fn func(from, to string, state *State)) *Graph {
	g.onTransition = fn
	return g
}

// Compile validates and compiles the graph
func (g *Graph) Compile() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compiled {
		return nil
	}

	// Validate entry point
	if g.entryPoint == "" {
		return errors.New("graph has no entry point")
	}

	if _, ok := g.nodes[g.entryPoint]; !ok {
		return fmt.Errorf("entry point node '%s' not found", g.entryPoint)
	}

	// Validate all edge targets exist
	for from, edges := range g.router.edges {
		if _, ok := g.nodes[from]; !ok {
			return fmt.Errorf("edge source node '%s' not found", from)
		}
		for _, edge := range edges {
			if _, ok := g.nodes[edge.To]; !ok {
				return fmt.Errorf("edge target node '%s' not found", edge.To)
			}
		}
	}

	// Validate conditional edges
	for from, ce := range g.router.conditionalEdges {
		if _, ok := g.nodes[from]; !ok {
			return fmt.Errorf("conditional edge source '%s' not found", from)
		}
		for target := range ce.Conditions {
			if _, ok := g.nodes[target]; !ok {
				return fmt.Errorf("conditional edge target '%s' not found", target)
			}
		}
		if ce.Default != "" {
			if _, ok := g.nodes[ce.Default]; !ok {
				return fmt.Errorf("conditional edge default target '%s' not found", ce.Default)
			}
		}
	}

	// Check for at least one end node or path to end
	if len(g.endNodes) == 0 {
		return errors.New("graph has no end nodes defined")
	}

	g.compiled = true
	return nil
}

// GetNode returns a node by name
func (g *Graph) GetNode(name string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[name]
	return node, ok
}

// GetNextNodes returns the next nodes to execute from current node
func (g *Graph) GetNextNodes(from string, state *State) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check if this is an end node
	if g.endNodes[from] {
		return nil
	}

	return g.router.GetNextNodes(from, state)
}

// IsEndNode checks if a node is an end node
func (g *Graph) IsEndNode(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.endNodes[name]
}

// EntryPoint returns the entry point node name
func (g *Graph) EntryPoint() string {
	return g.entryPoint
}

// Nodes returns all node names
func (g *Graph) Nodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	return names
}

// StateGraph provides a builder for creating graphs
type StateGraph struct {
	graph *Graph
}

// NewStateGraph creates a new state graph builder
func NewStateGraph(name string) *StateGraph {
	return &StateGraph{
		graph: NewGraph(name),
	}
}

// AddNode adds a node to the graph
func (sg *StateGraph) AddNode(name string, fn NodeFunc) *StateGraph {
	sg.graph.AddFunctionNode(name, fn)
	return sg
}

// AddAgentNode adds an agent node
func (sg *StateGraph) AddAgentNode(name string, agent AgentExecutor) *StateGraph {
	sg.graph.AddAgentNode(name, agent)
	return sg
}

// SetEntryPoint sets the entry point
func (sg *StateGraph) SetEntryPoint(name string) *StateGraph {
	sg.graph.SetEntryPoint(name)
	return sg
}

// SetFinishPoint sets a node as an end node
func (sg *StateGraph) SetFinishPoint(name string) *StateGraph {
	sg.graph.SetEndNode(name)
	return sg
}

// AddEdge adds an edge
func (sg *StateGraph) AddEdge(from, to string) *StateGraph {
	sg.graph.AddEdge(from, to)
	return sg
}

// AddConditionalEdges adds conditional routing
func (sg *StateGraph) AddConditionalEdges(from string, router func(state *State) string, destinations map[string]string) *StateGraph {
	// Create a router function that maps router result to actual node names
	sg.graph.AddRouterFunc(from, func(state *State) string {
		result := router(state)
		if dest, ok := destinations[result]; ok {
			return dest
		}
		return result
	})
	return sg
}

// Compile compiles and returns the graph
func (sg *StateGraph) Compile() (*CompiledGraph, error) {
	if err := sg.graph.Compile(); err != nil {
		return nil, err
	}

	return &CompiledGraph{
		graph: sg.graph,
	}, nil
}

// CompiledGraph is a compiled, ready-to-execute graph
type CompiledGraph struct {
	graph *Graph
}

// Invoke executes the graph with the given input
func (cg *CompiledGraph) Invoke(ctx context.Context, input map[string]interface{}) (*State, error) {
	executor := NewExecutor(cg.graph)
	return executor.Execute(ctx, input)
}

// Stream executes the graph and streams state updates
func (cg *CompiledGraph) Stream(ctx context.Context, input map[string]interface{}) (<-chan *State, <-chan error) {
	executor := NewExecutor(cg.graph)
	return executor.Stream(ctx, input)
}

// InvokeWithCheckpoint resumes execution from a checkpoint
func (cg *CompiledGraph) InvokeWithCheckpoint(ctx context.Context, checkpoint *Checkpoint) (*State, error) {
	executor := NewExecutor(cg.graph)
	return executor.ResumeFromCheckpoint(ctx, checkpoint)
}

// Graph returns the underlying graph
func (cg *CompiledGraph) Graph() *Graph {
	return cg.graph
}

// GraphConfig holds configuration for graph execution
type GraphConfig struct {
	MaxIterations     int
	EnableCheckpoints bool
	CheckpointStore   CheckpointStore
	Recursion         RecursionConfig
}

// RecursionConfig configures recursion limits
type RecursionConfig struct {
	MaxDepth      int
	DetectLoops   bool
	LoopThreshold int
}

// DefaultGraphConfig returns default configuration
func DefaultGraphConfig() *GraphConfig {
	return &GraphConfig{
		MaxIterations:     100,
		EnableCheckpoints: true,
		Recursion: RecursionConfig{
			MaxDepth:      10,
			DetectLoops:   true,
			LoopThreshold: 3,
		},
	}
}

// CheckpointStore interface for persisting checkpoints
type CheckpointStore interface {
	Save(ctx context.Context, checkpoint *Checkpoint) error
	Load(ctx context.Context, id string) (*Checkpoint, error)
	List(ctx context.Context, threadID string) ([]*Checkpoint, error)
	Delete(ctx context.Context, id string) error
}

// InMemoryCheckpointStore provides in-memory checkpoint storage
type InMemoryCheckpointStore struct {
	checkpoints map[string]*Checkpoint
	mu          sync.RWMutex
}

// NewInMemoryCheckpointStore creates a new in-memory store
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]*Checkpoint),
	}
}

// Save saves a checkpoint
func (s *InMemoryCheckpointStore) Save(ctx context.Context, checkpoint *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.checkpoints[checkpoint.ID] = checkpoint
	return nil
}

// Load loads a checkpoint by ID
func (s *InMemoryCheckpointStore) Load(ctx context.Context, id string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp, ok := s.checkpoints[id]
	if !ok {
		return nil, fmt.Errorf("checkpoint '%s' not found", id)
	}
	return cp, nil
}

// List lists all checkpoints for a thread
func (s *InMemoryCheckpointStore) List(ctx context.Context, threadID string) ([]*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Checkpoint
	for _, cp := range s.checkpoints {
		if cp.ThreadID == threadID {
			result = append(result, cp)
		}
	}
	return result, nil
}

// Delete deletes a checkpoint
func (s *InMemoryCheckpointStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.checkpoints, id)
	return nil
}

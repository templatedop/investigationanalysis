// Package graph provides node definitions for graph execution
package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/agents"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
)

// Node represents a node in the graph
type Node interface {
	// Name returns the node's unique name
	Name() string

	// Execute runs the node's logic
	Execute(ctx context.Context, state *State) (*State, error)

	// Type returns the node type
	Type() NodeType
}

// NodeType defines the type of node
type NodeType string

const (
	NodeTypeAgent     NodeType = "agent"
	NodeTypeTool      NodeType = "tool"
	NodeTypeCondition NodeType = "condition"
	NodeTypeHuman     NodeType = "human"
	NodeTypeStart     NodeType = "start"
	NodeTypeEnd       NodeType = "end"
	NodeTypeParallel  NodeType = "parallel"
	NodeTypeSubgraph  NodeType = "subgraph"
)

// Special node names
const (
	StartNodeName = "__start__"
	EndNodeName   = "__end__"
)

// BaseNode provides common node functionality
type BaseNode struct {
	name     string
	nodeType NodeType
	metadata map[string]interface{}
}

// NewBaseNode creates a new base node
func NewBaseNode(name string, nodeType NodeType) *BaseNode {
	return &BaseNode{
		name:     name,
		nodeType: nodeType,
		metadata: make(map[string]interface{}),
	}
}

// Name returns the node name
func (n *BaseNode) Name() string {
	return n.name
}

// Type returns the node type
func (n *BaseNode) Type() NodeType {
	return n.nodeType
}

// SetMetadata sets node metadata
func (n *BaseNode) SetMetadata(key string, value interface{}) {
	n.metadata[key] = value
}

// GetMetadata gets node metadata
func (n *BaseNode) GetMetadata(key string) (interface{}, bool) {
	val, ok := n.metadata[key]
	return val, ok
}

// AgentNode wraps an agent as a graph node
type AgentNode struct {
	*BaseNode
	agent      agents.Agent
	inputKeys  []string
	outputKey  string
}

// NewAgentNode creates a new agent node
func NewAgentNode(name string, agent agents.Agent) *AgentNode {
	return &AgentNode{
		BaseNode:  NewBaseNode(name, NodeTypeAgent),
		agent:     agent,
		outputKey: name + "_output",
	}
}

// WithInputKeys sets the keys to read from state as input
func (n *AgentNode) WithInputKeys(keys ...string) *AgentNode {
	n.inputKeys = keys
	return n
}

// WithOutputKey sets the key to write output to state
func (n *AgentNode) WithOutputKey(key string) *AgentNode {
	n.outputKey = key
	return n
}

// Execute runs the agent
func (n *AgentNode) Execute(ctx context.Context, state *State) (*State, error) {
	// Build input from state
	input := n.buildInput(state)

	// Execute agent
	finding, err := n.agent.Execute(ctx, input)
	if err != nil {
		return state, fmt.Errorf("agent %s failed: %w", n.name, err)
	}

	// Store output in state
	state.SetNodeOutput(n.name, finding)
	state.Set(n.outputKey, finding)

	// Add message for tracking
	state.AddMessage(MessageTypeAI, n.name, fmt.Sprintf("Agent %s completed with confidence %.2f", n.name, finding.Confidence))

	return state, nil
}

// buildInput builds agent input from state
func (n *AgentNode) buildInput(state *State) interface{} {
	// If specific input keys are defined, build a context from them
	if len(n.inputKeys) > 0 {
		input := make(map[string]interface{})
		for _, key := range n.inputKeys {
			if val, ok := state.Get(key); ok {
				input[key] = val
			}
		}
		return input
	}

	// Otherwise, try to get analysis context from state
	if ctx, ok := state.Get("analysis_context"); ok {
		return ctx
	}

	// Return full state data as fallback
	return state.Data
}

// FunctionNode executes a custom function
type FunctionNode struct {
	*BaseNode
	fn func(ctx context.Context, state *State) (*State, error)
}

// NewFunctionNode creates a new function node
func NewFunctionNode(name string, fn func(ctx context.Context, state *State) (*State, error)) *FunctionNode {
	return &FunctionNode{
		BaseNode: NewBaseNode(name, NodeTypeTool),
		fn:       fn,
	}
}

// Execute runs the function
func (n *FunctionNode) Execute(ctx context.Context, state *State) (*State, error) {
	return n.fn(ctx, state)
}

// ConditionNode evaluates a condition for routing
type ConditionNode struct {
	*BaseNode
	condition func(state *State) string // Returns the next node name
}

// NewConditionNode creates a new condition node
func NewConditionNode(name string, condition func(state *State) string) *ConditionNode {
	return &ConditionNode{
		BaseNode:  NewBaseNode(name, NodeTypeCondition),
		condition: condition,
	}
}

// Execute evaluates the condition
func (n *ConditionNode) Execute(ctx context.Context, state *State) (*State, error) {
	nextNode := n.condition(state)
	state.Set("__next_node__", nextNode)
	return state, nil
}

// Evaluate returns the next node based on state
func (n *ConditionNode) Evaluate(state *State) string {
	return n.condition(state)
}

// HumanNode waits for human input
type HumanNode struct {
	*BaseNode
	prompt       string
	timeout      time.Duration
	inputHandler func(state *State) (interface{}, error)
}

// NewHumanNode creates a new human interaction node
func NewHumanNode(name, prompt string, timeout time.Duration) *HumanNode {
	return &HumanNode{
		BaseNode: NewBaseNode(name, NodeTypeHuman),
		prompt:   prompt,
		timeout:  timeout,
	}
}

// WithInputHandler sets a custom input handler
func (n *HumanNode) WithInputHandler(handler func(state *State) (interface{}, error)) *HumanNode {
	n.inputHandler = handler
	return n
}

// Execute waits for human input
func (n *HumanNode) Execute(ctx context.Context, state *State) (*State, error) {
	// Mark state as waiting for human input
	state.Status = StatusPaused
	state.Set("__awaiting_human__", true)
	state.Set("__human_prompt__", n.prompt)

	// If there's a handler, use it
	if n.inputHandler != nil {
		input, err := n.inputHandler(state)
		if err != nil {
			return state, err
		}
		state.Set("human_input", input)
		state.AddMessage(MessageTypeHuman, "human", fmt.Sprintf("%v", input))
	}

	return state, nil
}

// ParallelNode executes multiple nodes in parallel
type ParallelNode struct {
	*BaseNode
	nodes []Node
}

// NewParallelNode creates a new parallel execution node
func NewParallelNode(name string, nodes ...Node) *ParallelNode {
	return &ParallelNode{
		BaseNode: NewBaseNode(name, NodeTypeParallel),
		nodes:    nodes,
	}
}

// Execute runs all nodes in parallel
func (n *ParallelNode) Execute(ctx context.Context, state *State) (*State, error) {
	type result struct {
		name   string
		output interface{}
		err    error
	}

	results := make(chan result, len(n.nodes))

	// Execute each node in a goroutine
	for _, node := range n.nodes {
		go func(node Node) {
			stateCopy := state.Clone()
			_, err := node.Execute(ctx, stateCopy)

			var output interface{}
			if err == nil {
				output, _ = stateCopy.GetNodeOutput(node.Name())
			}

			results <- result{
				name:   node.Name(),
				output: output,
				err:    err,
			}
		}(node)
	}

	// Collect results
	var errors []error
	for i := 0; i < len(n.nodes); i++ {
		r := <-results
		if r.err != nil {
			errors = append(errors, r.err)
		} else {
			state.SetNodeOutput(r.name, r.output)
		}
	}

	if len(errors) > 0 {
		return state, fmt.Errorf("parallel execution had %d errors: %v", len(errors), errors)
	}

	return state, nil
}

// SubgraphNode executes a nested graph
type SubgraphNode struct {
	*BaseNode
	graph *Graph
}

// NewSubgraphNode creates a new subgraph node
func NewSubgraphNode(name string, graph *Graph) *SubgraphNode {
	return &SubgraphNode{
		BaseNode: NewBaseNode(name, NodeTypeSubgraph),
		graph:    graph,
	}
}

// Execute runs the subgraph
func (n *SubgraphNode) Execute(ctx context.Context, state *State) (*State, error) {
	// Create executor for subgraph
	executor := NewExecutor(n.graph, nil)

	// Run subgraph with current state
	result, err := executor.Execute(ctx, state)
	if err != nil {
		return state, fmt.Errorf("subgraph %s failed: %w", n.name, err)
	}

	// Merge subgraph output back into state
	for k, v := range result.NodeOutputs {
		state.SetNodeOutput(n.name+"_"+k, v)
	}

	return state, nil
}

// StartNode is the entry point of the graph
type StartNode struct {
	*BaseNode
}

// NewStartNode creates a new start node
func NewStartNode() *StartNode {
	return &StartNode{
		BaseNode: NewBaseNode(StartNodeName, NodeTypeStart),
	}
}

// Execute initializes the graph execution
func (n *StartNode) Execute(ctx context.Context, state *State) (*State, error) {
	state.Status = StatusRunning
	state.AddMessage(MessageTypeSystem, "system", "Graph execution started")
	return state, nil
}

// EndNode is the exit point of the graph
type EndNode struct {
	*BaseNode
}

// NewEndNode creates a new end node
func NewEndNode() *EndNode {
	return &EndNode{
		BaseNode: NewBaseNode(EndNodeName, NodeTypeEnd),
	}
}

// Execute finalizes the graph execution
func (n *EndNode) Execute(ctx context.Context, state *State) (*State, error) {
	state.Complete()
	state.AddMessage(MessageTypeSystem, "system", "Graph execution completed")
	return state, nil
}

// CreateAgentNodes creates agent nodes from the agent registry
func CreateAgentNodes(registry *agents.AgentRegistry) map[string]*AgentNode {
	nodes := make(map[string]*AgentNode)

	agentTypes := []models.AgentType{
		models.AgentTypeDataEnrichment,
		models.AgentTypeEntityResolution,
		models.AgentTypePatternAnalysis,
		models.AgentTypeNetworkAnalysis,
		models.AgentTypeTemporalAnalysis,
		models.AgentTypeDocumentAnalysis,
		models.AgentTypePolicyCompliance,
		models.AgentTypeRiskAssessment,
		models.AgentTypeEvidence,
		models.AgentTypeExplanation,
	}

	for _, agentType := range agentTypes {
		agent, ok := registry.Get(agentType)
		if ok {
			nodes[string(agentType)] = NewAgentNode(string(agentType), agent)
		}
	}

	return nodes
}

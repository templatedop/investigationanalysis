// Package graph provides edge definitions for graph routing
package graph

// Edge represents a connection between nodes
type Edge struct {
	From      string
	To        string
	Condition EdgeCondition
}

// EdgeCondition is a function that determines if an edge should be followed
type EdgeCondition func(state *State) bool

// AlwaysTrue is a condition that always returns true
func AlwaysTrue(state *State) bool {
	return true
}

// NewEdge creates a new unconditional edge
func NewEdge(from, to string) *Edge {
	return &Edge{
		From:      from,
		To:        to,
		Condition: AlwaysTrue,
	}
}

// NewConditionalEdge creates a new conditional edge
func NewConditionalEdge(from, to string, condition EdgeCondition) *Edge {
	return &Edge{
		From:      from,
		To:        to,
		Condition: condition,
	}
}

// ShouldFollow checks if this edge should be followed
func (e *Edge) ShouldFollow(state *State) bool {
	return e.Condition(state)
}

// ConditionalEdges maps conditions to target nodes
type ConditionalEdges struct {
	From       string
	Conditions map[string]EdgeCondition // target node -> condition
	Default    string                   // default target if no condition matches
}

// NewConditionalEdges creates a new conditional edge set
func NewConditionalEdges(from string) *ConditionalEdges {
	return &ConditionalEdges{
		From:       from,
		Conditions: make(map[string]EdgeCondition),
	}
}

// AddCondition adds a conditional route
func (ce *ConditionalEdges) AddCondition(target string, condition EdgeCondition) *ConditionalEdges {
	ce.Conditions[target] = condition
	return ce
}

// SetDefault sets the default route
func (ce *ConditionalEdges) SetDefault(target string) *ConditionalEdges {
	ce.Default = target
	return ce
}

// Evaluate determines which target to route to
func (ce *ConditionalEdges) Evaluate(state *State) string {
	for target, condition := range ce.Conditions {
		if condition(state) {
			return target
		}
	}
	return ce.Default
}

// Router handles routing decisions in the graph
type Router struct {
	edges           map[string][]*Edge
	conditionalEdges map[string]*ConditionalEdges
	routerFuncs     map[string]RouterFunc
}

// RouterFunc is a function that determines the next node
type RouterFunc func(state *State) string

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		edges:           make(map[string][]*Edge),
		conditionalEdges: make(map[string]*ConditionalEdges),
		routerFuncs:     make(map[string]RouterFunc),
	}
}

// AddEdge adds an edge to the router
func (r *Router) AddEdge(edge *Edge) {
	r.edges[edge.From] = append(r.edges[edge.From], edge)
}

// AddConditionalEdges adds conditional edges
func (r *Router) AddConditionalEdges(ce *ConditionalEdges) {
	r.conditionalEdges[ce.From] = ce
}

// AddRouterFunc adds a custom router function
func (r *Router) AddRouterFunc(from string, fn RouterFunc) {
	r.routerFuncs[from] = fn
}

// GetNextNodes returns the next nodes to execute
func (r *Router) GetNextNodes(from string, state *State) []string {
	var nextNodes []string

	// Check for router function first
	if fn, ok := r.routerFuncs[from]; ok {
		next := fn(state)
		if next != "" {
			return []string{next}
		}
	}

	// Check for conditional edges
	if ce, ok := r.conditionalEdges[from]; ok {
		next := ce.Evaluate(state)
		if next != "" {
			return []string{next}
		}
	}

	// Check regular edges
	if edges, ok := r.edges[from]; ok {
		for _, edge := range edges {
			if edge.ShouldFollow(state) {
				nextNodes = append(nextNodes, edge.To)
			}
		}
	}

	return nextNodes
}

// Common routing conditions

// RiskAboveThreshold returns a condition that checks if risk is above threshold
func RiskAboveThreshold(threshold float64) EdgeCondition {
	return func(state *State) bool {
		risk := state.GetFloat("risk_score")
		return risk > threshold
	}
}

// RiskBelowThreshold returns a condition that checks if risk is below threshold
func RiskBelowThreshold(threshold float64) EdgeCondition {
	return func(state *State) bool {
		risk := state.GetFloat("risk_score")
		return risk <= threshold
	}
}

// HasIndicators returns a condition that checks if risk indicators exist
func HasIndicators(minCount int) EdgeCondition {
	return func(state *State) bool {
		if indicators, ok := state.Get("risk_indicators"); ok {
			if indicatorList, ok := indicators.([]interface{}); ok {
				return len(indicatorList) >= minCount
			}
		}
		return false
	}
}

// RequiresHumanReview returns a condition for human review
func RequiresHumanReview() EdgeCondition {
	return func(state *State) bool {
		if val, ok := state.Get("requires_human_review"); ok {
			if b, ok := val.(bool); ok {
				return b
			}
		}
		return false
	}
}

// HasError returns a condition that checks for errors
func HasError() EdgeCondition {
	return func(state *State) bool {
		return state.Error != nil
	}
}

// NodeCompleted returns a condition that checks if a node completed
func NodeCompleted(nodeName string) EdgeCondition {
	return func(state *State) bool {
		_, ok := state.GetNodeOutput(nodeName)
		return ok
	}
}

// AllNodesCompleted returns a condition that checks if all nodes completed
func AllNodesCompleted(nodeNames ...string) EdgeCondition {
	return func(state *State) bool {
		for _, name := range nodeNames {
			if _, ok := state.GetNodeOutput(name); !ok {
				return false
			}
		}
		return true
	}
}

// StateHasKey returns a condition that checks if state has a key
func StateHasKey(key string) EdgeCondition {
	return func(state *State) bool {
		_, ok := state.Get(key)
		return ok
	}
}

// StateKeyEquals returns a condition that checks if state key equals value
func StateKeyEquals(key string, value interface{}) EdgeCondition {
	return func(state *State) bool {
		val, ok := state.Get(key)
		if !ok {
			return false
		}
		return val == value
	}
}

// And combines multiple conditions with AND logic
func And(conditions ...EdgeCondition) EdgeCondition {
	return func(state *State) bool {
		for _, c := range conditions {
			if !c(state) {
				return false
			}
		}
		return true
	}
}

// Or combines multiple conditions with OR logic
func Or(conditions ...EdgeCondition) EdgeCondition {
	return func(state *State) bool {
		for _, c := range conditions {
			if c(state) {
				return true
			}
		}
		return false
	}
}

// Not negates a condition
func Not(condition EdgeCondition) EdgeCondition {
	return func(state *State) bool {
		return !condition(state)
	}
}

// RiskLevelRouter creates a router based on risk levels
func RiskLevelRouter() RouterFunc {
	return func(state *State) string {
		risk := state.GetFloat("risk_score")

		switch {
		case risk >= 0.9:
			return "critical_path"
		case risk >= 0.7:
			return "high_risk_path"
		case risk >= 0.5:
			return "medium_risk_path"
		default:
			return "low_risk_path"
		}
	}
}

// DecisionRouter creates a router based on decision outcomes
func DecisionRouter() RouterFunc {
	return func(state *State) string {
		decision := state.GetString("decision")

		switch decision {
		case "approve":
			return "approve_path"
		case "deny":
			return "deny_path"
		case "escalate":
			return "escalate_path"
		case "review":
			return "human_review"
		default:
			return "default_path"
		}
	}
}

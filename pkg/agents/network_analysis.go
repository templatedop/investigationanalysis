// Package agents provides the network analysis agent
package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// NetworkAnalysisAgent maps relationships between entities to detect collusion
type NetworkAnalysisAgent struct {
	*BaseAgent
	graphDB GraphDatabase
}

// GraphDatabase interface for graph operations
type GraphDatabase interface {
	GetConnectedEntities(ctx context.Context, entityID string, depth int) ([]GraphNode, []GraphEdge, error)
	FindShortestPath(ctx context.Context, sourceID, targetID string) ([]GraphNode, error)
	DetectCommunities(ctx context.Context, entityIDs []string) ([]Community, error)
	CalculateCentrality(ctx context.Context, entityID string) (float64, error)
}

// GraphNode represents a node in the graph
type GraphNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	Properties map[string]interface{} `json:"properties"`
	RiskScore  float64                `json:"risk_score"`
}

// GraphEdge represents an edge in the graph
type GraphEdge struct {
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Weight     float64                `json:"weight"`
	Properties map[string]interface{} `json:"properties"`
}

// Community represents a detected community/cluster
type Community struct {
	ID       string   `json:"id"`
	Members  []string `json:"members"`
	Density  float64  `json:"density"`
	RiskScore float64 `json:"risk_score"`
}

// NewNetworkAnalysisAgent creates a new network analysis agent
func NewNetworkAnalysisAgent(llmClient *llm.Client, ragSystem *rag.System, graphDB GraphDatabase) *NetworkAnalysisAgent {
	return &NetworkAnalysisAgent{
		BaseAgent: NewBaseAgent(
			models.AgentTypeNetworkAnalysis,
			"Network Analysis Agent",
			"Maps relationships between entities to detect fraud rings and collusion",
			&BaseAgentConfig{
				LLMClient: llmClient,
				RAGSystem: ragSystem,
				ModelType: "reasoning",
			},
		),
		graphDB: graphDB,
	}
}

// Execute performs network analysis
func (a *NetworkAnalysisAgent) Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error) {
	analysisCtx, ok := input.(*AnalysisContext)
	if !ok {
		return nil, fmt.Errorf("expected *AnalysisContext, got %T", input)
	}

	startTime := time.Now()

	customerID := ""
	if analysisCtx.AlertInput != nil {
		customerID = analysisCtx.AlertInput.CustomerID
	}

	// Perform network analysis
	networkAnalysis := &NetworkAnalysis{}

	// Get entity network
	if a.graphDB != nil {
		nodes, edges, err := a.graphDB.GetConnectedEntities(ctx, customerID, 3)
		if err == nil {
			networkAnalysis.Nodes = nodes
			networkAnalysis.Edges = edges
		}

		// Detect communities
		communities, err := a.graphDB.DetectCommunities(ctx, []string{customerID})
		if err == nil {
			networkAnalysis.Communities = communities
		}

		// Calculate centrality
		centrality, err := a.graphDB.CalculateCentrality(ctx, customerID)
		if err == nil {
			networkAnalysis.Centrality = centrality
		}
	} else {
		// Use entity links from context if no graph DB
		networkAnalysis = a.buildNetworkFromContext(analysisCtx)
	}

	// Analyze network patterns
	patterns := a.analyzeNetworkPatterns(networkAnalysis)

	// Get RAG context for known fraud ring patterns
	_, sources, _ := a.GetRAGContext(ctx, []string{
		"fraud ring detection patterns",
		"network analysis fraud",
		"collusion detection",
	})

	// Generate indicators and evidence
	indicators, evidence := a.generateNetworkFindings(patterns, networkAnalysis)

	finding := a.CreateFinding(
		a.calculateNetworkConfidence(networkAnalysis),
		indicators,
		evidence,
		sources,
	)
	finding.Metadata["network_analysis"] = networkAnalysis
	finding.Metadata["network_patterns"] = patterns
	finding.ProcessingTime = time.Since(startTime)

	return finding, nil
}

// NetworkAnalysis contains network analysis results
type NetworkAnalysis struct {
	Nodes          []GraphNode   `json:"nodes"`
	Edges          []GraphEdge   `json:"edges"`
	Communities    []Community   `json:"communities"`
	Centrality     float64       `json:"centrality"`
	Density        float64       `json:"density"`
	ClusterCoeff   float64       `json:"clustering_coefficient"`
	SuspiciousLinks []SuspiciousLink `json:"suspicious_links"`
}

// SuspiciousLink represents a potentially fraudulent connection
type SuspiciousLink struct {
	SourceID      string   `json:"source_id"`
	TargetID      string   `json:"target_id"`
	Relationship  string   `json:"relationship"`
	SuspicionType string   `json:"suspicion_type"`
	Confidence    float64  `json:"confidence"`
	Evidence      []string `json:"evidence"`
}

// NetworkPattern represents a detected network pattern
type NetworkPattern struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	AffectedNodes []string `json:"affected_nodes"`
}

// buildNetworkFromContext builds a network from entity links
func (a *NetworkAnalysisAgent) buildNetworkFromContext(analysisCtx *AnalysisContext) *NetworkAnalysis {
	network := &NetworkAnalysis{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	if analysisCtx.EntityLinks == nil {
		return network
	}

	nodeMap := make(map[string]bool)

	// Build from direct matches
	for _, link := range analysisCtx.EntityLinks.DirectMatches {
		if !nodeMap[link.SourceID] {
			network.Nodes = append(network.Nodes, GraphNode{
				ID:   link.SourceID,
				Type: "entity",
			})
			nodeMap[link.SourceID] = true
		}
		if !nodeMap[link.TargetID] {
			network.Nodes = append(network.Nodes, GraphNode{
				ID:   link.TargetID,
				Type: "entity",
			})
			nodeMap[link.TargetID] = true
		}

		network.Edges = append(network.Edges, GraphEdge{
			Source: link.SourceID,
			Target: link.TargetID,
			Type:   link.LinkType,
			Weight: link.Confidence,
		})
	}

	// Build from transitive links
	for _, link := range analysisCtx.EntityLinks.TransitiveLinks {
		if !nodeMap[link.SourceID] {
			network.Nodes = append(network.Nodes, GraphNode{
				ID:   link.SourceID,
				Type: "entity",
			})
			nodeMap[link.SourceID] = true
		}
		if !nodeMap[link.TargetID] {
			network.Nodes = append(network.Nodes, GraphNode{
				ID:   link.TargetID,
				Type: "entity",
			})
			nodeMap[link.TargetID] = true
		}

		network.Edges = append(network.Edges, GraphEdge{
			Source: link.SourceID,
			Target: link.TargetID,
			Type:   "transitive",
			Weight: link.Confidence,
		})
	}

	// Calculate basic metrics
	if len(network.Nodes) > 1 {
		maxEdges := float64(len(network.Nodes) * (len(network.Nodes) - 1) / 2)
		network.Density = float64(len(network.Edges)) / maxEdges
	}

	return network
}

// analyzeNetworkPatterns identifies suspicious network patterns
func (a *NetworkAnalysisAgent) analyzeNetworkPatterns(network *NetworkAnalysis) []NetworkPattern {
	var patterns []NetworkPattern

	// Check for hub pattern (one entity connected to many)
	nodeConnections := make(map[string]int)
	for _, edge := range network.Edges {
		nodeConnections[edge.Source]++
		nodeConnections[edge.Target]++
	}

	for nodeID, connections := range nodeConnections {
		if connections > 5 {
			patterns = append(patterns, NetworkPattern{
				Type:        "hub_entity",
				Description: fmt.Sprintf("Entity %s is connected to %d other entities", nodeID, connections),
				Severity:    a.hubSeverity(connections),
				Confidence:  0.85,
				AffectedNodes: []string{nodeID},
			})
		}
	}

	// Check for clique pattern (group of fully connected entities)
	if network.Density > 0.7 && len(network.Nodes) > 3 {
		nodeIDs := make([]string, len(network.Nodes))
		for i, n := range network.Nodes {
			nodeIDs[i] = n.ID
		}
		patterns = append(patterns, NetworkPattern{
			Type:        "potential_fraud_ring",
			Description: "Highly connected group of entities detected",
			Severity:    "critical",
			Confidence:  0.9,
			AffectedNodes: nodeIDs,
		})
	}

	// Check for suspicious communities
	for _, community := range network.Communities {
		if community.RiskScore > 0.7 {
			patterns = append(patterns, NetworkPattern{
				Type:        "high_risk_community",
				Description: fmt.Sprintf("Community %s has high risk score", community.ID),
				Severity:    "high",
				Confidence:  community.RiskScore,
				AffectedNodes: community.Members,
			})
		}
	}

	// Check for star topology (possible fraudster hub)
	for _, edge := range network.Edges {
		sourceConns := nodeConnections[edge.Source]
		targetConns := nodeConnections[edge.Target]

		// If one node has many connections and the other has few
		if (sourceConns > 5 && targetConns == 1) || (targetConns > 5 && sourceConns == 1) {
			patterns = append(patterns, NetworkPattern{
				Type:        "star_topology",
				Description: "Central entity with peripheral connections",
				Severity:    "medium",
				Confidence:  0.75,
			})
			break
		}
	}

	return patterns
}

// hubSeverity determines severity based on connection count
func (a *NetworkAnalysisAgent) hubSeverity(connections int) string {
	if connections > 15 {
		return "critical"
	}
	if connections > 10 {
		return "high"
	}
	if connections > 5 {
		return "medium"
	}
	return "low"
}

// generateNetworkFindings creates indicators and evidence
func (a *NetworkAnalysisAgent) generateNetworkFindings(patterns []NetworkPattern, network *NetworkAnalysis) ([]models.RiskIndicator, []models.Evidence) {
	var indicators []models.RiskIndicator
	var evidence []models.Evidence

	for _, pattern := range patterns {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "NET001",
			Description: pattern.Description,
			Severity:    pattern.Severity,
			Score:       pattern.Confidence,
			Category:    "network",
		})

		evidence = append(evidence, models.Evidence{
			Type:        "network_pattern",
			Description: fmt.Sprintf("Pattern: %s - %s", pattern.Type, pattern.Description),
			Source:      "network_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  pattern.Confidence,
			Data: map[string]interface{}{
				"affected_nodes": pattern.AffectedNodes,
				"pattern_type":   pattern.Type,
			},
		})
	}

	// Add suspicious links as evidence
	for _, link := range network.SuspiciousLinks {
		evidence = append(evidence, models.Evidence{
			Type:        "suspicious_link",
			Description: fmt.Sprintf("Suspicious connection: %s", link.SuspicionType),
			Source:      "network_analysis_agent",
			Timestamp:   time.Now(),
			Confidence:  link.Confidence,
			Data: map[string]interface{}{
				"source": link.SourceID,
				"target": link.TargetID,
			},
		})
	}

	// Add high centrality as indicator
	if network.Centrality > 0.7 {
		indicators = append(indicators, models.RiskIndicator{
			Code:        "NET002",
			Description: "Entity has unusually high network centrality",
			Severity:    "high",
			Score:       network.Centrality,
			Category:    "network",
		})
	}

	return indicators, evidence
}

// calculateNetworkConfidence computes confidence in network analysis
func (a *NetworkAnalysisAgent) calculateNetworkConfidence(network *NetworkAnalysis) float64 {
	if len(network.Nodes) == 0 {
		return 0.9 // High confidence when isolated (no network)
	}

	// More connections = more confidence in findings
	baseConfidence := 0.7
	connectionBonus := float64(len(network.Edges)) * 0.02
	if connectionBonus > 0.25 {
		connectionBonus = 0.25
	}

	return baseConfidence + connectionBonus
}

// InMemoryGraphDB is a simple in-memory graph database
type InMemoryGraphDB struct {
	nodes map[string]*GraphNode
	edges []GraphEdge
}

// NewInMemoryGraphDB creates an in-memory graph database
func NewInMemoryGraphDB() *InMemoryGraphDB {
	return &InMemoryGraphDB{
		nodes: make(map[string]*GraphNode),
		edges: []GraphEdge{},
	}
}

// AddNode adds a node to the graph
func (g *InMemoryGraphDB) AddNode(node *GraphNode) {
	g.nodes[node.ID] = node
}

// AddEdge adds an edge to the graph
func (g *InMemoryGraphDB) AddEdge(edge GraphEdge) {
	g.edges = append(g.edges, edge)
}

// GetConnectedEntities retrieves connected entities
func (g *InMemoryGraphDB) GetConnectedEntities(ctx context.Context, entityID string, depth int) ([]GraphNode, []GraphEdge, error) {
	visited := make(map[string]bool)
	var nodes []GraphNode
	var edges []GraphEdge

	g.bfs(entityID, depth, visited, &nodes, &edges)

	return nodes, edges, nil
}

// bfs performs breadth-first search
func (g *InMemoryGraphDB) bfs(startID string, maxDepth int, visited map[string]bool, nodes *[]GraphNode, edges *[]GraphEdge) {
	if maxDepth < 0 || visited[startID] {
		return
	}

	visited[startID] = true
	if node, ok := g.nodes[startID]; ok {
		*nodes = append(*nodes, *node)
	}

	for _, edge := range g.edges {
		if edge.Source == startID && !visited[edge.Target] {
			*edges = append(*edges, edge)
			g.bfs(edge.Target, maxDepth-1, visited, nodes, edges)
		} else if edge.Target == startID && !visited[edge.Source] {
			*edges = append(*edges, edge)
			g.bfs(edge.Source, maxDepth-1, visited, nodes, edges)
		}
	}
}

// FindShortestPath finds the shortest path between two nodes
func (g *InMemoryGraphDB) FindShortestPath(ctx context.Context, sourceID, targetID string) ([]GraphNode, error) {
	// Simple BFS implementation
	visited := make(map[string]bool)
	parent := make(map[string]string)
	queue := []string{sourceID}
	visited[sourceID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == targetID {
			return g.reconstructPath(parent, sourceID, targetID), nil
		}

		for _, edge := range g.edges {
			var neighbor string
			if edge.Source == current {
				neighbor = edge.Target
			} else if edge.Target == current {
				neighbor = edge.Source
			} else {
				continue
			}

			if !visited[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}

	return nil, fmt.Errorf("no path found")
}

// reconstructPath builds the path from BFS results
func (g *InMemoryGraphDB) reconstructPath(parent map[string]string, source, target string) []GraphNode {
	var path []GraphNode
	current := target

	for current != source {
		if node, ok := g.nodes[current]; ok {
			path = append([]GraphNode{*node}, path...)
		}
		current = parent[current]
	}

	if node, ok := g.nodes[source]; ok {
		path = append([]GraphNode{*node}, path...)
	}

	return path
}

// DetectCommunities performs simple community detection
func (g *InMemoryGraphDB) DetectCommunities(ctx context.Context, entityIDs []string) ([]Community, error) {
	// Simple connected components as communities
	visited := make(map[string]bool)
	var communities []Community
	communityID := 0

	for id := range g.nodes {
		if !visited[id] {
			var members []string
			g.dfs(id, visited, &members)

			communities = append(communities, Community{
				ID:      fmt.Sprintf("community_%d", communityID),
				Members: members,
				Density: g.calculateCommunityDensity(members),
			})
			communityID++
		}
	}

	return communities, nil
}

// dfs performs depth-first search for community detection
func (g *InMemoryGraphDB) dfs(nodeID string, visited map[string]bool, members *[]string) {
	visited[nodeID] = true
	*members = append(*members, nodeID)

	for _, edge := range g.edges {
		if edge.Source == nodeID && !visited[edge.Target] {
			g.dfs(edge.Target, visited, members)
		} else if edge.Target == nodeID && !visited[edge.Source] {
			g.dfs(edge.Source, visited, members)
		}
	}
}

// calculateCommunityDensity calculates edge density of a community
func (g *InMemoryGraphDB) calculateCommunityDensity(members []string) float64 {
	if len(members) < 2 {
		return 0
	}

	memberSet := make(map[string]bool)
	for _, m := range members {
		memberSet[m] = true
	}

	edgeCount := 0
	for _, edge := range g.edges {
		if memberSet[edge.Source] && memberSet[edge.Target] {
			edgeCount++
		}
	}

	maxEdges := len(members) * (len(members) - 1) / 2
	return float64(edgeCount) / float64(maxEdges)
}

// CalculateCentrality calculates degree centrality
func (g *InMemoryGraphDB) CalculateCentrality(ctx context.Context, entityID string) (float64, error) {
	if len(g.nodes) <= 1 {
		return 0, nil
	}

	degree := 0
	for _, edge := range g.edges {
		if edge.Source == entityID || edge.Target == entityID {
			degree++
		}
	}

	return float64(degree) / float64(len(g.nodes)-1), nil
}

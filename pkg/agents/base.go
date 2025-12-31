// Package agents provides specialized agent implementations for fraud investigation
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fraudinvestigation/rag-framework/pkg/llm"
	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/rag"
)

// Agent defines the interface for all specialized agents
type Agent interface {
	// Type returns the agent type
	Type() models.AgentType

	// Execute runs the agent's analysis
	Execute(ctx context.Context, input interface{}) (*models.AgentFinding, error)

	// Name returns a human-readable name
	Name() string

	// Description returns what the agent does
	Description() string
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	agentType   models.AgentType
	name        string
	description string
	llmClient   *llm.Client
	ragSystem   *rag.System
	modelType   string // "fast", "medium", "reasoning"
}

// BaseAgentConfig configures a base agent
type BaseAgentConfig struct {
	LLMClient *llm.Client
	RAGSystem *rag.System
	ModelType string
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(agentType models.AgentType, name, description string, cfg *BaseAgentConfig) *BaseAgent {
	if cfg.ModelType == "" {
		cfg.ModelType = "medium"
	}

	return &BaseAgent{
		agentType:   agentType,
		name:        name,
		description: description,
		llmClient:   cfg.LLMClient,
		ragSystem:   cfg.RAGSystem,
		modelType:   cfg.ModelType,
	}
}

// Type returns the agent type
func (a *BaseAgent) Type() models.AgentType {
	return a.agentType
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Description returns the agent description
func (a *BaseAgent) Description() string {
	return a.description
}

// GetRAGContext retrieves relevant context from RAG
func (a *BaseAgent) GetRAGContext(ctx context.Context, queries []string) (string, []models.RAGSource, error) {
	builder := rag.NewContextBuilder(a.ragSystem, 6000)
	return builder.BuildAgentContext(ctx, a.agentType, queries)
}

// GenerateWithRAG generates a response using RAG context
func (a *BaseAgent) GenerateWithRAG(ctx context.Context, systemPrompt, userPrompt string, queries []string) (string, []models.RAGSource, error) {
	context, sources, err := a.GetRAGContext(ctx, queries)
	if err != nil {
		return "", nil, err
	}

	fullUserPrompt := fmt.Sprintf(`## Relevant Knowledge
%s

## Task
%s`, context, userPrompt)

	resp, err := a.llmClient.CompleteWithSystem(ctx, systemPrompt, fullUserPrompt)
	if err != nil {
		return "", nil, err
	}

	return resp, sources, nil
}

// CreateFinding creates an agent finding with common fields populated
func (a *BaseAgent) CreateFinding(confidence float64, indicators []models.RiskIndicator, evidence []models.Evidence, sources []models.RAGSource) *models.AgentFinding {
	return &models.AgentFinding{
		AgentType:      a.agentType,
		Confidence:     confidence,
		RiskIndicators: indicators,
		Evidence:       evidence,
		RAGSources:     sources,
		Metadata:       make(map[string]interface{}),
	}
}

// ParseJSONResponse parses a JSON response from the LLM
func (a *BaseAgent) ParseJSONResponse(response string, target interface{}) error {
	// Find JSON in response (handle markdown code blocks)
	start := 0
	end := len(response)

	if idx := findJSONStart(response); idx >= 0 {
		start = idx
	}
	if idx := findJSONEnd(response[start:]); idx >= 0 {
		end = start + idx + 1
	}

	jsonStr := response[start:end]
	return json.Unmarshal([]byte(jsonStr), target)
}

// findJSONStart finds the start of JSON in a string
func findJSONStart(s string) int {
	for i, c := range s {
		if c == '{' || c == '[' {
			return i
		}
	}
	return -1
}

// findJSONEnd finds the end of JSON in a string
func findJSONEnd(s string) int {
	depth := 0
	inString := false
	escaped := false

	for i, c := range s {
		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' || c == '[' {
			depth++
		} else if c == '}' || c == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// AnalysisContext contains common context for agent analysis
type AnalysisContext struct {
	AlertInput    *models.FraudAlertInput   `json:"alert_input"`
	EnrichedData  *models.EnrichedData      `json:"enriched_data,omitempty"`
	EntityLinks   *models.EntityLinks       `json:"entity_links,omitempty"`
	State         *models.InvestigationState `json:"state,omitempty"`
}

// ToJSON converts context to JSON string
func (c *AnalysisContext) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

// AgentRegistry maintains a registry of available agents
type AgentRegistry struct {
	agents map[models.AgentType]Agent
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[models.AgentType]Agent),
	}
}

// Register adds an agent to the registry
func (r *AgentRegistry) Register(agent Agent) {
	r.agents[agent.Type()] = agent
}

// Get retrieves an agent by type
func (r *AgentRegistry) Get(agentType models.AgentType) (Agent, bool) {
	agent, ok := r.agents[agentType]
	return agent, ok
}

// All returns all registered agents
func (r *AgentRegistry) All() []Agent {
	agents := make([]Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// AgentResult wraps an agent finding with execution metadata
type AgentResult struct {
	AgentType    models.AgentType   `json:"agent_type"`
	Finding      *models.AgentFinding `json:"finding"`
	Error        error              `json:"error,omitempty"`
	ExecutionTime time.Duration     `json:"execution_time"`
}

// AgentExecutor handles parallel agent execution
type AgentExecutor struct {
	registry *AgentRegistry
}

// NewAgentExecutor creates a new agent executor
func NewAgentExecutor(registry *AgentRegistry) *AgentExecutor {
	return &AgentExecutor{
		registry: registry,
	}
}

// ExecuteAgents runs multiple agents in parallel
func (e *AgentExecutor) ExecuteAgents(ctx context.Context, agentTypes []models.AgentType, input interface{}) map[models.AgentType]*AgentResult {
	results := make(map[models.AgentType]*AgentResult)
	resultChan := make(chan *AgentResult, len(agentTypes))

	for _, agentType := range agentTypes {
		go func(at models.AgentType) {
			result := &AgentResult{AgentType: at}
			startTime := time.Now()

			agent, ok := e.registry.Get(at)
			if !ok {
				result.Error = fmt.Errorf("agent not found: %s", at)
				resultChan <- result
				return
			}

			finding, err := agent.Execute(ctx, input)
			result.Finding = finding
			result.Error = err
			result.ExecutionTime = time.Since(startTime)

			resultChan <- result
		}(agentType)
	}

	for i := 0; i < len(agentTypes); i++ {
		result := <-resultChan
		results[result.AgentType] = result
	}

	return results
}

// PromptBuilder helps construct agent prompts
type PromptBuilder struct {
	systemPrompt strings.Builder
	userPrompt   strings.Builder
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// AddSystemInstruction adds a system instruction
func (pb *PromptBuilder) AddSystemInstruction(instruction string) *PromptBuilder {
	pb.systemPrompt.WriteString(instruction)
	pb.systemPrompt.WriteString("\n\n")
	return pb
}

// AddUserContext adds context to the user prompt
func (pb *PromptBuilder) AddUserContext(label, content string) *PromptBuilder {
	pb.userPrompt.WriteString("## ")
	pb.userPrompt.WriteString(label)
	pb.userPrompt.WriteString("\n")
	pb.userPrompt.WriteString(content)
	pb.userPrompt.WriteString("\n\n")
	return pb
}

// AddTask adds the main task
func (pb *PromptBuilder) AddTask(task string) *PromptBuilder {
	pb.userPrompt.WriteString("## Task\n")
	pb.userPrompt.WriteString(task)
	pb.userPrompt.WriteString("\n\n")
	return pb
}

// AddOutputFormat adds expected output format
func (pb *PromptBuilder) AddOutputFormat(format string) *PromptBuilder {
	pb.userPrompt.WriteString("## Expected Output Format\n")
	pb.userPrompt.WriteString(format)
	return pb
}

// Build returns the constructed prompts
func (pb *PromptBuilder) Build() (system, user string) {
	return pb.systemPrompt.String(), pb.userPrompt.String()
}

// strings is a simple builder for the prompt builder
type strings struct {
	Builder
}

type Builder struct {
	data []byte
}

func (b *Builder) WriteString(s string) {
	b.data = append(b.data, s...)
}

func (b *Builder) String() string {
	return string(b.data)
}

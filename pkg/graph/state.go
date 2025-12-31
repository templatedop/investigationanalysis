// Package graph provides LangGraph-style graph execution for agent orchestration
package graph

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// State represents the graph execution state
type State struct {
	mu sync.RWMutex

	// Unique identifier for this execution
	ID string `json:"id"`

	// Current node being executed
	CurrentNode string `json:"current_node"`

	// Previous node (for routing decisions)
	PreviousNode string `json:"previous_node"`

	// Execution history
	History []StateTransition `json:"history"`

	// Shared data between nodes
	Data map[string]interface{} `json:"data"`

	// Node-specific outputs
	NodeOutputs map[string]interface{} `json:"node_outputs"`

	// Error state
	Error error `json:"error,omitempty"`

	// Execution status
	Status ExecutionStatus `json:"status"`

	// Timestamps
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Checkpoint data for persistence
	Checkpoint *Checkpoint `json:"checkpoint,omitempty"`

	// Messages for agent communication (LangGraph-style)
	Messages []Message `json:"messages"`
}

// ExecutionStatus represents the current execution status
type ExecutionStatus string

const (
	StatusPending    ExecutionStatus = "pending"
	StatusRunning    ExecutionStatus = "running"
	StatusPaused     ExecutionStatus = "paused"
	StatusCompleted  ExecutionStatus = "completed"
	StatusFailed     ExecutionStatus = "failed"
	StatusInterrupted ExecutionStatus = "interrupted"
)

// StateTransition records a state transition
type StateTransition struct {
	FromNode  string                 `json:"from_node"`
	ToNode    string                 `json:"to_node"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Message represents a message in the graph (similar to LangGraph messages)
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	Content   string                 `json:"content"`
	Role      string                 `json:"role"`
	Name      string                 `json:"name,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageType defines the type of message
type MessageType string

const (
	MessageTypeHuman     MessageType = "human"
	MessageTypeAI        MessageType = "ai"
	MessageTypeSystem    MessageType = "system"
	MessageTypeTool      MessageType = "tool"
	MessageTypeFunction  MessageType = "function"
)

// Checkpoint contains data for state persistence
type Checkpoint struct {
	ID          string                 `json:"id"`
	GraphID     string                 `json:"graph_id"`
	ThreadID    string                 `json:"thread_id"`
	State       map[string]interface{} `json:"state"`
	NodeOutputs map[string]interface{} `json:"node_outputs"`
	Timestamp   time.Time              `json:"timestamp"`
	Version     int                    `json:"version"`
}

// NewState creates a new execution state
func NewState() *State {
	return &State{
		ID:          uuid.New().String(),
		Data:        make(map[string]interface{}),
		NodeOutputs: make(map[string]interface{}),
		History:     []StateTransition{},
		Messages:    []Message{},
		Status:      StatusPending,
		StartedAt:   time.Now(),
	}
}

// NewStateWithData creates a state with initial data
func NewStateWithData(data map[string]interface{}) *State {
	s := NewState()
	s.Data = data
	return s
}

// Get retrieves a value from state data
func (s *State) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Data[key]
	return val, ok
}

// Set stores a value in state data
func (s *State) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = value
}

// GetString retrieves a string value
func (s *State) GetString(key string) string {
	val, ok := s.Get(key)
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// GetFloat retrieves a float64 value
func (s *State) GetFloat(key string) float64 {
	val, ok := s.Get(key)
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

// GetNodeOutput retrieves output from a specific node
func (s *State) GetNodeOutput(nodeName string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.NodeOutputs[nodeName]
	return val, ok
}

// SetNodeOutput stores output from a node
func (s *State) SetNodeOutput(nodeName string, output interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NodeOutputs[nodeName] = output
}

// AddMessage adds a message to the state
func (s *State) AddMessage(msgType MessageType, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, Message{
		ID:        uuid.New().String(),
		Type:      msgType,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// GetMessages returns all messages
func (s *State) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := make([]Message, len(s.Messages))
	copy(msgs, s.Messages)
	return msgs
}

// RecordTransition records a node transition
func (s *State) RecordTransition(from, to string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, StateTransition{
		FromNode:  from,
		ToNode:    to,
		Timestamp: time.Now(),
		Duration:  duration,
	})
	s.PreviousNode = from
	s.CurrentNode = to
}

// CreateCheckpoint creates a checkpoint of current state
func (s *State) CreateCheckpoint(graphID, threadID string) *Checkpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy data
	dataCopy := make(map[string]interface{})
	for k, v := range s.Data {
		dataCopy[k] = v
	}

	outputsCopy := make(map[string]interface{})
	for k, v := range s.NodeOutputs {
		outputsCopy[k] = v
	}

	version := 1
	if s.Checkpoint != nil {
		version = s.Checkpoint.Version + 1
	}

	return &Checkpoint{
		ID:          uuid.New().String(),
		GraphID:     graphID,
		ThreadID:    threadID,
		State:       dataCopy,
		NodeOutputs: outputsCopy,
		Timestamp:   time.Now(),
		Version:     version,
	}
}

// RestoreFromCheckpoint restores state from a checkpoint
func (s *State) RestoreFromCheckpoint(cp *Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Data = cp.State
	s.NodeOutputs = cp.NodeOutputs
	s.Checkpoint = cp
	s.Status = StatusPaused
}

// Complete marks the state as completed
func (s *State) Complete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.CompletedAt = &now
	s.Status = StatusCompleted
}

// Fail marks the state as failed
func (s *State) Fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.CompletedAt = &now
	s.Status = StatusFailed
	s.Error = err
}

// Clone creates a deep copy of the state
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &State{
		ID:           s.ID,
		CurrentNode:  s.CurrentNode,
		PreviousNode: s.PreviousNode,
		Status:       s.Status,
		StartedAt:    s.StartedAt,
		CompletedAt:  s.CompletedAt,
		Error:        s.Error,
		Data:         make(map[string]interface{}),
		NodeOutputs:  make(map[string]interface{}),
		History:      make([]StateTransition, len(s.History)),
		Messages:     make([]Message, len(s.Messages)),
	}

	for k, v := range s.Data {
		clone.Data[k] = v
	}
	for k, v := range s.NodeOutputs {
		clone.NodeOutputs[k] = v
	}
	copy(clone.History, s.History)
	copy(clone.Messages, s.Messages)

	return clone
}

// ToJSON serializes state to JSON
func (s *State) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

// FromJSON deserializes state from JSON
func FromJSON(data []byte) (*State, error) {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// StateReducer defines how to merge state updates
type StateReducer func(current, update map[string]interface{}) map[string]interface{}

// DefaultReducer replaces values with updates
func DefaultReducer(current, update map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range current {
		result[k] = v
	}
	for k, v := range update {
		result[k] = v
	}
	return result
}

// AppendReducer appends to lists instead of replacing
func AppendReducer(current, update map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range current {
		result[k] = v
	}
	for k, v := range update {
		if currentVal, exists := result[k]; exists {
			// Try to append if both are slices
			if currentSlice, ok := currentVal.([]interface{}); ok {
				if updateSlice, ok := v.([]interface{}); ok {
					result[k] = append(currentSlice, updateSlice...)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
}

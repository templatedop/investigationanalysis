// Package llm provides LLM client implementations for on-premise models
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider represents an LLM provider type
type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderVLLM   Provider = "vllm"
	ProviderOpenAI Provider = "openai"
	ProviderLocal  Provider = "local"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Config configures the LLM client
type Config struct {
	Provider    Provider      `json:"provider"`
	BaseURL     string        `json:"base_url"`
	APIKey      string        `json:"api_key,omitempty"`
	DefaultModel string       `json:"default_model"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int           `json:"max_retries"`
}

// DefaultConfig returns default LLM configuration
func DefaultConfig() *Config {
	return &Config{
		Provider:    ProviderOllama,
		BaseURL:     "http://localhost:11434",
		DefaultModel: "deepseek-r1:32b",
		Timeout:     120 * time.Second,
		MaxRetries:  3,
	}
}

// Client provides LLM interaction functionality
type Client struct {
	config   *Config
	http     *http.Client
	provider LLMProvider
}

// LLMProvider defines the interface for LLM implementations
type LLMProvider interface {
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req *ChatRequest, callback StreamCallback) error
}

// StreamCallback is called for each streamed response chunk
type StreamCallback func(chunk string) error

// NewClient creates a new LLM client
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	var provider LLMProvider

	switch cfg.Provider {
	case ProviderOllama:
		provider = NewOllamaProvider(cfg, httpClient)
	case ProviderVLLM:
		provider = NewVLLMProvider(cfg, httpClient)
	case ProviderOpenAI:
		provider = NewOpenAIProvider(cfg, httpClient)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	return &Client{
		config:   cfg,
		http:     httpClient,
		provider: provider,
	}, nil
}

// ChatCompletion sends a chat completion request
func (c *Client) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.config.DefaultModel
	}

	return c.provider.ChatCompletion(ctx, req)
}

// StreamChat streams a chat completion
func (c *Client) StreamChat(ctx context.Context, req *ChatRequest, callback StreamCallback) error {
	if req.Model == "" {
		req.Model = c.config.DefaultModel
	}
	req.Stream = true

	return c.provider.StreamChat(ctx, req, callback)
}

// Complete is a convenience method for single-turn completion
func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
	resp, err := c.ChatCompletion(ctx, &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// CompleteWithSystem sends a completion with a system prompt
func (c *Client) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := c.ChatCompletion(ctx, &ChatRequest{
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// OllamaProvider implements LLMProvider for Ollama
type OllamaProvider struct {
	config *Config
	client *http.Client
}

// NewOllamaProvider creates an Ollama provider
func NewOllamaProvider(cfg *Config, client *http.Client) *OllamaProvider {
	return &OllamaProvider{
		config: cfg,
		client: client,
	}
}

// ollamaChatRequest is the Ollama-specific request format
type ollamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  struct {
		Temperature float64 `json:"temperature,omitempty"`
		NumPredict  int     `json:"num_predict,omitempty"`
		TopP        float64 `json:"top_p,omitempty"`
		Stop        []string `json:"stop,omitempty"`
	} `json:"options,omitempty"`
}

// ollamaChatResponse is the Ollama-specific response format
type ollamaChatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
	TotalDuration  int64 `json:"total_duration"`
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount int `json:"eval_count"`
}

// ChatCompletion sends a chat completion to Ollama
func (o *OllamaProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	ollamaReq := ollamaChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
	}
	ollamaReq.Options.Temperature = req.Temperature
	ollamaReq.Options.NumPredict = req.MaxTokens
	ollamaReq.Options.TopP = req.TopP
	ollamaReq.Options.Stop = req.Stop

	jsonBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/chat", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: %s", string(body))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	return &ChatResponse{
		Model: ollamaResp.Model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    ollamaResp.Message.Role,
					Content: ollamaResp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// StreamChat streams a chat completion from Ollama
func (o *OllamaProvider) StreamChat(ctx context.Context, req *ChatRequest, callback StreamCallback) error {
	ollamaReq := ollamaChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   true,
	}
	ollamaReq.Options.Temperature = req.Temperature
	ollamaReq.Options.NumPredict = req.MaxTokens

	jsonBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/chat", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaChatResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if err := callback(chunk.Message.Content); err != nil {
			return err
		}

		if chunk.Done {
			break
		}
	}

	return nil
}

// VLLMProvider implements LLMProvider for vLLM
type VLLMProvider struct {
	config *Config
	client *http.Client
}

// NewVLLMProvider creates a vLLM provider
func NewVLLMProvider(cfg *Config, client *http.Client) *VLLMProvider {
	return &VLLMProvider{
		config: cfg,
		client: client,
	}
}

// vllmChatRequest is the vLLM-specific request format (OpenAI-compatible)
type vllmChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// ChatCompletion sends a chat completion to vLLM
func (v *VLLMProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	vllmReq := vllmChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      false,
		Stop:        req.Stop,
	}

	jsonBody, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/chat/completions", v.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if v.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+v.config.APIKey)
	}

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vllm error: %s", string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

// StreamChat streams a chat completion from vLLM
func (v *VLLMProvider) StreamChat(ctx context.Context, req *ChatRequest, callback StreamCallback) error {
	vllmReq := vllmChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	jsonBody, err := json.Marshal(vllmReq)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/chat/completions", v.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if v.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+v.config.APIKey)
	}

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := resp.Body
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		lines := strings.Split(string(buf[:n]), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if err := callback(chunk.Choices[0].Delta.Content); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// OpenAIProvider implements LLMProvider for OpenAI API
type OpenAIProvider struct {
	config *Config
	client *http.Client
}

// NewOpenAIProvider creates an OpenAI provider
func NewOpenAIProvider(cfg *Config, client *http.Client) *OpenAIProvider {
	return &OpenAIProvider{
		config: cfg,
		client: client,
	}
}

// ChatCompletion sends a chat completion to OpenAI
func (o *OpenAIProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/chat/completions", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error: %s", string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

// StreamChat streams a chat completion from OpenAI
func (o *OpenAIProvider) StreamChat(ctx context.Context, req *ChatRequest, callback StreamCallback) error {
	req.Stream = true
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/chat/completions", o.config.BaseURL),
		bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := resp.Body
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		lines := strings.Split(string(buf[:n]), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return nil
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				if err := callback(chunk.Choices[0].Delta.Content); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ModelSelector helps select appropriate models for different tasks
type ModelSelector struct {
	models map[string]string
}

// NewModelSelector creates a model selector with default mappings
func NewModelSelector() *ModelSelector {
	return &ModelSelector{
		models: map[string]string{
			"fast":      "llama3.2:7b",
			"medium":    "llama3.2:14b",
			"reasoning": "deepseek-r1:32b",
			"vision":    "llava:13b",
		},
	}
}

// Select returns the appropriate model for a task type
func (ms *ModelSelector) Select(taskType string) string {
	if model, ok := ms.models[taskType]; ok {
		return model
	}
	return ms.models["medium"]
}

// SetModel sets a custom model for a task type
func (ms *ModelSelector) SetModel(taskType, model string) {
	ms.models[taskType] = model
}

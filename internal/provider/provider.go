package provider

import (
	"context"
	"fmt"
	"os"
)

// CompletionRequest is the input to an LLM completion.
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxTokens    int
}

// CompletionResponse is the output from an LLM completion.
type CompletionResponse struct {
	Text      string
	Model     string
	LatencyMs int64
}

// LLMClient is the interface for making completions against any LLM provider.
type LLMClient interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// Config holds provider configuration.
type Config struct {
	Provider  string // "anthropic", "openai", "openai-compatible"
	Model     string
	BaseURL   string // for openai-compatible
	APIKeyEnv string // env var name to read API key from
	MaxTokens int
}

// NewClient creates an LLMClient from configuration.
func NewClient(cfg Config) (LLMClient, error) {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 512
	}

	switch cfg.Provider {
	case "anthropic":
		if cfg.Model == "" {
			cfg.Model = "claude-sonnet-4-5-20250514"
		}
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "ANTHROPIC_API_KEY"
		}
		apiKey := os.Getenv(keyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("environment variable %s is not set", keyEnv)
		}
		return &AnthropicClient{
			apiKey:    apiKey,
			model:     cfg.Model,
			maxTokens: cfg.MaxTokens,
		}, nil

	case "openai":
		if cfg.Model == "" {
			cfg.Model = "gpt-4o"
		}
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "OPENAI_API_KEY"
		}
		apiKey := os.Getenv(keyEnv)
		if apiKey == "" {
			return nil, fmt.Errorf("environment variable %s is not set", keyEnv)
		}
		return &OpenAIClient{
			apiKey:    apiKey,
			model:     cfg.Model,
			maxTokens: cfg.MaxTokens,
			baseURL:   "https://api.openai.com/v1",
		}, nil

	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("base_url is required for openai-compatible provider")
		}
		if cfg.Model == "" {
			return nil, fmt.Errorf("model is required for openai-compatible provider")
		}
		keyEnv := cfg.APIKeyEnv
		apiKey := ""
		if keyEnv != "" {
			apiKey = os.Getenv(keyEnv)
		}
		return &OpenAIClient{
			apiKey:    apiKey, // may be empty for local providers like Ollama
			model:     cfg.Model,
			maxTokens: cfg.MaxTokens,
			baseURL:   cfg.BaseURL,
		}, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: anthropic, openai, openai-compatible)", cfg.Provider)
	}
}

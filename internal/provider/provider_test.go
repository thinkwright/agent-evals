package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// --- NewClient tests ---

func TestNewClientUnknownProvider(t *testing.T) {
	_, err := NewClient(Config{Provider: "nope"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewClientAnthropicMissingKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	_, err := NewClient(Config{Provider: "anthropic"})
	if err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is unset")
	}
}

func TestNewClientAnthropicDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	client, err := NewClient(Config{Provider: "anthropic"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ac, ok := client.(*AnthropicClient)
	if !ok {
		t.Fatal("expected *AnthropicClient")
	}
	if ac.model == "" {
		t.Error("expected default model to be set")
	}
	if ac.maxTokens != 512 {
		t.Errorf("expected default maxTokens 512, got %d", ac.maxTokens)
	}
}

func TestNewClientOpenAIMissingKey(t *testing.T) {
	os.Unsetenv("OPENAI_API_KEY")
	_, err := NewClient(Config{Provider: "openai"})
	if err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is unset")
	}
}

func TestNewClientOpenAIDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	client, err := NewClient(Config{Provider: "openai"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oc, ok := client.(*OpenAIClient)
	if !ok {
		t.Fatal("expected *OpenAIClient")
	}
	if oc.baseURL != "https://api.openai.com/v1" {
		t.Errorf("unexpected baseURL: %s", oc.baseURL)
	}
}

func TestNewClientOpenAICompatMissingBaseURL(t *testing.T) {
	_, err := NewClient(Config{Provider: "openai-compatible", Model: "llama3"})
	if err == nil {
		t.Fatal("expected error when base_url is missing")
	}
}

func TestNewClientOpenAICompatMissingModel(t *testing.T) {
	_, err := NewClient(Config{Provider: "openai-compatible", BaseURL: "http://localhost:11434/v1"})
	if err == nil {
		t.Fatal("expected error when model is missing")
	}
}

func TestNewClientOpenAICompatNoKeyRequired(t *testing.T) {
	client, err := NewClient(Config{
		Provider: "openai-compatible",
		BaseURL:  "http://localhost:11434/v1",
		Model:    "llama3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oc := client.(*OpenAIClient)
	if oc.apiKey != "" {
		t.Error("expected empty API key for local provider")
	}
}

func TestNewClientCustomAPIKeyEnv(t *testing.T) {
	t.Setenv("CEREBRAS_API_KEY", "crs-test-key")
	client, err := NewClient(Config{
		Provider:  "openai-compatible",
		BaseURL:   "https://api.cerebras.ai/v1",
		Model:     "llama-4-scout-17b-16e-instruct",
		APIKeyEnv: "CEREBRAS_API_KEY",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oc := client.(*OpenAIClient)
	if oc.apiKey != "crs-test-key" {
		t.Errorf("expected API key from CEREBRAS_API_KEY, got %q", oc.apiKey)
	}
	if oc.baseURL != "https://api.cerebras.ai/v1" {
		t.Errorf("unexpected baseURL: %s", oc.baseURL)
	}
}

// --- HTTP round-trip tests ---

func TestOpenAIClientComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing or wrong Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}

		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Error("expected temperature to be set")
		}
		if req.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", req.Model)
		}

		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "hello from test"}},
			},
			Model: "test-model",
		})
	}))
	defer server.Close()

	client := &OpenAIClient{
		apiKey:    "test-key",
		model:     "test-model",
		maxTokens: 100,
		baseURL:   server.URL,
	}

	resp, err := client.Complete(context.Background(), CompletionRequest{
		UserPrompt:  "hi",
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello from test" {
		t.Errorf("unexpected response text: %s", resp.Text)
	}
	if resp.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestAnthropicClientComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing or wrong x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("missing anthropic-version header")
		}

		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Temperature == nil {
			t.Error("expected temperature to be set")
		}
		if req.System != "you are helpful" {
			t.Errorf("expected system prompt, got %q", req.System)
		}

		json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Text string `json:"text"`
			}{{Text: "hello from anthropic"}},
			Model: "claude-test",
		})
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:    "test-key",
		model:     "claude-test",
		maxTokens: 100,
		baseURL:   server.URL,
	}

	resp, err := client.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "you are helpful",
		UserPrompt:   "hi",
		Temperature:  0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello from anthropic" {
		t.Errorf("unexpected response text: %s", resp.Text)
	}
	if resp.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}
}

func TestOpenAIClientErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	client := &OpenAIClient{
		apiKey:    "test-key",
		model:     "test-model",
		maxTokens: 100,
		baseURL:   server.URL,
	}

	_, err := client.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestOpenAIClientEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openaiResponse{Model: "test"})
	}))
	defer server.Close()

	client := &OpenAIClient{
		apiKey:    "test-key",
		model:     "test-model",
		maxTokens: 100,
		baseURL:   server.URL,
	}

	_, err := client.Complete(context.Background(), CompletionRequest{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

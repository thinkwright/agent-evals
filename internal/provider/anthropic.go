package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicClient implements LLMClient for the Anthropic Messages API.
type AnthropicClient struct {
	apiKey    string
	model     string
	maxTokens int
	baseURL   string // defaults to "https://api.anthropic.com/v1"
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *AnthropicClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.UserPrompt},
		},
	}
	temp := req.Temperature
	body.Temperature = &temp
	if req.SystemPrompt != "" {
		body.System = req.SystemPrompt
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	base := c.baseURL
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", base+"/messages", bytes.NewReader(payload))
	if err != nil {
		return CompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return CompletionResponse{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error != nil {
		return CompletionResponse{}, fmt.Errorf("anthropic error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return CompletionResponse{}, fmt.Errorf("empty response from anthropic")
	}

	return CompletionResponse{
		Text:      result.Content[0].Text,
		Model:     result.Model,
		LatencyMs: latency,
	}, nil
}

package probes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thinkwright/agent-evals/internal/loader"
	"github.com/thinkwright/agent-evals/internal/provider"
)

// panicClient is a mock LLMClient that panics when the prompt contains
// a trigger string, and returns a normal response otherwise.
type panicClient struct {
	trigger string
}

func (c *panicClient) Complete(_ context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	if strings.Contains(req.UserPrompt, c.trigger) {
		panic("simulated crash in LLM response handling")
	}
	return provider.CompletionResponse{
		Text:  "I'm not sure about that. Confidence: 30",
		Model: "test-model",
	}, nil
}

func TestRunLiveProbesPanicRecovery(t *testing.T) {
	agents := []loader.AgentDefinition{
		{ID: "agent1", SystemPrompt: "You are a test agent."},
	}

	questions := []ProbeQuestion{
		{
			ID:               "panic-probe",
			Text:             "TRIGGER_PANIC",
			TargetAgent:      "agent1",
			Domain:           "testing",
			ProbeType:        "boundary",
			ExpectedBehavior: "hedge",
		},
		{
			ID:               "normal-probe",
			Text:             "What is Go?",
			TargetAgent:      "agent1",
			Domain:           "backend",
			ProbeType:        "calibration",
			ExpectedBehavior: "answer",
		},
	}

	client := &panicClient{trigger: "TRIGGER_PANIC"}

	var progressCalls int
	progress := func(done, total int, agentID, probeID string) {
		progressCalls++
	}

	report := RunLiveProbes(context.Background(), agents, questions, client, RunConfig{
		StochasticRuns: 1,
		BatchDelay:     time.Millisecond,
		Concurrency:    1,
	}, progress)

	// The run should complete without crashing
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	results := report.AgentResults["agent1"]
	if results == nil {
		t.Fatal("expected results for agent1")
	}

	// Both probes should be recorded (one panicked, one normal)
	if results.ProbesRun != 2 {
		t.Errorf("expected 2 probes run, got %d", results.ProbesRun)
	}

	// Progress should have been called for both probes
	if progressCalls != 2 {
		t.Errorf("expected 2 progress calls, got %d", progressCalls)
	}

	// Find the panicked probe and verify it recorded the error
	var panicDetail *ProbeDetail
	var normalDetail *ProbeDetail
	for i := range results.Details {
		switch results.Details[i].ProbeID {
		case "panic-probe":
			panicDetail = &results.Details[i]
		case "normal-probe":
			normalDetail = &results.Details[i]
		}
	}

	if panicDetail == nil {
		t.Fatal("expected panic-probe in results")
	}
	if len(panicDetail.Responses) != 1 {
		t.Fatalf("expected 1 response for panicked probe, got %d", len(panicDetail.Responses))
	}
	if !strings.Contains(panicDetail.Responses[0].Error, "panic:") {
		t.Errorf("expected panic error message, got %q", panicDetail.Responses[0].Error)
	}

	if normalDetail == nil {
		t.Fatal("expected normal-probe in results")
	}
	if len(normalDetail.Responses) == 0 {
		t.Fatal("expected responses for normal probe")
	}
	// Normal probe should have successful responses (no error)
	if normalDetail.Responses[0].Error != "" {
		t.Errorf("expected no error for normal probe, got %q", normalDetail.Responses[0].Error)
	}
}

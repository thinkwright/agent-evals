package probes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/thinkwright/agent-evals/internal/loader"
	"github.com/thinkwright/agent-evals/internal/provider"
)

// LiveProbeReport holds results from all live probes.
type LiveProbeReport struct {
	AgentResults map[string]*AgentProbeResults
	TotalCalls   int
	Budget       int
	Timestamp    string
}

// ProgressCallback is called after each probe completes.
type ProgressCallback func(done, total int, agentID, probeID string)

// RunConfig holds configuration for running probes.
type RunConfig struct {
	StochasticRuns int
	BatchDelay     time.Duration
	Concurrency    int
}

// RunLiveProbes executes live probes against agents via the LLM API.
func RunLiveProbes(ctx context.Context, agents []loader.AgentDefinition, questions []ProbeQuestion,
	client provider.LLMClient, cfg RunConfig, progress ProgressCallback) *LiveProbeReport {

	if cfg.StochasticRuns == 0 {
		cfg.StochasticRuns = 5
	}
	if cfg.BatchDelay == 0 {
		cfg.BatchDelay = 300 * time.Millisecond
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 1
	}

	agentMap := make(map[string]*loader.AgentDefinition)
	for i := range agents {
		agentMap[agents[i].ID] = &agents[i]
	}

	results := make(map[string]*AgentProbeResults)
	for _, a := range agents {
		results[a.ID] = &AgentProbeResults{AgentID: a.ID}
	}

	var mu sync.Mutex
	totalCalls := 0
	completed := 0
	total := len(questions)

	sem := make(chan struct{}, cfg.Concurrency)

	var wg sync.WaitGroup
	for _, q := range questions {
		agent, ok := agentMap[q.TargetAgent]
		if !ok {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(probe ProbeQuestion, agent *loader.AgentDefinition) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[probe.TargetAgent].ProbesRun++
					results[probe.TargetAgent].Details = append(results[probe.TargetAgent].Details, ProbeDetail{
						ProbeID:   probe.ID,
						Question:  probe.Text,
						Domain:    probe.Domain,
						ProbeType: probe.ProbeType,
						Expected:  probe.ExpectedBehavior,
						Responses: []ResponseRecord{{Run: 0, Error: fmt.Sprintf("panic: %v", r)}},
					})
					completed++
					if progress != nil {
						progress(completed, total, probe.TargetAgent, probe.ID)
					}
					mu.Unlock()
				}
			}()

			prompt := fmt.Sprintf(BoundaryProbeTemplate, probe.Text)
			var responses []ResponseRecord

			// Deterministic run
			resp, err := client.Complete(ctx, provider.CompletionRequest{
				SystemPrompt: agent.SystemPrompt,
				UserPrompt:   prompt,
				Temperature:  0,
			})
			mu.Lock()
			totalCalls++
			mu.Unlock()

			if err != nil {
				responses = append(responses, ResponseRecord{Run: 0, Error: err.Error()})
			} else {
				parsed := ParseProbeResponse(resp.Text)
				responses = append(responses, ResponseRecord{
					Run:          0,
					Temperature:  0,
					Confidence:   parsed.Confidence,
					HedgingScore: parsed.HedgingScore,
					IsRefusal:    parsed.IsRefusal,
					Raw:          resp.Text,
				})
			}

			// Stochastic runs
			for i := 1; i <= cfg.StochasticRuns; i++ {
				resp, err := client.Complete(ctx, provider.CompletionRequest{
					SystemPrompt: agent.SystemPrompt,
					UserPrompt:   prompt,
					Temperature:  0.7,
				})
				mu.Lock()
				totalCalls++
				mu.Unlock()

				if err != nil {
					responses = append(responses, ResponseRecord{Run: i, Temperature: 0.7, Error: err.Error()})
				} else {
					parsed := ParseProbeResponse(resp.Text)
					responses = append(responses, ResponseRecord{
						Run:          i,
						Temperature:  0.7,
						Confidence:   parsed.Confidence,
						HedgingScore: parsed.HedgingScore,
						IsRefusal:    parsed.IsRefusal,
						Raw:          resp.Text,
					})
				}

				time.Sleep(cfg.BatchDelay)
			}

			detail := ProbeDetail{
				ProbeID:   probe.ID,
				Question:  probe.Text,
				Domain:    probe.Domain,
				ProbeType: probe.ProbeType,
				Expected:  probe.ExpectedBehavior,
				Responses: responses,
			}

			mu.Lock()
			results[probe.TargetAgent].ProbesRun++
			results[probe.TargetAgent].Details = append(results[probe.TargetAgent].Details, detail)
			completed++
			if progress != nil {
				progress(completed, total, probe.TargetAgent, probe.ID)
			}
			mu.Unlock()

		}(q, agent)
	}

	wg.Wait()

	// Score each agent
	for _, r := range results {
		ScoreAgentProbes(r)
	}

	return &LiveProbeReport{
		AgentResults: results,
		TotalCalls:   totalCalls,
		Budget:       len(questions) * (1 + cfg.StochasticRuns),
		Timestamp:    time.Now().Format(time.RFC3339),
	}
}

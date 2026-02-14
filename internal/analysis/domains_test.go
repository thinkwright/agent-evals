package analysis

import (
	"testing"

	"github.com/thinkwright/agent-evals/internal/loader"
)

func TestExtractDomainsKeywordMatching(t *testing.T) {
	agent := &loader.AgentDefinition{
		ID: "backend_api",
		SystemPrompt: `You are a backend API developer. You build REST APIs,
			work with PostgreSQL databases, handle SQL query optimization,
			and design microservice architectures.`,
	}

	domains := ExtractDomains(agent)

	// Should detect backend (multiple keyword hits: backend, api, rest, microservice)
	if domains["backend"] == 0 {
		t.Error("expected backend domain to be detected from keyword hits")
	}
	if domains["backend"] < 0.5 {
		t.Errorf("expected backend score > 0.5 given multiple keyword hits, got %.2f", domains["backend"])
	}

	// Should detect databases (database, sql, postgres, query optimization)
	if domains["databases"] == 0 {
		t.Error("expected databases domain to be detected")
	}

	// Should NOT detect unrelated domains
	if domains["legal"] > 0 {
		t.Errorf("expected no legal domain score, got %.2f", domains["legal"])
	}
	if domains["medical"] > 0 {
		t.Errorf("expected no medical domain score, got %.2f", domains["medical"])
	}
}

func TestExtractDomainsClaimedDomainsOverride(t *testing.T) {
	agent := &loader.AgentDefinition{
		ID:             "custom",
		SystemPrompt:   "You help with things.",
		ClaimedDomains: []string{"Security", "dev-ops"},
	}

	domains := ExtractDomains(agent)

	// Claimed domains should be normalized and set to 1.0
	if domains["security"] != 1.0 {
		t.Errorf("expected claimed domain 'security' to score 1.0, got %.2f", domains["security"])
	}
	if domains["dev_ops"] != 1.0 {
		t.Errorf("expected claimed domain 'dev-ops' normalized to 'dev_ops' at 1.0, got %.2f", domains["dev_ops"])
	}
}

func TestExtractDomainsClaimedAndKeywordBothPresent(t *testing.T) {
	// When a domain is both claimed and keyword-matched, the higher score should win
	agent := &loader.AgentDefinition{
		ID:             "sec_agent",
		SystemPrompt:   "You handle security, authentication, and oauth.",
		ClaimedDomains: []string{"security"},
	}

	domains := ExtractDomains(agent)

	// Claimed = 1.0, keyword hits would produce some score. Result should be 1.0.
	if domains["security"] != 1.0 {
		t.Errorf("expected claimed domain to keep 1.0 even with keyword hits, got %.2f", domains["security"])
	}
}

func TestExtractDomainsEmptyPrompt(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "empty", SystemPrompt: ""}
	domains := ExtractDomains(agent)

	if len(domains) != 0 {
		t.Errorf("expected no domains for empty prompt, got %d: %v", len(domains), domains)
	}
}

func TestExtractDomainsScoreCapping(t *testing.T) {
	// A prompt saturated with keywords for one domain should cap at 1.0
	agent := &loader.AgentDefinition{
		ID: "testing_heavy",
		SystemPrompt: `testing test unit test integration test e2e coverage
			tdd bdd cypress playwright jest testing test test test`,
	}

	domains := ExtractDomains(agent)

	if domains["testing"] > 1.0 {
		t.Errorf("domain score should be capped at 1.0, got %.2f", domains["testing"])
	}
}

func TestExtractDomainsSkillsAndRulesContribute(t *testing.T) {
	agent := &loader.AgentDefinition{
		ID:           "with_skills",
		SystemPrompt: "You are a helpful assistant.",
		Skills:       []string{"Docker deployment", "Kubernetes management", "Terraform provisioning"},
		Rules:        []string{"Always follow CI/CD best practices", "Use Helm for deployments"},
	}

	domains := ExtractDomains(agent)

	// DevOps keywords are in skills and rules
	if domains["devops"] == 0 {
		t.Error("expected devops domain from skills/rules keywords (docker, kubernetes, terraform, ci/cd, helm)")
	}
}

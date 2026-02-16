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

	domains := ExtractDomains(agent, BuiltinDomains)

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

	domains := ExtractDomains(agent, BuiltinDomains)

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

	domains := ExtractDomains(agent, BuiltinDomains)

	// Claimed = 1.0, keyword hits would produce some score. Result should be 1.0.
	if domains["security"] != 1.0 {
		t.Errorf("expected claimed domain to keep 1.0 even with keyword hits, got %.2f", domains["security"])
	}
}

func TestExtractDomainsEmptyPrompt(t *testing.T) {
	agent := &loader.AgentDefinition{ID: "empty", SystemPrompt: ""}
	domains := ExtractDomains(agent, BuiltinDomains)

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

	domains := ExtractDomains(agent, BuiltinDomains)

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

	domains := ExtractDomains(agent, BuiltinDomains)

	// DevOps keywords are in skills and rules
	if domains["devops"] == 0 {
		t.Error("expected devops domain from skills/rules keywords (docker, kubernetes, terraform, ci/cd, helm)")
	}
}

// --- ResolveDomains tests ---

func TestResolveDomainsMissing(t *testing.T) {
	result := ResolveDomains(nil)
	if len(result) != len(BuiltinDomains) {
		t.Errorf("expected %d built-in domains, got %d", len(BuiltinDomains), len(result))
	}
}

func TestResolveDomainsEmptyList(t *testing.T) {
	result := ResolveDomains(map[string]any{"domains": []any{}})
	if len(result) != len(BuiltinDomains) {
		t.Errorf("expected %d built-in domains for empty list, got %d", len(BuiltinDomains), len(result))
	}
}

func TestResolveDomainsStringRefs(t *testing.T) {
	result := ResolveDomains(map[string]any{
		"domains": []any{"backend", "frontend"},
	})
	if len(result) != 2 {
		t.Errorf("expected 2 domains, got %d", len(result))
	}
	if _, ok := result["backend"]; !ok {
		t.Error("expected backend domain")
	}
	if _, ok := result["frontend"]; !ok {
		t.Error("expected frontend domain")
	}
}

func TestResolveDomainsCustom(t *testing.T) {
	result := ResolveDomains(map[string]any{
		"domains": []any{
			map[string]any{
				"name":     "payments",
				"keywords": []any{"stripe", "plaid", "ach transfer"},
			},
		},
	})
	if len(result) != 1 {
		t.Errorf("expected 1 domain, got %d", len(result))
	}
	if kw, ok := result["payments"]; !ok {
		t.Error("expected payments domain")
	} else if len(kw) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(kw))
	}
}

func TestResolveDomainsExtends(t *testing.T) {
	result := ResolveDomains(map[string]any{
		"domains": []any{
			map[string]any{
				"name":     "backend",
				"extends":  "builtin",
				"keywords": []any{"axum", "actix-web"},
			},
		},
	})
	kw := result["backend"]
	builtinLen := len(BuiltinDomains["backend"])
	if len(kw) != builtinLen+2 {
		t.Errorf("expected %d keywords (builtin + 2), got %d", builtinLen+2, len(kw))
	}
	// Check that custom keywords are present
	found := false
	for _, k := range kw {
		if k == "axum" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom keyword 'axum' in merged result")
	}
}

func TestResolveDomainsMixed(t *testing.T) {
	result := ResolveDomains(map[string]any{
		"domains": []any{
			"frontend",
			map[string]any{
				"name":     "backend",
				"extends":  "builtin",
				"keywords": []any{"tokio"},
			},
			map[string]any{
				"name":     "gaming",
				"keywords": []any{"unity", "unreal"},
			},
		},
	})
	if len(result) != 3 {
		t.Errorf("expected 3 domains, got %d", len(result))
	}
	if _, ok := result["frontend"]; !ok {
		t.Error("expected frontend domain")
	}
	if _, ok := result["backend"]; !ok {
		t.Error("expected backend domain")
	}
	if _, ok := result["gaming"]; !ok {
		t.Error("expected gaming domain")
	}
}

func TestResolveDomainsUnknownBuiltin(t *testing.T) {
	result := ResolveDomains(map[string]any{
		"domains": []any{"backend", "nonexistent"},
	})
	if len(result) != 1 {
		t.Errorf("expected 1 domain (unknown skipped), got %d", len(result))
	}
	if _, ok := result["backend"]; !ok {
		t.Error("expected backend domain")
	}
}

func TestExtractDomainsCustomKeywords(t *testing.T) {
	custom := map[string][]string{
		"payments": {"stripe", "plaid", "payment gateway"},
	}
	agent := &loader.AgentDefinition{
		ID:           "pay_agent",
		SystemPrompt: "You process payments via Stripe and Plaid.",
	}
	domains := ExtractDomains(agent, custom)
	if domains["payments"] == 0 {
		t.Error("expected payments domain from custom keywords")
	}
	// Should NOT have any built-in domains since we only passed custom
	if domains["backend"] > 0 {
		t.Error("did not expect backend domain with custom-only keywords")
	}
}

package analysis

import (
	"fmt"
	"os"
	"strings"

	"github.com/thinkwright/agent-evals/internal/loader"
)

// BuiltinDomains maps normalized domain labels to keywords found in agent prompts.
var BuiltinDomains = map[string][]string{
	// Software engineering
	"backend": {"backend", "server", "api", "rest", "graphql", "grpc",
		"microservice", "service layer", "business logic", "middleware",
		"endpoint", "request handling", "http server"},
	"frontend": {"frontend", "front-end", "react", "vue", "angular", "svelte",
		"css", "html", "browser", "dom", "ui component", "web app",
		"responsive", "accessibility", "a11y", "tailwind", "next.js", "nuxt"},
	"databases": {"database", "sql", "postgres", "mysql", "mongodb", "redis",
		"query optimization", "indexing", "schema", "migration", "orm",
		"sqlite", "dynamodb", "cassandra", "connection pool", "transaction"},
	"devops": {"devops", "ci/cd", "pipeline", "docker", "kubernetes", "k8s",
		"terraform", "ansible", "infrastructure", "deployment", "helm",
		"github actions", "gitlab ci", "jenkins", "argocd", "container"},
	"security": {"security", "authentication", "authorization", "oauth", "jwt",
		"encryption", "vulnerability", "penetration", "owasp", "cors", "csrf",
		"xss", "rbac", "sso", "zero trust", "secrets management", "tls",
		"certificate", "firewall", "audit log"},
	"distributed_systems": {"distributed", "consensus", "replication",
		"partition", "raft", "paxos", "eventual consistency",
		"message queue", "kafka", "event-driven", "pub/sub", "rabbitmq",
		"nats", "grpc streaming", "load balancing", "circuit breaker"},
	"mobile": {"mobile", "ios", "android", "react native", "flutter",
		"swift", "kotlin", "xcode", "app store", "google play",
		"push notification", "deep link", "mobile ui"},
	"ml_ai": {"machine learning", "deep learning", "neural network",
		"training", "inference", "pytorch", "tensorflow", "transformer",
		"fine-tuning", "rag", "embedding", "llm", "prompt engineering",
		"classification", "regression", "nlp", "computer vision",
		"reinforcement learning", "diffusion model", "vector database"},
	"testing": {"testing", "test", "unit test", "integration test", "e2e",
		"coverage", "tdd", "bdd", "cypress", "playwright", "jest",
		"pytest", "vitest", "test fixture", "mock", "stub",
		"snapshot test", "load test", "regression test"},
	"architecture": {"architecture", "system design", "design pattern",
		"microservices", "monolith", "event sourcing", "cqrs",
		"domain-driven", "hexagonal", "clean architecture", "solid",
		"api gateway", "service mesh", "saga pattern"},
	"data_science": {"data science", "data analysis", "pandas", "numpy",
		"jupyter", "visualization", "statistics", "data pipeline",
		"etl", "data warehouse", "spark", "airflow", "dbt",
		"feature engineering", "a/b test", "experiment", "dashboard",
		"data lake", "bigquery", "snowflake", "redshift"},
	"cloud": {"aws", "azure", "gcp", "cloud", "s3", "ec2", "lambda",
		"serverless", "cloud function", "cloud run", "iam",
		"vpc", "cdn", "route 53", "cloudfront", "load balancer",
		"auto scaling", "fargate", "ecs", "cloud formation"},
	"observability": {"observability", "monitoring", "logging", "tracing",
		"metrics", "prometheus", "grafana", "datadog", "opentelemetry",
		"alerting", "sli", "slo", "sla", "incident", "on-call",
		"pagerduty", "kibana", "elasticsearch", "apm"},
	"api_design": {"api design", "openapi", "swagger", "rest api",
		"api versioning", "rate limiting", "pagination", "hateoas",
		"api gateway", "webhook", "idempotent", "api contract",
		"protobuf", "schema registry", "backward compatible"},
	// Non-technical
	"legal": {"legal", "law", "regulation", "compliance", "contract",
		"liability", "intellectual property", "gdpr", "hipaa",
		"terms of service", "privacy policy", "copyright", "patent"},
	"medical": {"medical", "clinical", "diagnosis", "treatment", "patient",
		"pharmacology", "symptom", "dosage", "contraindication",
		"clinical trial", "healthcare", "therapeutic"},
	"financial": {"financial", "accounting", "revenue", "profit", "balance sheet",
		"investment", "portfolio", "tax", "audit", "budgeting",
		"financial model", "valuation", "equity", "debt", "forex"},
	"writing": {"writing", "copywriting", "content", "blog", "article",
		"editorial", "prose", "narrative", "technical writing",
		"documentation", "style guide", "tone of voice"},
}

// ResolveDomains builds a domain keyword map from configuration. If config is
// nil or has no "domains" key, all built-in domains are returned. Entries can
// be strings (built-in refs) or maps with name, optional extends, and keywords.
func ResolveDomains(config map[string]any) map[string][]string {
	if config == nil {
		return copyDomains(BuiltinDomains)
	}
	raw, ok := config["domains"]
	if !ok {
		return copyDomains(BuiltinDomains)
	}
	entries, ok := raw.([]any)
	if !ok || len(entries) == 0 {
		return copyDomains(BuiltinDomains)
	}

	result := make(map[string][]string)
	for _, entry := range entries {
		switch v := entry.(type) {
		case string:
			if kw, ok := BuiltinDomains[v]; ok {
				result[v] = copySlice(kw)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: unknown built-in domain %q, skipping\n", v)
			}
		case map[string]any:
			name, _ := v["name"].(string)
			if name == "" {
				continue
			}
			keywords := toStringSlice(v["keywords"])
			extends, _ := v["extends"].(string)
			if extends == "builtin" {
				if builtin, ok := BuiltinDomains[name]; ok {
					merged := copySlice(builtin)
					merged = append(merged, keywords...)
					result[name] = merged
				} else {
					// extends unknown built-in — treat as custom-only
					if len(keywords) > 0 {
						result[name] = keywords
					}
				}
			} else {
				if len(keywords) > 0 {
					result[name] = keywords
				}
			}
		}
	}

	return result
}

func copyDomains(src map[string][]string) map[string][]string {
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		dst[k] = copySlice(v)
	}
	return dst
}

func copySlice(s []string) []string {
	c := make([]string, len(s))
	copy(c, s)
	return c
}

func toStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ExtractDomains extracts domains from an agent's definition with relevance scores.
// Returns a map of domain -> relevance_score (0-1).
func ExtractDomains(agent *loader.AgentDefinition, domainKeywords map[string][]string) map[string]float64 {
	text := strings.ToLower(agent.FullContext())
	scores := make(map[string]float64)

	// Start with explicitly claimed domains
	for _, domain := range agent.ClaimedDomains {
		normalized := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(domain), " ", "_"), "-", "_")
		scores[normalized] = 1.0
	}

	// Keyword-based extraction.
	// Score = hits / (len(keywords) * 0.5). The 0.5 factor means an agent
	// matching half its domain's keywords reaches 1.0, reflecting that no
	// single prompt will use every keyword in a domain.
	for domain, keywords := range domainKeywords {
		hits := 0
		for _, kw := range keywords {
			hits += strings.Count(text, kw)
		}
		if hits > 0 {
			score := float64(hits) / (float64(len(keywords)) * 0.5)
			if score > 1.0 {
				score = 1.0
			}
			if existing, ok := scores[domain]; !ok || score > existing {
				scores[domain] = score
			}
		}
	}

	return scores
}

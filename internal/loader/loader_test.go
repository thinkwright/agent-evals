package loader

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestLoadYAML(t *testing.T) {
	agent, err := loadYAML(testdataPath("backend_api.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}

	if agent.ID != "backend_api" {
		t.Errorf("ID = %q, want %q", agent.ID, "backend_api")
	}
	if agent.Name != "Backend API Agent" {
		t.Errorf("Name = %q, want %q", agent.Name, "Backend API Agent")
	}
	if agent.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if len(agent.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(agent.Skills))
	}
	if len(agent.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(agent.Rules))
	}
	if len(agent.ClaimedDomains) != 2 {
		t.Errorf("expected 2 claimed domains, got %d: %v", len(agent.ClaimedDomains), agent.ClaimedDomains)
	}
}

func TestLoadYAMLAlternativeFields(t *testing.T) {
	agent, err := loadYAML(testdataPath("alt_fields.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent from 'instructions' field, got nil")
	}
	if agent.SystemPrompt == "" {
		t.Error("expected system prompt from 'instructions' field")
	}
	// domain_tags should populate ClaimedDomains
	if len(agent.ClaimedDomains) != 1 || agent.ClaimedDomains[0] != "devops" {
		t.Errorf("expected ClaimedDomains=[devops] from domain_tags, got %v", agent.ClaimedDomains)
	}
}

func TestLoadYAMLNoPrompt(t *testing.T) {
	agent, err := loadYAML(testdataPath("no_prompt.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent != nil {
		t.Error("expected nil agent when no system_prompt field exists")
	}
}

func TestLoadYAMLIDFromFilename(t *testing.T) {
	agent, err := loadYAML(testdataPath("alt_fields.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}
	// No id field → should derive from filename
	if agent.ID != "alt_fields" {
		t.Errorf("ID = %q, want %q (derived from filename)", agent.ID, "alt_fields")
	}
}

func TestLoadJSON(t *testing.T) {
	agent, err := loadJSON(testdataPath("frontend.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}

	if agent.ID != "frontend_react" {
		t.Errorf("ID = %q, want %q", agent.ID, "frontend_react")
	}
	if agent.Name != "Frontend React Agent" {
		t.Errorf("Name = %q, want %q", agent.Name, "Frontend React Agent")
	}
	if len(agent.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(agent.Skills))
	}
	if len(agent.ClaimedDomains) != 1 || agent.ClaimedDomains[0] != "frontend" {
		t.Errorf("expected ClaimedDomains=[frontend], got %v", agent.ClaimedDomains)
	}
}

func TestLoadTextWithFrontmatter(t *testing.T) {
	agent, err := loadText(testdataPath("security_agent.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}

	if agent.Name != "Security Agent" {
		t.Errorf("Name = %q, want %q (from frontmatter)", agent.Name, "Security Agent")
	}
	if len(agent.ClaimedDomains) != 1 || agent.ClaimedDomains[0] != "security" {
		t.Errorf("expected ClaimedDomains=[security] from frontmatter, got %v", agent.ClaimedDomains)
	}
	if len(agent.Skills) != 2 {
		t.Errorf("expected 2 skills from frontmatter, got %d", len(agent.Skills))
	}
	// System prompt should be the content after frontmatter
	if agent.SystemPrompt == "" {
		t.Error("expected non-empty system prompt from markdown body")
	}
}

func TestLoadTextPlain(t *testing.T) {
	agent, err := loadText(testdataPath("plain_agent.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}

	// ID derived from filename
	if agent.ID != "plain_agent" {
		t.Errorf("ID = %q, want %q", agent.ID, "plain_agent")
	}
	// Name derived from stem
	if agent.Name != "Plain Agent" {
		t.Errorf("Name = %q, want %q", agent.Name, "Plain Agent")
	}
	// No frontmatter → no skills/rules/domains
	if len(agent.Skills) != 0 {
		t.Errorf("expected 0 skills for plain text, got %d", len(agent.Skills))
	}
}

func TestLoadTextTooShort(t *testing.T) {
	agent, err := loadText(testdataPath("too_short.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent != nil {
		t.Error("expected nil agent for content < 20 chars")
	}
}

func TestTryLoadDirectoryAgent(t *testing.T) {
	agent, err := tryLoadDirectoryAgent(testdataPath("dir_agent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent from directory, got nil")
	}

	if agent.ID != "dir_agent" {
		t.Errorf("ID = %q, want %q", agent.ID, "dir_agent")
	}
	if agent.SystemPrompt == "" {
		t.Error("expected system prompt from AGENT.md")
	}
	if len(agent.Skills) != 4 {
		t.Errorf("expected 4 skills from SKILLS.md, got %d: %v", len(agent.Skills), agent.Skills)
	}
	if len(agent.Rules) != 2 {
		t.Errorf("expected 2 rules from RULES.md, got %d: %v", len(agent.Rules), agent.Rules)
	}
}

func TestLoadAgentsDirectory(t *testing.T) {
	agents, err := LoadAgents(testdataPath(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// testdata has: dir_agent/ (directory), backend_api.yaml, frontend.json,
	// security_agent.md, plain_agent.txt, alt_fields.yaml
	// no_prompt.yaml → nil, too_short.txt → nil
	if len(agents) < 5 {
		t.Errorf("expected at least 5 agents from testdata, got %d", len(agents))
		for _, a := range agents {
			t.Logf("  loaded: %s (%s)", a.ID, a.SourcePath)
		}
	}

	// Verify we can find specific agents
	ids := make(map[string]bool)
	for _, a := range agents {
		ids[a.ID] = true
	}

	for _, expected := range []string{"backend_api", "frontend_react", "dir_agent"} {
		if !ids[expected] {
			t.Errorf("expected agent %q in loaded set", expected)
		}
	}
}

func TestExtractListItems(t *testing.T) {
	input := `# Skills
- React Native development
- Flutter widgets
* iOS deployment
* Android builds
Not a list item
  - Indented item
`
	items := extractListItems(input)

	if len(items) != 5 {
		t.Fatalf("expected 5 list items, got %d: %v", len(items), items)
	}
	if items[0] != "React Native development" {
		t.Errorf("items[0] = %q, want %q", items[0], "React Native development")
	}
}

func TestExtractListItemsEmpty(t *testing.T) {
	items := extractListItems("")
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty input, got %d", len(items))
	}
}

func TestFilenameStem(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/path/to/agent.yaml", "agent"},
		{"/path/to/my_agent.json", "my_agent"},
		{"simple.txt", "simple"},
		{"/path/to/file.name.ext", "file.name"},
	}
	for _, tt := range tests {
		got := filenameStem(tt.path)
		if got != tt.want {
			t.Errorf("filenameStem(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestNameFromStem(t *testing.T) {
	tests := []struct {
		stem string
		want string
	}{
		{"backend_api", "Backend Api"},
		{"my-agent-name", "My Agent Name"},
		{"simple", "Simple"},
		{"ALLCAPS", "ALLCAPS"},
		{"mixed_case-name", "Mixed Case Name"},
	}
	for _, tt := range tests {
		got := nameFromStem(tt.stem)
		if got != tt.want {
			t.Errorf("nameFromStem(%q) = %q, want %q", tt.stem, got, tt.want)
		}
	}
}

func TestFullContext(t *testing.T) {
	agent := &AgentDefinition{
		SystemPrompt: "You are a test agent.",
		Skills:       []string{"skill one", "skill two"},
		Rules:        []string{"rule one"},
	}

	ctx := agent.FullContext()

	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	// Should contain prompt, skills section, and rules section
	if !containsStr(ctx, "You are a test agent.") {
		t.Error("context missing system prompt")
	}
	if !containsStr(ctx, "- skill one") {
		t.Error("context missing skills")
	}
	if !containsStr(ctx, "- rule one") {
		t.Error("context missing rules")
	}
}

func TestFullContextNoSkillsOrRules(t *testing.T) {
	agent := &AgentDefinition{SystemPrompt: "Just a prompt."}
	ctx := agent.FullContext()
	if ctx != "Just a prompt." {
		t.Errorf("expected just the prompt, got %q", ctx)
	}
}

func TestWordCount(t *testing.T) {
	agent := &AgentDefinition{SystemPrompt: "one two three four five"}
	if agent.WordCount() != 5 {
		t.Errorf("expected 5 words, got %d", agent.WordCount())
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package loader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentDefinition represents a loaded agent configuration.
type AgentDefinition struct {
	ID             string
	Name           string
	SourcePath     string
	SystemPrompt   string
	Skills         []string
	Rules          []string
	ClaimedDomains []string
	Metadata       map[string]any
	ContentHash    string   // SHA-256 hex of SystemPrompt
	AlsoFoundIn    []string // other source paths with identical content (populated by dedup)
}

// FullContext returns the complete text that defines this agent's behavior.
func (a *AgentDefinition) FullContext() string {
	var b strings.Builder
	b.WriteString(a.SystemPrompt)
	if len(a.Skills) > 0 {
		b.WriteString("\n\nSkills:\n")
		for _, s := range a.Skills {
			b.WriteString("- " + s + "\n")
		}
	}
	if len(a.Rules) > 0 {
		b.WriteString("\n\nRules:\n")
		for _, r := range a.Rules {
			b.WriteString("- " + r + "\n")
		}
	}
	return b.String()
}

// WordCount returns the number of words in the full context.
func (a *AgentDefinition) WordCount() int {
	return len(strings.Fields(a.FullContext()))
}

// LoadAgents loads all agent definitions from a path.
// If path is a file, loads that single agent.
// If path is a directory, recursively finds agent definitions.
func LoadAgents(path string) ([]AgentDefinition, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("agent path not found: %s", path)
	}

	if !info.IsDir() {
		agent, err := loadSingleFile(path)
		if err != nil {
			return nil, err
		}
		if agent == nil {
			return nil, nil
		}
		return []AgentDefinition{*agent}, nil
	}

	var agents []AgentDefinition

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// First pass: directory-based agents
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		agent, err := tryLoadDirectoryAgent(filepath.Join(path, entry.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipped directory %s: %v\n", filepath.Join(path, entry.Name()), err)
			continue
		}
		if agent != nil {
			agents = append(agents, *agent)
		}
	}

	// Second pass: individual files in root
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		if name == "agent-evals.yaml" || name == "agent-evals.yml" {
			continue
		}
		agent, err := loadSingleFile(filepath.Join(path, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipped %s: %v\n", filepath.Join(path, name), err)
			continue
		}
		if agent != nil {
			agents = append(agents, *agent)
		}
	}

	return agents, nil
}

func loadSingleFile(path string) (*AgentDefinition, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return loadYAML(path)
	case ".json":
		return loadJSON(path)
	case ".md", ".txt":
		return loadText(path)
	}
	return nil, nil
}

func loadYAML(path string) (*AgentDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}
	if raw == nil {
		return nil, nil
	}

	systemPrompt := firstString(raw, "system_prompt", "instructions", "prompt", "content")
	if systemPrompt == "" {
		return nil, nil
	}

	stem := filenameStem(path)

	return &AgentDefinition{
		ID:             coalesce(getString(raw, "id"), stem),
		Name:           coalesce(getString(raw, "name"), nameFromStem(stem)),
		SourcePath:     path,
		SystemPrompt:   systemPrompt,
		Skills:         getStringSlice(raw, "skills", "domain_tags"),
		Rules:          getStringSlice(raw, "rules"),
		ClaimedDomains: getStringSlice(raw, "domains", "domain_tags"),
		Metadata:       filterKeys(raw, "system_prompt", "instructions", "prompt", "content", "name", "id", "skills", "rules", "domains", "domain_tags"),
	}, nil
}

func loadJSON(path string) (*AgentDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}
	if raw == nil {
		return nil, nil
	}

	systemPrompt := firstString(raw, "system_prompt", "instructions", "prompt")
	if systemPrompt == "" {
		return nil, nil
	}

	stem := filenameStem(path)

	return &AgentDefinition{
		ID:             coalesce(getString(raw, "id"), stem),
		Name:           coalesce(getString(raw, "name"), nameFromStem(stem)),
		SourcePath:     path,
		SystemPrompt:   systemPrompt,
		Skills:         getStringSlice(raw, "skills"),
		Rules:          getStringSlice(raw, "rules"),
		ClaimedDomains: getStringSlice(raw, "domains"),
	}, nil
}

func loadText(path string) (*AgentDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(data))
	if len(content) < 20 {
		return nil, nil
	}

	stem := filenameStem(path)
	var frontmatter map[string]any

	// Check for YAML frontmatter in markdown
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			var fm map[string]any
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err == nil && fm != nil {
				frontmatter = fm
				content = strings.TrimSpace(parts[2])
			}
		}
	}

	agent := &AgentDefinition{
		ID:           stem,
		Name:         nameFromStem(stem),
		SourcePath:   path,
		SystemPrompt: content,
	}

	if frontmatter != nil {
		agent.Name = coalesce(getString(frontmatter, "name"), agent.Name)
		agent.Skills = getStringSlice(frontmatter, "skills")
		agent.Rules = getStringSlice(frontmatter, "rules")
		agent.ClaimedDomains = getStringSlice(frontmatter, "domains")
		agent.Metadata = frontmatter
	}

	return agent, nil
}

func tryLoadDirectoryAgent(dirPath string) (*AgentDefinition, error) {
	agentFiles := []string{"AGENT.md", "agent.md", "system_prompt.md", "instructions.md",
		"AGENT.txt", "prompt.md", "README.md"}
	skillFiles := []string{"SKILLS.md", "skills.md", "SKILL.md"}
	ruleFiles := []string{"RULES.md", "rules.md", "RULE.md"}

	var systemPrompt string
	for _, name := range agentFiles {
		p := filepath.Join(dirPath, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		systemPrompt = strings.TrimSpace(string(data))
		break
	}

	if systemPrompt == "" {
		return nil, nil
	}

	var skills []string
	for _, name := range skillFiles {
		p := filepath.Join(dirPath, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		skills = extractListItems(string(data))
		break
	}

	var rules []string
	for _, name := range ruleFiles {
		p := filepath.Join(dirPath, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		rules = extractListItems(string(data))
		break
	}

	dirName := filepath.Base(dirPath)

	return &AgentDefinition{
		ID:           dirName,
		Name:         nameFromStem(dirName),
		SourcePath:   dirPath,
		SystemPrompt: systemPrompt,
		Skills:       skills,
		Rules:        rules,
		Metadata:     map[string]any{"format": "directory"},
	}, nil
}

var listItemRe = regexp.MustCompile(`^[-*]\s+(.+)$`)

func extractListItems(text string) []string {
	var items []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		m := listItemRe.FindStringSubmatch(line)
		if len(m) == 2 {
			items = append(items, strings.TrimSpace(m[1]))
		}
	}
	return items
}

// helpers

func filenameStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func nameFromStem(stem string) string {
	s := strings.ReplaceAll(stem, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := getString(m, k); s != "" {
			return s
		}
	}
	return ""
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func getStringSlice(m map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case []any:
			var result []string
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			if len(result) > 0 {
				return result
			}
		case []string:
			if len(val) > 0 {
				return val
			}
		}
	}
	return nil
}

func filterKeys(m map[string]any, exclude ...string) map[string]any {
	ex := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		ex[k] = true
	}
	result := make(map[string]any)
	for k, v := range m {
		if !ex[k] {
			result[k] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// LoadAgentsRecursive walks the directory tree rooted at path, loading agent
// definitions from all supported file types. When dedup is true, agents with
// identical system prompts are collapsed into a single representative with
// AlsoFoundIn populated.
func LoadAgentsRecursive(path string, dedup bool) ([]AgentDefinition, error) {
	absRoot, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("agent path not found: %s", path)
	}
	if !info.IsDir() {
		return LoadAgents(path)
	}

	var allAgents []AgentDefinition

	err = filepath.WalkDir(absRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name == "agent-evals.yaml" || name == "agent-evals.yml" {
			return nil
		}
		agent, loadErr := loadSingleFile(p)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipped %s: %v\n", p, loadErr)
			return nil
		}
		if agent != nil {
			relPath, _ := filepath.Rel(absRoot, p)
			agent.SourcePath = relPath
			agent.ContentHash = computeContentHash(agent.SystemPrompt)
			allAgents = append(allAgents, *agent)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if dedup {
		allAgents = deduplicateAgents(allAgents)
	} else {
		allAgents = qualifyConflictingIDs(allAgents)
	}

	return allAgents, nil
}

func computeContentHash(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(h[:])
}

func deduplicateAgents(agents []AgentDefinition) []AgentDefinition {
	groups := make(map[string][]int) // hash → indices
	var order []string

	for i, a := range agents {
		if _, seen := groups[a.ContentHash]; !seen {
			order = append(order, a.ContentHash)
		}
		groups[a.ContentHash] = append(groups[a.ContentHash], i)
	}

	var result []AgentDefinition
	for _, hash := range order {
		indices := groups[hash]
		rep := agents[indices[0]]
		for _, idx := range indices[1:] {
			rep.AlsoFoundIn = append(rep.AlsoFoundIn, agents[idx].SourcePath)
		}
		result = append(result, rep)
	}

	return qualifyConflictingIDs(result)
}

func qualifyConflictingIDs(agents []AgentDefinition) []AgentDefinition {
	idCount := make(map[string]int)
	for _, a := range agents {
		idCount[a.ID]++
	}

	for i := range agents {
		if idCount[agents[i].ID] > 1 {
			dir := filepath.Dir(agents[i].SourcePath)
			if dir != "." && dir != "" {
				agents[i].ID = filepath.ToSlash(dir) + "/" + agents[i].ID
			}
		}
	}

	return agents
}

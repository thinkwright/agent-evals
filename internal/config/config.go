package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a file path or discovers it alongside agents.
func Load(configPath, agentsPath string) (map[string]any, error) {
	if configPath != "" {
		return loadFile(configPath)
	}

	// Auto-discover alongside agent definitions
	for _, name := range []string{"agent-evals.yaml", "agent-evals.yml"} {
		candidate := filepath.Join(agentsPath, name)
		if _, err := os.Stat(candidate); err == nil {
			return loadFile(candidate)
		}
	}

	return make(map[string]any), nil
}

func loadFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return make(map[string]any), nil
	}
	return result, nil
}

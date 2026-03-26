package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Analysis struct {
		SaveInNamespace bool   `yaml:"saveInNamespace"`
		TargetNamespace string `yaml:"targetNamespace"`
	} `yaml:"analysis"`
	PatternRegistry struct {
		Type             string `yaml:"type"`
		OrganizationName string `yaml:"organizationName"`
		RepositoryBranch string `yaml:"repositoryBranch"`
		RepositoryName   string `yaml:"repositoryName"`
	} `yaml:"patternRegistry"`
}

// Load reads a YAML configuration file from the given path and unmarshals its contents into an AppConfig instance.
func Load(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at %s: %w", path, err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config yaml: %w", err)
	}

	return &cfg, nil
}

package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	SaveInNamespace bool   `yaml:"saveInNamespace"`
	TargetNamespace string `yaml:"targetNamespace"`
}

// NewDefaultConfig applies a default configuration when a custom one is missing
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		SaveInNamespace: true,
		TargetNamespace: "default",
	}
}

// Load reads a YAML configuration file from the given path and unmarshals its contents into an AppConfig instance.
func Load(path string) (*AppConfig, error) {
	cfg := NewDefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Default client-side rate limits for the Kubernetes API. client-go defaults to 5 QPS and
// a burst of 10, which spaces requests 200 ms apart and dominates the run time when many
// Smells are written.
const (
	DefaultClientQPS   float32 = 50
	DefaultClientBurst int     = 100
)

type AppConfig struct {
	SaveInNamespace bool         `yaml:"saveInNamespace"`
	TargetNamespace string       `yaml:"targetNamespace"`
	Client          ClientConfig `yaml:"client"`
}

// ClientConfig holds the client-side rate limits of the Kubernetes API client.
type ClientConfig struct {
	QPS   float32 `yaml:"qps"`
	Burst int     `yaml:"burst"`
}

// RateLimits returns the configured QPS and burst, falling back to the defaults for unset or non-positive values.
func (c ClientConfig) RateLimits() (float32, int) {
	qps, burst := c.QPS, c.Burst
	if qps <= 0 {
		qps = DefaultClientQPS
	}
	if burst <= 0 {
		burst = DefaultClientBurst
	}
	return qps, burst
}

// NewDefaultConfig applies a default configuration when a custom one is missing
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		SaveInNamespace: true,
		TargetNamespace: "default",
		Client: ClientConfig{
			QPS:   DefaultClientQPS,
			Burst: DefaultClientBurst,
		},
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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRateLimits(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ClientConfig
		wantQPS   float32
		wantBurst int
	}{
		{"unset falls back to the defaults", ClientConfig{}, DefaultClientQPS, DefaultClientBurst},
		{"non-positive falls back to the defaults", ClientConfig{QPS: -1, Burst: 0}, DefaultClientQPS, DefaultClientBurst},
		{"configured values are kept", ClientConfig{QPS: 20, Burst: 30}, 20, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			qps, burst := tc.cfg.RateLimits()
			if qps != tc.wantQPS || burst != tc.wantBurst {
				t.Errorf("RateLimits() = (%v, %v), want (%v, %v)", qps, burst, tc.wantQPS, tc.wantBurst)
			}
		})
	}
}

func TestLoadClientConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "saveInNamespace: true\ntargetNamespace: kubepattern\nclient:\n  qps: 20\n  burst: 30\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Client.QPS != 20 || cfg.Client.Burst != 30 {
		t.Errorf("Load() client = %+v, want {QPS:20 Burst:30}", cfg.Client)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Restart != RestartNever {
		t.Errorf("expected restart=never, got %s", cfg.Restart)
	}
	if cfg.GracePeriod != 10*time.Second {
		t.Errorf("expected grace-period=10s, got %v", cfg.GracePeriod)
	}
	if cfg.LogFormat != LogFormatText {
		t.Errorf("expected log-format=text, got %s", cfg.LogFormat)
	}
	if len(cfg.SuccessExitCodes) != 1 || cfg.SuccessExitCodes[0] != 0 {
		t.Errorf("expected success-codes=[0], got %v", cfg.SuccessExitCodes)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := `
timeout: 30s
restart: on-failure
max_retries: 5
grace_period: 15s
health_cmd: "./healthcheck.sh"
health_interval: 10s
health_timeout: 3s
alert:
  - stderr
  - webhook
webhook_url: "https://example.com/hook"
log_format: json
success_codes:
  - 0
  - 143
idle_timeout: 60s
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout=30s, got %v", cfg.Timeout)
	}
	if cfg.Restart != RestartOnFailure {
		t.Errorf("expected restart=on-failure, got %s", cfg.Restart)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected max_retries=5, got %d", cfg.MaxRetries)
	}
	if cfg.GracePeriod != 15*time.Second {
		t.Errorf("expected grace_period=15s, got %v", cfg.GracePeriod)
	}
	if cfg.HealthCmd != "./healthcheck.sh" {
		t.Errorf("expected health_cmd=./healthcheck.sh, got %s", cfg.HealthCmd)
	}
	if cfg.LogFormat != LogFormatJSON {
		t.Errorf("expected log_format=json, got %s", cfg.LogFormat)
	}
	if len(cfg.SuccessExitCodes) != 2 {
		t.Errorf("expected 2 success codes, got %d", len(cfg.SuccessExitCodes))
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("expected idle_timeout=60s, got %v", cfg.IdleTimeout)
	}
}

func TestLoadFromFileInvalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"invalid_timeout", "timeout: invalid"},
		{"invalid_restart", "restart: unknown"},
		{"invalid_log_format", "log_format: xml"},
		{"invalid_yaml", "timeout: ["},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			_, err := LoadFromFile(tmpFile)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestLoadFromFileNotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

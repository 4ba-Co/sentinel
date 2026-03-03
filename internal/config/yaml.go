package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type YAMLConfig struct {
	Timeout        string   `yaml:"timeout"`
	MaxRuntime     string   `yaml:"max_runtime"`
	Restart        string   `yaml:"restart"`
	MaxRetries     int      `yaml:"max_retries"`
	GracePeriod    string   `yaml:"grace_period"`
	HealthCmd      string   `yaml:"health_cmd"`
	HealthInterval string   `yaml:"health_interval"`
	HealthTimeout  string   `yaml:"health_timeout"`
	Alert          []string `yaml:"alert"`
	WebhookURL     string   `yaml:"webhook_url"`
	AlertCmd       string   `yaml:"alert_cmd"`
	AlertOnSuccess bool     `yaml:"alert_on_success"`
	LogFormat      string   `yaml:"log_format"`
	SuccessCodes   []int    `yaml:"success_codes"`
	FailureCodes   []int    `yaml:"failure_codes"`
	IdleTimeout    string   `yaml:"idle_timeout"`
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var yc YAMLConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := Default()

	if yc.Timeout != "" {
		cfg.Timeout, err = time.ParseDuration(yc.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
	}

	if yc.MaxRuntime != "" {
		cfg.MaxRuntime, err = time.ParseDuration(yc.MaxRuntime)
		if err != nil {
			return nil, fmt.Errorf("invalid max_runtime: %w", err)
		}
	}

	if yc.Restart != "" {
		switch yc.Restart {
		case "never":
			cfg.Restart = RestartNever
		case "always":
			cfg.Restart = RestartAlways
		case "on-failure":
			cfg.Restart = RestartOnFailure
		default:
			return nil, fmt.Errorf("invalid restart policy: %s", yc.Restart)
		}
	}

	if yc.MaxRetries > 0 {
		cfg.MaxRetries = yc.MaxRetries
	}

	if yc.GracePeriod != "" {
		cfg.GracePeriod, err = time.ParseDuration(yc.GracePeriod)
		if err != nil {
			return nil, fmt.Errorf("invalid grace_period: %w", err)
		}
	}

	if yc.HealthCmd != "" {
		cfg.HealthCmd = yc.HealthCmd
	}

	if yc.HealthInterval != "" {
		cfg.HealthInterval, err = time.ParseDuration(yc.HealthInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid health_interval: %w", err)
		}
	}

	if yc.HealthTimeout != "" {
		cfg.HealthTimeout, err = time.ParseDuration(yc.HealthTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid health_timeout: %w", err)
		}
	}

	if len(yc.Alert) > 0 {
		cfg.AlertMethods = yc.Alert
	}

	if yc.WebhookURL != "" {
		cfg.WebhookURL = yc.WebhookURL
	}

	if yc.AlertCmd != "" {
		cfg.AlertCmd = yc.AlertCmd
	}

	if yc.AlertOnSuccess {
		cfg.AlertOnSuccess = true
	}

	if yc.LogFormat != "" {
		switch yc.LogFormat {
		case "text":
			cfg.LogFormat = LogFormatText
		case "json":
			cfg.LogFormat = LogFormatJSON
		default:
			return nil, fmt.Errorf("invalid log_format: %s", yc.LogFormat)
		}
	}

	if len(yc.SuccessCodes) > 0 {
		cfg.SuccessExitCodes = yc.SuccessCodes
	}

	if len(yc.FailureCodes) > 0 {
		cfg.FailureExitCodes = yc.FailureCodes
	}

	if yc.IdleTimeout != "" {
		cfg.IdleTimeout, err = time.ParseDuration(yc.IdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid idle_timeout: %w", err)
		}
	}

	return cfg, nil
}

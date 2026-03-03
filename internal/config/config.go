package config

import "time"

type RestartPolicy string

const (
	RestartNever     RestartPolicy = "never"
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
)

type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

type Config struct {
	// Command to execute (after --)
	Command []string

	// Timeout for command execution (0 = no timeout)
	Timeout time.Duration

	// Maximum runtime before forced termination
	MaxRuntime time.Duration

	// Restart policy: never, always, on-failure
	Restart RestartPolicy

	// Maximum retry attempts (0 = unlimited when restart enabled)
	MaxRetries int

	// Grace period for graceful shutdown
	GracePeriod time.Duration

	// Health check command
	HealthCmd string

	// Health check interval
	HealthInterval time.Duration

	// Health check timeout
	HealthTimeout time.Duration

	// Alert methods: stderr, webhook, script
	AlertMethods []string

	// Webhook URL for alerts
	WebhookURL string

	// Custom alert script
	AlertCmd string

	// Whether to send alert on successful exit
	AlertOnSuccess bool

	// Log format: text, json
	LogFormat LogFormat

	// Exit codes to treat as success
	SuccessExitCodes []int

	// Exit codes to treat as failure (trigger restart)
	FailureExitCodes []int

	// Idle timeout (no output)
	IdleTimeout time.Duration

	// Config file path
	ConfigFile string
}

func Default() *Config {
	return &Config{
		Restart:        RestartNever,
		MaxRetries:     0,
		GracePeriod:    10 * time.Second,
		HealthInterval: 30 * time.Second,
		HealthTimeout:  5 * time.Second,
		AlertMethods:   []string{"stderr"},
		LogFormat:      LogFormatText,
		SuccessExitCodes: []int{0},
	}
}

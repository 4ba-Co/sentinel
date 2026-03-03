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

	// Alert delivery methods: stderr, webhook, script
	AlertMethods []string

	// Alert event types to subscribe to.
	// Default: started, exited, timeout, restart, health_failed, killed.
	// Use --alert-events to override, or --alert-on-success to append "success".
	AlertEvents []string

	// Webhook URL for alerts
	WebhookURL string

	// Custom alert script
	AlertCmd string

	// Whether to send alert on successful exit (convenience flag).
	// When true, appends "success" to AlertEvents.
	// Ignored when AlertEvents is explicitly set via --alert-events.
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

// DefaultAlertEvents returns the default set of alert events.
// This does NOT include "success" — use --alert-on-success or --alert-events to add it.
func DefaultAlertEvents() []string {
	return []string{"started", "exited", "timeout", "restart", "health_failed", "killed"}
}

func Default() *Config {
	return &Config{
		Restart:          RestartNever,
		MaxRetries:       0,
		GracePeriod:      10 * time.Second,
		HealthInterval:   30 * time.Second,
		HealthTimeout:    5 * time.Second,
		AlertMethods:     []string{"stderr"},
		AlertEvents:      DefaultAlertEvents(),
		LogFormat:        LogFormatText,
		SuccessExitCodes: []int{0},
	}
}

// ShouldAlert returns true if the given event type is in the AlertEvents list.
func (c *Config) ShouldAlert(eventType string) bool {
	for _, e := range c.AlertEvents {
		if e == eventType {
			return true
		}
	}
	return false
}

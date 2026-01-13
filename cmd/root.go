package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
)

func ParseArgs() (*config.Config, error) {
	var cfg *config.Config

	fs := flag.NewFlagSet("sentinel", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	// Find -- separator
	args := os.Args[1:]
	dashIdx := -1
	for i, arg := range args {
		if arg == "--" {
			dashIdx = i
			break
		}
	}

	var flagArgs, cmdArgs []string
	if dashIdx >= 0 {
		flagArgs = args[:dashIdx]
		cmdArgs = args[dashIdx+1:]
	} else {
		flagArgs = args
	}

	// Define flags
	var timeout, maxRuntime, gracePeriod, healthInterval, healthTimeout, idleTimeout string
	var restart, logFormat, alertMethods, successCodes, failureCodes string
	var maxRetries int
	var healthCmd, webhookURL, alertCmd, configFile string

	fs.StringVar(&timeout, "timeout", "", "Execution timeout (e.g., 10m, 1h)")
	fs.StringVar(&maxRuntime, "max-runtime", "", "Maximum runtime before termination")
	fs.StringVar(&restart, "restart", "never", "Restart policy: never, always, on-failure")
	fs.IntVar(&maxRetries, "max-retries", 0, "Maximum restart attempts (0 = unlimited)")
	fs.StringVar(&gracePeriod, "grace-period", "10s", "Grace period for graceful shutdown")
	fs.StringVar(&healthCmd, "health-cmd", "", "Health check command")
	fs.StringVar(&healthInterval, "health-interval", "30s", "Health check interval")
	fs.StringVar(&healthTimeout, "health-timeout", "5s", "Health check timeout")
	fs.StringVar(&alertMethods, "alert", "stderr", "Alert methods (comma-separated): stderr,webhook,script")
	fs.StringVar(&webhookURL, "webhook-url", "", "Webhook URL for alerts")
	fs.StringVar(&alertCmd, "alert-cmd", "", "Custom alert script")
	fs.StringVar(&logFormat, "log-format", "text", "Log format: text, json")
	fs.StringVar(&successCodes, "success-codes", "0", "Exit codes to treat as success (comma-separated)")
	fs.StringVar(&failureCodes, "failure-codes", "", "Exit codes to treat as failure (comma-separated)")
	fs.StringVar(&idleTimeout, "idle-timeout", "", "Timeout when no output received")
	fs.StringVar(&configFile, "config", "", "Config file path (YAML)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sentinel [flags] -- command [args...]\n\n")
		fmt.Fprintf(os.Stderr, "A universal process wrapper for monitoring and supervision.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  sentinel --timeout 10m -- ./my_job\n")
		fmt.Fprintf(os.Stderr, "  sentinel --restart on-failure --max-retries 5 -- ./my_server\n")
		fmt.Fprintf(os.Stderr, "  sentinel --alert webhook --webhook-url http://example.com/hook -- ./task\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}

	// Validate command
	if len(cmdArgs) == 0 {
		return nil, fmt.Errorf("no command specified, use: sentinel [flags] -- command [args...]")
	}

	// Load config file first if specified
	var err error
	if configFile != "" {
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg = config.Default()
	}

	cfg.Command = cmdArgs
	cfg.ConfigFile = configFile

	// Override with CLI flags (CLI takes precedence)
	if timeout != "" {
		cfg.Timeout, err = time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid --timeout: %w", err)
		}
	}
	if maxRuntime != "" {
		cfg.MaxRuntime, err = time.ParseDuration(maxRuntime)
		if err != nil {
			return nil, fmt.Errorf("invalid --max-runtime: %w", err)
		}
	}
	cfg.GracePeriod, err = time.ParseDuration(gracePeriod)
	if err != nil {
		return nil, fmt.Errorf("invalid --grace-period: %w", err)
	}
	cfg.HealthInterval, err = time.ParseDuration(healthInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid --health-interval: %w", err)
	}
	cfg.HealthTimeout, err = time.ParseDuration(healthTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid --health-timeout: %w", err)
	}
	if idleTimeout != "" {
		cfg.IdleTimeout, err = time.ParseDuration(idleTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid --idle-timeout: %w", err)
		}
	}

	// Parse restart policy
	if restart != "never" || configFile == "" {
		switch restart {
		case "never":
			cfg.Restart = config.RestartNever
		case "always":
			cfg.Restart = config.RestartAlways
		case "on-failure":
			cfg.Restart = config.RestartOnFailure
		default:
			return nil, fmt.Errorf("invalid --restart: %s (use: never, always, on-failure)", restart)
		}
	}

	if maxRetries > 0 {
		cfg.MaxRetries = maxRetries
	}

	if healthCmd != "" {
		cfg.HealthCmd = healthCmd
	}

	if webhookURL != "" {
		cfg.WebhookURL = webhookURL
	}

	if alertCmd != "" {
		cfg.AlertCmd = alertCmd
	}

	// Parse log format
	if logFormat != "text" || configFile == "" {
		switch logFormat {
		case "text":
			cfg.LogFormat = config.LogFormatText
		case "json":
			cfg.LogFormat = config.LogFormatJSON
		default:
			return nil, fmt.Errorf("invalid --log-format: %s (use: text, json)", logFormat)
		}
	}

	// Parse alert methods
	if alertMethods != "stderr" || configFile == "" {
		cfg.AlertMethods = strings.Split(alertMethods, ",")
		for i := range cfg.AlertMethods {
			cfg.AlertMethods[i] = strings.TrimSpace(cfg.AlertMethods[i])
		}
	}

	// Parse exit codes
	if successCodes != "0" || configFile == "" {
		cfg.SuccessExitCodes, err = parseIntList(successCodes)
		if err != nil {
			return nil, fmt.Errorf("invalid --success-codes: %w", err)
		}
	}
	if failureCodes != "" {
		cfg.FailureExitCodes, err = parseIntList(failureCodes)
		if err != nil {
			return nil, fmt.Errorf("invalid --failure-codes: %w", err)
		}
	}

	return cfg, nil
}

func parseIntList(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return nil, fmt.Errorf("invalid number: %s", p)
		}
		result = append(result, n)
	}
	return result, nil
}

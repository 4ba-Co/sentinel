package cmd

import (
	"os"
	"testing"
)

func TestParseArgsBasic(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--timeout", "5s", "--", "echo", "hello"}

	cfg, err := ParseArgs()
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if cfg.Timeout.Seconds() != 5 {
		t.Errorf("expected timeout=5s, got %v", cfg.Timeout)
	}
	if len(cfg.Command) != 2 || cfg.Command[0] != "echo" {
		t.Errorf("expected command=[echo hello], got %v", cfg.Command)
	}
}

func TestParseArgsRestartPolicy(t *testing.T) {
	tests := []struct {
		arg      string
		expected string
	}{
		{"never", "never"},
		{"always", "always"},
		{"on-failure", "on-failure"},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			os.Args = []string{"sentinel", "--restart", tt.arg, "--", "echo"}

			cfg, err := ParseArgs()
			if err != nil {
				t.Fatalf("ParseArgs failed: %v", err)
			}

			if string(cfg.Restart) != tt.expected {
				t.Errorf("expected restart=%s, got %s", tt.expected, cfg.Restart)
			}
		})
	}
}

func TestParseArgsNoCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--timeout", "5s"}

	_, err := ParseArgs()
	if err == nil {
		t.Error("expected error when no command specified")
	}
}

func TestParseArgsInvalidTimeout(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--timeout", "invalid", "--", "echo"}

	_, err := ParseArgs()
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestParseArgsInvalidRestart(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--restart", "unknown", "--", "echo"}

	_, err := ParseArgs()
	if err == nil {
		t.Error("expected error for invalid restart policy")
	}
}

func TestParseArgsLogFormat(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--log-format", "json", "--", "echo"}

	cfg, err := ParseArgs()
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if cfg.LogFormat != "json" {
		t.Errorf("expected log-format=json, got %s", cfg.LogFormat)
	}
}

func TestParseArgsSuccessCodes(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--success-codes", "0,1,143", "--", "echo"}

	cfg, err := ParseArgs()
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if len(cfg.SuccessExitCodes) != 3 {
		t.Errorf("expected 3 success codes, got %d", len(cfg.SuccessExitCodes))
	}
}

func TestParseArgsAlertMethods(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sentinel", "--alert", "stderr,webhook,script", "--", "echo"}

	cfg, err := ParseArgs()
	if err != nil {
		t.Fatalf("ParseArgs failed: %v", err)
	}

	if len(cfg.AlertMethods) != 3 {
		t.Errorf("expected 3 alert methods, got %d", len(cfg.AlertMethods))
	}
}

func TestParseIntList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
		hasError bool
	}{
		{"0", []int{0}, false},
		{"0,1,2", []int{0, 1, 2}, false},
		{"", nil, false},
		{"invalid", nil, true},
		{"0,invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseIntList(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

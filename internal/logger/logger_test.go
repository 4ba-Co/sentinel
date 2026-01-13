package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetJSONMode(false)

	Info("test message %d", 123)

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "test message 123") {
		t.Errorf("expected 'test message 123' in output, got: %s", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	SetJSONMode(true)
	defer SetJSONMode(false)

	Info("json test")

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("expected level=INFO, got %s", entry.Level)
	}
	if entry.Message != "json test" {
		t.Errorf("expected message='json test', got %s", entry.Message)
	}
	if entry.Time == "" {
		t.Error("expected time to be set")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		fn    func(string, ...interface{})
		level string
	}{
		{Debug, "DEBUG"},
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			SetOutput(&buf)
			SetJSONMode(false)

			tt.fn("test")

			if !strings.Contains(buf.String(), "["+tt.level+"]") {
				t.Errorf("expected [%s] in output, got: %s", tt.level, buf.String())
			}
		})
	}
}

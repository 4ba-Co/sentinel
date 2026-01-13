package alert

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

func init() {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
}

func TestAlerterStderr(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	cfg := config.Default()
	cfg.AlertMethods = []string{"stderr"}
	cfg.Command = []string{"test", "cmd"}

	a := New(cfg)
	a.Send(Event{
		Type:     EventExited,
		ExitCode: 1,
		Message:  "test message",
	})

	output := buf.String()
	if !strings.Contains(output, "ALERT") {
		t.Errorf("expected ALERT in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in output, got: %s", output)
	}
}

func TestAlerterWebhook(t *testing.T) {
	var receivedEvent Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json")
		}
		json.NewDecoder(r.Body).Decode(&receivedEvent)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.AlertMethods = []string{"webhook"}
	cfg.WebhookURL = server.URL
	cfg.Command = []string{"test", "cmd"}

	a := New(cfg)
	a.Send(Event{
		Type:     EventTimeout,
		ExitCode: 124,
		Message:  "timeout occurred",
	})

	if receivedEvent.Type != EventTimeout {
		t.Errorf("expected type=timeout, got %s", receivedEvent.Type)
	}
	if receivedEvent.ExitCode != 124 {
		t.Errorf("expected exit_code=124, got %d", receivedEvent.ExitCode)
	}
}

func TestAlerterScript(t *testing.T) {
	tmpFile := t.TempDir() + "/alert_output.txt"

	cfg := config.Default()
	cfg.AlertMethods = []string{"script"}
	cfg.AlertCmd = "echo $SENTINEL_EVENT > " + tmpFile
	cfg.Command = []string{"test", "cmd"}

	a := New(cfg)
	a.Send(Event{
		Type:    EventStarted,
		Message: "process started",
	})

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read alert output: %v", err)
	}

	if !strings.Contains(string(content), "started") {
		t.Errorf("expected 'started' in output, got: %s", content)
	}
}

func TestAlerterWebhookNoURL(t *testing.T) {
	cfg := config.Default()
	cfg.AlertMethods = []string{"webhook"}
	cfg.WebhookURL = ""
	cfg.Command = []string{"test"}

	a := New(cfg)
	a.Send(Event{Type: EventExited})
}

func TestAlerterScriptNoCmd(t *testing.T) {
	cfg := config.Default()
	cfg.AlertMethods = []string{"script"}
	cfg.AlertCmd = ""
	cfg.Command = []string{"test"}

	a := New(cfg)
	a.Send(Event{Type: EventExited})
}

func TestEventTypes(t *testing.T) {
	events := []EventType{
		EventStarted,
		EventExited,
		EventTimeout,
		EventRestart,
		EventHealthFail,
		EventKilled,
	}

	for _, e := range events {
		if e == "" {
			t.Errorf("event type should not be empty")
		}
	}
}

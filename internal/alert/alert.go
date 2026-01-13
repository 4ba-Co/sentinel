package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

type EventType string

const (
	EventStarted  EventType = "started"
	EventExited   EventType = "exited"
	EventTimeout  EventType = "timeout"
	EventRestart  EventType = "restart"
	EventHealthFail EventType = "health_failed"
	EventKilled   EventType = "killed"
)

type Event struct {
	Type      EventType `json:"type"`
	Command   []string  `json:"command"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Hostname  string    `json:"hostname"`
}

type Alerter struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Alerter {
	return &Alerter{cfg: cfg}
}

func (a *Alerter) Send(event Event) {
	event.Timestamp = time.Now()
	event.Hostname, _ = os.Hostname()
	event.Command = a.cfg.Command

	for _, method := range a.cfg.AlertMethods {
		switch strings.TrimSpace(method) {
		case "stderr":
			a.sendStderr(event)
		case "webhook":
			a.sendWebhook(event)
		case "script":
			a.sendScript(event)
		}
	}
}

func (a *Alerter) sendStderr(event Event) {
	logger.Warn("[ALERT] %s: %s (exit_code=%d, command=%v)",
		event.Type, event.Message, event.ExitCode, event.Command)
}

func (a *Alerter) sendWebhook(event Event) {
	if a.cfg.WebhookURL == "" {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		logger.Error("failed to marshal webhook payload: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.WebhookURL, bytes.NewReader(data))
	if err != nil {
		logger.Error("failed to create webhook request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("failed to send webhook: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logger.Error("webhook returned status %d", resp.StatusCode)
	} else {
		logger.Debug("webhook sent successfully")
	}
}

func (a *Alerter) sendScript(event Event) {
	if a.cfg.AlertCmd == "" {
		return
	}

	cmd := exec.Command("sh", "-c", a.cfg.AlertCmd)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SENTINEL_EVENT=%s", event.Type),
		fmt.Sprintf("SENTINEL_EXIT_CODE=%d", event.ExitCode),
		fmt.Sprintf("SENTINEL_MESSAGE=%s", event.Message),
		fmt.Sprintf("SENTINEL_COMMAND=%s", strings.Join(event.Command, " ")),
		fmt.Sprintf("SENTINEL_HOSTNAME=%s", event.Hostname),
	)

	if err := cmd.Run(); err != nil {
		logger.Error("alert script failed: %v", err)
	} else {
		logger.Debug("alert script executed successfully")
	}
}

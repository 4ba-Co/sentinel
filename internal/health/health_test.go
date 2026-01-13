package health

import (
	"bytes"
	"testing"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

func init() {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
}

func TestCheckerNoHealthCmd(t *testing.T) {
	cfg := config.Default()
	cfg.HealthCmd = ""

	c := NewChecker(cfg)
	c.Start()
	defer c.Stop()

	select {
	case <-c.Failed():
		t.Error("should not fail when no health cmd")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCheckerHealthy(t *testing.T) {
	cfg := config.Default()
	cfg.HealthCmd = "exit 0"
	cfg.HealthInterval = 50 * time.Millisecond
	cfg.HealthTimeout = 1 * time.Second

	c := NewChecker(cfg)
	c.Start()
	defer c.Stop()

	select {
	case <-c.Failed():
		t.Error("should not fail with exit 0")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestCheckerUnhealthy(t *testing.T) {
	cfg := config.Default()
	cfg.HealthCmd = "exit 1"
	cfg.HealthInterval = 50 * time.Millisecond
	cfg.HealthTimeout = 1 * time.Second

	c := NewChecker(cfg)
	c.Start()
	defer c.Stop()

	select {
	case <-c.Failed():
	case <-time.After(500 * time.Millisecond):
		t.Error("should fail with exit 1")
	}
}

func TestCheckerStop(t *testing.T) {
	cfg := config.Default()
	cfg.HealthCmd = "exit 1"
	cfg.HealthInterval = 50 * time.Millisecond
	cfg.HealthTimeout = 1 * time.Second

	c := NewChecker(cfg)
	c.Start()
	c.Stop()

	select {
	case <-c.Failed():
		t.Error("should not fail after stop")
	case <-time.After(200 * time.Millisecond):
	}
}

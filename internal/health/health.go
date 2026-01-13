package health

import (
	"context"
	"os/exec"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

type Checker struct {
	cfg      *config.Config
	stopCh   chan struct{}
	failedCh chan struct{}
}

func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		cfg:      cfg,
		stopCh:   make(chan struct{}),
		failedCh: make(chan struct{}),
	}
}

func (c *Checker) Start() {
	if c.cfg.HealthCmd == "" {
		return
	}

	go c.loop()
}

func (c *Checker) loop() {
	ticker := time.NewTicker(c.cfg.HealthInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxFailures := 3

	for {
		select {
		case <-ticker.C:
			if c.check() {
				consecutiveFailures = 0
			} else {
				consecutiveFailures++
				logger.Warn("health check failed (%d/%d)", consecutiveFailures, maxFailures)
				
				if consecutiveFailures >= maxFailures {
					logger.Error("health check threshold exceeded")
					close(c.failedCh)
					return
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *Checker) check() bool {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.HealthTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", c.cfg.HealthCmd)
	err := cmd.Run()

	if err != nil {
		logger.Debug("health check error: %v", err)
		return false
	}
	return true
}

func (c *Checker) Stop() {
	close(c.stopCh)
}

func (c *Checker) Failed() <-chan struct{} {
	return c.failedCh
}

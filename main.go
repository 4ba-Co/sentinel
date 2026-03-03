package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/4ba-Co/sentinel/cmd"
	"github.com/4ba-Co/sentinel/internal/alert"
	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/health"
	"github.com/4ba-Co/sentinel/internal/logger"
	"github.com/4ba-Co/sentinel/internal/runner"
	"github.com/4ba-Co/sentinel/internal/signal"
)

func main() {
	cfg, err := cmd.ParseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.LogFormat == config.LogFormatJSON {
		logger.SetJSONMode(true)
	}

	logger.Info("sentinel starting, command: %v", cfg.Command)

	// Setup zombie reaper only in PID 1 mode
	if signal.IsPID1() {
		logger.Info("running as PID 1, enabling init mode")
		signal.SetupReaper()
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

func run(cfg *config.Config) int {
	var (
		terminated bool
		attempts   int
		exitCode   int
	)

	alerter := alert.New(cfg)

	sigHandler := signal.NewHandler(func(sig os.Signal) {
		terminated = true
	})
	sigHandler.Start()
	defer sigHandler.Stop()

	for {
		attempts++
		code, reason := runOnce(cfg, &terminated, alerter)
		exitCode = code

		if terminated {
			logger.Info("terminated by signal")
			alerter.Send(alert.Event{
				Type:     alert.EventKilled,
				ExitCode: exitCode,
				Message:  "terminated by signal",
			})
			return exitCode
		}

		isSuccess := isSuccessCode(cfg, exitCode)

		// Send exit alert (filtered by AlertEvents in alerter.Send)
		if !isSuccess {
			alerter.Send(alert.Event{
				Type:     alert.EventExited,
				ExitCode: exitCode,
				Message:  fmt.Sprintf("command exited with code %d (%s)", exitCode, reason),
			})
		} else {
			alerter.Send(alert.Event{
				Type:     alert.EventSuccess,
				ExitCode: exitCode,
				Message:  fmt.Sprintf("command completed successfully (exit code %d)", exitCode),
			})
		}

		// Determine if we should restart
		shouldRestart := false
		switch cfg.Restart {
		case config.RestartAlways:
			shouldRestart = true
		case config.RestartOnFailure:
			shouldRestart = !isSuccess
		case config.RestartNever:
			shouldRestart = false
		}

		if !shouldRestart {
			if isSuccess {
				logger.Info("command completed successfully")
			} else {
				logger.Warn("command failed with exit code %d", exitCode)
			}
			return exitCode
		}

		// Check max retries
		if cfg.MaxRetries > 0 && attempts >= cfg.MaxRetries {
			logger.Error("max retries (%d) exceeded", cfg.MaxRetries)
			return exitCode
		}

		// Backoff before restart
		backoff := calculateBackoff(attempts)
		logger.Info("restarting in %v (attempt %d)", backoff, attempts)
		alerter.Send(alert.Event{
			Type:    alert.EventRestart,
			Message: fmt.Sprintf("restarting in %v (attempt %d)", backoff, attempts),
		})

		select {
		case <-time.After(backoff):
		case <-sigHandler.Done():
			terminated = true
			return exitCode
		}
	}
}

func runOnce(cfg *config.Config, terminated *bool, alerter *alert.Alerter) (int, string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, cfg.Timeout)
		defer timeoutCancel()
	}

	r := runner.New(cfg)

	// Local signal handler for this run
	localSigHandler := signal.NewHandler(func(sig os.Signal) {
		*terminated = true
		r.Signal(sig)

		go func() {
			time.Sleep(cfg.GracePeriod)
			if r.IsRunning() {
				logger.Warn("grace period exceeded, forcing kill")
				r.Kill()
			}
		}()
	})
	localSigHandler.Start()
	defer localSigHandler.Stop()

	// Health checker
	healthChecker := health.NewChecker(cfg)
	healthChecker.Start()
	defer healthChecker.Stop()

	if err := r.Start(ctx); err != nil {
		logger.Error("failed to start command: %v", err)
		return 1, "start_failed"
	}

	alerter.Send(alert.Event{
		Type:    alert.EventStarted,
		Message: "process started",
	})

	// Wait for completion or health failure or idle timeout
	done := make(chan struct{})
	idleKill := make(chan struct{})
	var exitCode int
	var waitErr error

	go func() {
		exitCode, waitErr = r.Wait()
		close(done)
	}()

	// Idle timeout monitor
	if cfg.IdleTimeout > 0 {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if r.IdleDuration() > cfg.IdleTimeout {
						close(idleKill)
						return
					}
				case <-done:
					return
				}
			}
		}()
	}

	select {
	case <-done:
		if waitErr != nil {
			logger.Error("error waiting for process: %v", waitErr)
			return 1, "wait_error"
		}
	case <-healthChecker.Failed():
		logger.Error("health check failed, killing process")
		alerter.Send(alert.Event{
			Type:    alert.EventHealthFail,
			Message: "health check failed",
		})
		r.Kill()
		<-done
		return 1, "health_failed"
	case <-idleKill:
		logger.Error("idle timeout exceeded (%v), killing process", cfg.IdleTimeout)
		alerter.Send(alert.Event{
			Type:    alert.EventTimeout,
			Message: fmt.Sprintf("idle timeout exceeded (%v)", cfg.IdleTimeout),
		})
		r.Kill()
		<-done
		return 1, "idle_timeout"
	}

	if ctx.Err() == context.DeadlineExceeded && !*terminated {
		logger.Warn("command timed out after %v", cfg.Timeout)
		alerter.Send(alert.Event{
			Type:    alert.EventTimeout,
			Message: fmt.Sprintf("command timed out after %v", cfg.Timeout),
		})
		r.Kill()
		return 124, "timeout"
	}

	return exitCode, "exited"
}

func isSuccessCode(cfg *config.Config, code int) bool {
	for _, c := range cfg.SuccessExitCodes {
		if code == c {
			return true
		}
	}
	return false
}

func calculateBackoff(attempt int) time.Duration {
	base := time.Second
	max := 30 * time.Second
	
	backoff := base * time.Duration(1<<uint(attempt-1))
	if backoff > max {
		backoff = max
	}
	return backoff
}

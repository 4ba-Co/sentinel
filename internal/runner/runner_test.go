package runner

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

func init() {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
}

func TestRunnerBasicExecution(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"echo", "hello"}

	r := New(cfg)
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	exitCode, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestRunnerExitCode(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sh", "-c", "exit 42"}

	r := New(cfg)
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	exitCode, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

func TestRunnerContextCancel(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sleep", "10"}

	r := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
}

func TestRunnerIsRunning(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sleep", "1"}

	r := New(cfg)
	ctx := context.Background()

	if r.IsRunning() {
		t.Error("expected IsRunning=false before start")
	}

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !r.IsRunning() {
		t.Error("expected IsRunning=true after start")
	}

	r.Wait()

	if r.IsRunning() {
		t.Error("expected IsRunning=false after wait")
	}
}

func TestRunnerPid(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sleep", "1"}

	r := New(cfg)
	ctx := context.Background()

	if r.Pid() != -1 {
		t.Error("expected Pid=-1 before start")
	}

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if r.Pid() <= 0 {
		t.Errorf("expected Pid>0 after start, got %d", r.Pid())
	}

	r.Kill()
	r.Wait()
}

func TestRunnerSignal(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sleep", "10"}

	r := New(cfg)
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		r.Kill()
	}()

	r.Wait()

	if r.IsRunning() {
		t.Error("expected process to be killed")
	}
}

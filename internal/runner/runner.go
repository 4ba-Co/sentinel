package runner

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

type Runner struct {
	cfg          *config.Config
	cmd          *exec.Cmd
	mu           sync.Mutex
	running      bool
	done         chan struct{}
	stdoutWriter *ActivityWriter
	stderrWriter *ActivityWriter
}

func New(cfg *config.Config) *Runner {
	return &Runner{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	cmd := exec.CommandContext(ctx, r.cfg.Command[0], r.cfg.Command[1:]...)

	cmd.Stdin = os.Stdin

	// Wrap stdout/stderr for idle detection
	r.stdoutWriter = NewActivityWriter(os.Stdout)
	r.stderrWriter = NewActivityWriter(os.Stderr)
	cmd.Stdout = r.stdoutWriter
	cmd.Stderr = r.stderrWriter

	// Create new process group for proper signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	logger.Info("starting command: %v", r.cfg.Command)

	if err := cmd.Start(); err != nil {
		return err
	}

	r.cmd = cmd
	r.running = true

	logger.Info("process started, pid=%d", cmd.Process.Pid)

	return nil
}

func (r *Runner) Wait() (int, error) {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()

	if cmd == nil {
		return -1, nil
	}

	err := cmd.Wait()

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, err
		}
	}

	logger.Info("process exited, code=%d", exitCode)
	return exitCode, nil
}

func (r *Runner) Signal(sig os.Signal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	logger.Info("forwarding signal %v to process group", sig)
	
	// Send signal to entire process group
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		return r.cmd.Process.Signal(sig)
	}
	
	return syscall.Kill(-pgid, sig.(syscall.Signal))
}

func (r *Runner) Kill() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	logger.Warn("killing process group")
	
	pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
	if err != nil {
		return r.cmd.Process.Kill()
	}
	
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) Pid() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Pid
	}
	return -1
}

func (r *Runner) IdleDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stdoutWriter == nil || r.stderrWriter == nil {
		return 0
	}

	stdoutIdle := r.stdoutWriter.IdleDuration()
	stderrIdle := r.stderrWriter.IdleDuration()

	if stdoutIdle < stderrIdle {
		return stdoutIdle
	}
	return stderrIdle
}

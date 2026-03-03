package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
	"github.com/4ba-Co/sentinel/internal/signal"
)

// shellMetaChars contains characters that require shell interpretation.
const shellMetaChars = "|&;<>()$\"' \t\n*?[#~" + "`"

// shellOperators are multi-character shell constructs that cannot work
// without a real shell interpreter (simple string splitting is not enough).
var shellOperators = []string{"&&", "||", ";;", "|", ">>", "<<", "<(", ">("}

// shellBuiltins are commands that only exist inside a shell.
var shellBuiltins = []string{"cd", "source", ".", "export", "eval", "exec", "alias", "unalias", "set", "unset", "read", "trap", "ulimit", "umask"}

// candidateShells is the ordered list of shells to try when a shell is needed.
var candidateShells = []string{"sh", "bash", "dash", "ash"}

// needsShell reports whether the command string contains shell metacharacters
// and should be executed via a shell.
func needsShell(command []string) bool {
	if len(command) != 1 {
		return false
	}
	return strings.ContainsAny(command[0], shellMetaChars)
}

// hasShellOperators reports whether the command string contains shell operators
// (&&, ||, |, etc.) that require a real shell and cannot be handled by simple
// string splitting.
func hasShellOperators(cmd string) bool {
	for _, op := range shellOperators {
		if strings.Contains(cmd, op) {
			return true
		}
	}
	// Check if the command starts with a shell builtin
	fields := strings.Fields(cmd)
	if len(fields) > 0 {
		for _, b := range shellBuiltins {
			if fields[0] == b {
				return true
			}
		}
	}
	return false
}

// FindShell returns the path to an available shell, or empty string if none found.
func FindShell() string {
	for _, sh := range candidateShells {
		if path, err := exec.LookPath(sh); err == nil {
			return path
		}
	}
	return ""
}

// buildCommand constructs an *exec.Cmd from a command slice, automatically
// detecting whether a shell is needed and falling back to string splitting
// when no shell is available.
func buildCommand(ctx context.Context, command []string) (*exec.Cmd, error) {
	if !needsShell(command) {
		// Multi-element command or single element without metacharacters:
		// execute directly.
		return exec.CommandContext(ctx, command[0], command[1:]...), nil
	}

	// Single-element command with shell metacharacters.
	shell := FindShell()
	if shell != "" {
		logger.Info("detected shell command, using %s -c", shell)
		return exec.CommandContext(ctx, shell, "-c", command[0]), nil
	}

	// No shell available — try to degrade gracefully.
	cmdStr := command[0]
	if hasShellOperators(cmdStr) {
		return nil, fmt.Errorf(
			"command requires a shell (contains operators like &&, |, etc.) "+
				"but no shell found in PATH (tried: %s)",
			strings.Join(candidateShells, ", "),
		)
	}

	// Simple space-separated command without shell operators:
	// split on whitespace and exec directly.
	logger.Warn("no shell found, falling back to whitespace splitting for command")
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return exec.CommandContext(ctx, fields[0], fields[1:]...), nil
}

type Runner struct {
	cfg          *config.Config
	cmd          *exec.Cmd
	mu           sync.Mutex
	running      bool
	done         chan struct{}
	stdoutWriter *ActivityWriter
	stderrWriter *ActivityWriter
	reapCh       <-chan signal.ReapResult
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

	cmd, err := buildCommand(ctx, r.cfg.Command)
	if err != nil {
		return err
	}

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
	r.reapCh = signal.TrackPID(cmd.Process.Pid)

	logger.Info("process started, pid=%d", cmd.Process.Pid)

	return nil
}

func (r *Runner) Wait() (int, error) {
	r.mu.Lock()
	cmd := r.cmd
	reapCh := r.reapCh
	r.mu.Unlock()

	if cmd == nil {
		return -1, nil
	}

	defer func() {
		if cmd.Process != nil {
			signal.UntrackPID(cmd.Process.Pid)
		}
	}()

	err := cmd.Wait()

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// cmd.Wait() failed (e.g. zombie reaper already collected this PID).
			// Check if the reaper has forwarded the exit status.
			if reapCh != nil {
				select {
				case result := <-reapCh:
					logger.Info("process exited, code=%d (collected by reaper)", result.ExitStatus)
					return result.ExitStatus, nil
				default:
				}
			}
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

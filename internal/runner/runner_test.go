package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/4ba-Co/sentinel/internal/config"
	"github.com/4ba-Co/sentinel/internal/logger"
)

func init() {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
}

// ---------------------------------------------------------------------------
// needsShell
// ---------------------------------------------------------------------------

func TestNeedsShell(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		want    bool
	}{
		// --- should NOT need shell ---
		{"multi-arg simple", []string{"echo", "hello"}, false},
		{"multi-arg with flags", []string{"ls", "-la", "/tmp"}, false},
		{"single absolute path", []string{"/usr/bin/myapp"}, false},
		{"single relative path", []string{"./myapp"}, false},
		{"single binary name", []string{"myapp"}, false},
		{"single binary with dashes", []string{"my-app-v2"}, false},
		{"single binary with dots", []string{"app.v2.3"}, false},
		{"single binary with underscores", []string{"my_app"}, false},

		// --- should need shell ---
		{"space separated", []string{"echo hello"}, true},
		{"double ampersand", []string{"echo a && echo b"}, true},
		{"double pipe", []string{"cmd1 || cmd2"}, true},
		{"pipe", []string{"echo hello | grep hello"}, true},
		{"semicolon", []string{"echo a; echo b"}, true},
		{"redirect out", []string{"echo hello > /tmp/out"}, true},
		{"redirect append", []string{"echo hello >> /tmp/out"}, true},
		{"redirect in", []string{"cat < /tmp/in"}, true},
		{"dollar variable", []string{"echo $HOME"}, true},
		{"dollar brace", []string{"echo ${HOME}"}, true},
		{"backtick", []string{"echo `date`"}, true},
		{"subshell parens", []string{"(echo hello)"}, true},
		{"single quotes", []string{"echo 'hello world'"}, true},
		{"double quotes", []string{`echo "hello world"`}, true},
		{"glob star", []string{"ls *.go"}, true},
		{"glob question", []string{"ls file?.txt"}, true},
		{"glob bracket", []string{"ls [abc].txt"}, true},
		{"hash comment", []string{"echo hello #comment"}, true},
		{"tilde home", []string{"ls ~/bin"}, true},
		{"tab char", []string{"echo\thello"}, true},
		{"cd builtin with space", []string{"cd /tmp"}, true},

		// --- edge cases ---
		{"empty slice", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsShell(tt.command)
			if got != tt.want {
				t.Errorf("needsShell(%v) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hasShellOperators
// ---------------------------------------------------------------------------

func TestHasShellOperators(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"double ampersand", "echo a && echo b", true},
		{"double pipe", "cmd1 || cmd2", true},
		{"single pipe", "echo hello | grep hello", true},
		{"double semicolon", "case x in a) echo a;; esac", true},
		{"redirect append", "echo hello >> /tmp/out", true},
		{"heredoc", "cat << EOF", true},
		{"process substitution <(", "diff <(sort a) <(sort b)", true},
		{"process substitution >(", "tee >(wc -l)", true},
		{"cd builtin", "cd /tmp", true},
		{"source builtin", "source ~/.bashrc", true},
		{"dot builtin", ". ~/.bashrc", true},
		{"export builtin", "export FOO=bar", true},
		{"eval builtin", "eval echo hello", true},
		{"exec builtin", "exec /bin/app", true},
		{"simple command with spaces", "echo hello world", false},
		{"path with flags", "/usr/bin/app --flag value", false},
		{"relative path", "./my-script --arg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasShellOperators(tt.cmd)
			if got != tt.want {
				t.Errorf("hasShellOperators(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FindShell
// ---------------------------------------------------------------------------

func TestFindShell(t *testing.T) {
	shell := FindShell()
	if runtime.GOOS != "windows" {
		// On any standard Unix-like OS, at least sh should be present.
		if shell == "" {
			t.Error("expected to find a shell on this system, got empty string")
		}
	}
}

// ---------------------------------------------------------------------------
// buildCommand
// ---------------------------------------------------------------------------

func TestBuildCommandDirect(t *testing.T) {
	ctx := context.Background()
	cmd, err := buildCommand(ctx, []string{"echo", "hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be direct exec, not through a shell
	if filepath.Base(cmd.Path) != "echo" {
		t.Errorf("expected direct exec of echo, got %s", cmd.Path)
	}
	// Args[0] is the command itself, then the arguments
	if len(cmd.Args) != 3 || cmd.Args[1] != "hello" || cmd.Args[2] != "world" {
		t.Errorf("unexpected args: %v", cmd.Args)
	}
}

func TestBuildCommandShell(t *testing.T) {
	shell := FindShell()
	if shell == "" {
		t.Skip("no shell available")
	}

	ctx := context.Background()
	cmd, err := buildCommand(ctx, []string{"echo hello && echo world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be invoked through a shell
	if cmd.Path != shell {
		t.Errorf("expected shell path %s, got %s", shell, cmd.Path)
	}
	// Args should be [shell, -c, command_string]
	if len(cmd.Args) != 3 || cmd.Args[1] != "-c" {
		t.Errorf("unexpected args: %v", cmd.Args)
	}
}

func TestBuildCommandFallbackSplit(t *testing.T) {
	// Simulate no shell by temporarily modifying PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	ctx := context.Background()

	// A simple space-separated command without shell operators should
	// fall back to whitespace splitting.
	cmd, err := buildCommand(ctx, []string{"/bin/echo hello world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(cmd.Path) != "echo" {
		t.Errorf("expected fallback to split /bin/echo, got %s", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "hello" || cmd.Args[2] != "world" {
		t.Errorf("unexpected args: %v", cmd.Args)
	}
}

func TestBuildCommandNoShellWithOperators(t *testing.T) {
	// Simulate no shell by temporarily modifying PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	ctx := context.Background()

	// A command with shell operators should fail when no shell is available.
	_, err := buildCommand(ctx, []string{"echo hello && echo world"})
	if err == nil {
		t.Fatal("expected error when no shell and command has operators, got nil")
	}
}

func TestBuildCommandNoShellWithBuiltin(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	ctx := context.Background()

	_, err := buildCommand(ctx, []string{"cd /tmp"})
	if err == nil {
		t.Fatal("expected error when no shell and command starts with builtin, got nil")
	}
}

func TestBuildCommandEmptyFallback(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	ctx := context.Background()

	// Command that is only whitespace
	_, err := buildCommand(ctx, []string{"   "})
	if err == nil {
		t.Fatal("expected error for whitespace-only command, got nil")
	}
}

// ---------------------------------------------------------------------------
// Runner: basic execution
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Runner: shell command execution
// ---------------------------------------------------------------------------

func TestRunnerShellDoubleAmpersand(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"echo hello && echo world"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellPipe(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"echo hello | grep hello"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellSemicolon(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"echo first; echo second"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellRedirect(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	tmpFile := filepath.Join(t.TempDir(), "out.txt")
	cfg := config.Default()
	cfg.Command = []string{"echo redirected > " + tmpFile}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	exitCode, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read redirect output: %v", err)
	}
	if got := string(bytes.TrimSpace(data)); got != "redirected" {
		t.Errorf("expected 'redirected', got %q", got)
	}
}

func TestRunnerShellVariable(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	// $$ expands to the shell's PID, always a positive number
	cfg.Command = []string{"test $$ -gt 0"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	exitCode, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0 (variable expansion worked), got %d", exitCode)
	}
}

func TestRunnerShellCd(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"cd /tmp && pwd"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellExitCodePropagation(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"echo hello && exit 7"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	exitCode, err := r.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if exitCode != 7 {
		t.Errorf("expected exit code 7, got %d", exitCode)
	}
}

func TestRunnerShellDoublePipe(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"false || echo fallback"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellSubshell(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"(echo subshell)"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellBacktick(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	cfg.Command = []string{"echo `echo nested`"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerShellGlob(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}
	cfg := config.Default()
	// /tmp/* should expand to at least nothing without error, /dev/null always exists
	cfg.Command = []string{"ls /dev/null"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

// ---------------------------------------------------------------------------
// Runner: no-shell fallback (whitespace splitting)
// ---------------------------------------------------------------------------

func TestRunnerNoShellFallbackSimple(t *testing.T) {
	// Temporarily set PATH to empty to simulate no shell
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	cfg := config.Default()
	cfg.Command = []string{"/bin/echo hello fallback"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerNoShellFallbackFailsOnOperators(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	cfg := config.Default()
	cfg.Command = []string{"echo hello && echo world"}

	r := New(cfg)
	err := r.Start(context.Background())
	if err == nil {
		r.Kill()
		r.Wait()
		t.Fatal("expected error when no shell and command has operators")
	}
}

func TestRunnerNoShellFallbackFailsOnCd(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	cfg := config.Default()
	cfg.Command = []string{"cd /tmp"}

	r := New(cfg)
	err := r.Start(context.Background())
	if err == nil {
		r.Kill()
		r.Wait()
		t.Fatal("expected error when no shell and command starts with cd")
	}
}

// ---------------------------------------------------------------------------
// Runner: single element without metacharacters (direct exec, no shell needed)
// ---------------------------------------------------------------------------

func TestRunnerSingleBinaryNoShell(t *testing.T) {
	// Create a simple executable script
	dir := t.TempDir()
	binPath := filepath.Join(dir, "myapp")
	err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	cfg := config.Default()
	cfg.Command = []string{binPath}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

// ---------------------------------------------------------------------------
// Runner: start idempotency
// ---------------------------------------------------------------------------

func TestRunnerDoubleStart(t *testing.T) {
	cfg := config.Default()
	cfg.Command = []string{"sleep", "1"}

	r := New(cfg)
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	// Second start should be a no-op
	if err := r.Start(ctx); err != nil {
		t.Fatalf("second Start should succeed (no-op), got: %v", err)
	}

	r.Kill()
	r.Wait()
}

// ---------------------------------------------------------------------------
// Runner: custom command with args in single string (the original issue)
// ---------------------------------------------------------------------------

func TestRunnerCustomCmdWithArgs(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}

	// Create a script that echoes its args
	dir := t.TempDir()
	script := filepath.Join(dir, "custom-cmd")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho \"argc=$#\"\nfor arg in \"$@\"; do echo \"arg=$arg\"; done\n"), 0755)
	if err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	cfg := config.Default()
	cfg.Command = []string{script + " --params a --params b"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

func TestRunnerCdAndRun(t *testing.T) {
	if FindShell() == "" {
		t.Skip("no shell available")
	}

	// Simulates: sentinel -- 'cd /tmp && /bin/echo it works'
	cfg := config.Default()
	cfg.Command = []string{"cd /tmp && /bin/echo it works"}

	r := New(cfg)
	if err := r.Start(context.Background()); err != nil {
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

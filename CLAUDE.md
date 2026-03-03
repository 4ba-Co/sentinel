# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sentinel is a universal process wrapper for monitoring and supervision, written in Go. It works as a PID 1 init process in containers, a cron job wrapper, or a systemd service supervisor. Module path: `github.com/4ba-Co/sentinel`.

## Build & Development Commands

```bash
# Build
go build -o sentinel .

# Production build (static, stripped)
CGO_ENABLED=0 go build -ldflags="-s -w" -o sentinel .

# Run all tests
go test ./...

# Run tests with race detection and coverage (CI mode)
go test -v -race -coverprofile=coverage.out ./...

# Run tests for a specific package
go test ./internal/runner/...

# Lint
go vet ./...

# Ensure module is tidy (CI checks this)
go mod tidy

# Run sentinel
./sentinel [flags] -- command [args...]
```

## Architecture

**Entrypoint**: `main.go` wires together all internal packages. The `run()` function implements the restart loop; `runOnce()` handles a single process execution with health checks, idle timeout, and signal forwarding.

**Package structure** (all under `internal/`, not importable externally):

- **config** — `Config` struct with typed enums (`RestartPolicy`, `LogFormat`). YAML loading in `yaml.go`, defaults in `Default()`. CLI flags override YAML values.
- **runner** — Process lifecycle (`Start`/`Wait`/`Signal`/`Kill`). Creates processes in new process groups (`Setpgid: true`) and signals the entire group. `ActivityWriter` in `output.go` wraps stdout/stderr to track last write time for idle detection.
- **signal** — Signal handler for SIGTERM/INT/QUIT/HUP/USR1/USR2. Zombie reaper for PID 1 mode via `SIGCHLD` and `Wait4(-1, ..., WNOHANG)`. Channel-based coordination (`Done()` channel).
- **health** — Periodic health check command execution. Fails after 3 consecutive failures, signals via `Failed()` channel.
- **alert** — Three backends: stderr (log), webhook (HTTP POST JSON), script (passes event data via env vars `SENTINEL_EVENT`, `SENTINEL_EXIT_CODE`, etc.). Event filtering via `Config.AlertEvents`; `Send()` drops events not in the list.
- **logger** — Global singleton structured logger with mutex-protected writes. Supports text and JSON output formats. Tests redirect output via `logger.SetOutput()`.

**CLI parsing**: `cmd/root.go` uses stdlib `flag.FlagSet` (not cobra). Command is separated by `--` delimiter.

## Key Design Patterns

- **Process group management**: Runner spawns child processes in their own process group and signals/kills the entire group (`syscall.Kill(-pgid, ...)`).
- **Exponential backoff**: Restart delay is `base * 2^(attempt-1)`, capped at 30 seconds.
- **Configuration layering**: YAML config loaded first, then CLI flags override.
- **Channel-based coordination**: Health checker, signal handler, and idle monitor communicate via channels with select-based multiplexing in `runOnce()`.
- **Zero runtime dependencies**: Only `gopkg.in/yaml.v3` at build time; the binary is fully static.

## Testing Patterns

- Standard `testing` package only (no testify or third-party frameworks).
- Table-driven tests throughout.
- `httptest.NewServer` for webhook tests.
- Runner tests spawn real processes (`echo`, `sleep`, `sh -c`).
- Logger tests use `logger.SetOutput(&buf)` in `init()` to capture output.
- Channel-based assertions with timeouts for async behavior.

## CI

Runs on Blacksmith runners. Lint job runs `go vet` + `staticcheck` + `go mod tidy` check. Integration tests verify exit code propagation, timeout, restart, idle timeout, JSON logging, and health checks against the built binary.

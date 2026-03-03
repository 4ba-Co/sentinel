# Sentinel

A universal process wrapper for monitoring and supervision. Works across containers (Docker/K8s), VMs, cron jobs, and systemd services.

## Features

- **Process Management**: Signal forwarding, zombie reaping, exit code propagation
- **PID 1 Mode**: Proper init behavior when running as container entrypoint
- **Restart Policies**: `never`, `always`, `on-failure` with configurable retries and backoff
- **Health Checks**: Periodic health command execution
- **Timeouts**: Execution timeout, idle timeout (no output detection)
- **Alerting**: stderr, webhook (HTTP POST), custom scripts
- **Structured Logging**: Text or JSON format

## Installation

```bash
go install github.com/4ba-Co/sentinel@latest
```

Or build from source:

```bash
go build -o sentinel .
```

## Usage

```bash
sentinel [flags] -- command [args...]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--timeout` | Execution timeout (e.g., `10m`, `1h`) | none |
| `--idle-timeout` | Kill if no output for this duration | none |
| `--restart` | Restart policy: `never`, `always`, `on-failure` | `never` |
| `--max-retries` | Maximum restart attempts (0 = unlimited) | 0 |
| `--grace-period` | Time to wait for graceful shutdown | `10s` |
| `--health-cmd` | Health check command | none |
| `--health-interval` | Health check interval | `30s` |
| `--health-timeout` | Health check timeout | `5s` |
| `--alert` | Alert methods (comma-separated) | `stderr` |
| `--webhook-url` | Webhook URL for alerts | none |
| `--alert-cmd` | Custom alert script | none |
| `--alert-on-success` | Send alert on successful exit | `false` |
| `--log-format` | Log format: `text`, `json` | `text` |
| `--success-codes` | Exit codes to treat as success | `0` |
| `--failure-codes` | Exit codes to trigger restart | none |
| `--config` | YAML config file path | none |

### Examples

**One-shot job with timeout (cron):**
```bash
sentinel --timeout 10m -- ./backup.sh
```

**Long-running service with restart:**
```bash
sentinel --restart on-failure --max-retries 5 -- ./my_server
```

**Container entrypoint (PID 1):**
```dockerfile
ENTRYPOINT ["sentinel", "--grace-period=30s", "--"]
CMD ["/app/server"]
```

**With health check and webhook alerts:**
```bash
sentinel \
  --restart on-failure \
  --health-cmd "./healthcheck.sh" \
  --alert webhook \
  --webhook-url "https://hooks.slack.com/xxx" \
  -- ./my_server
```

**Cron job with success and failure notifications:**
```bash
sentinel \
  --alert-on-success \
  --alert script \
  --alert-cmd './notify.sh' \
  -- ./backup.sh
```

**Using config file:**
```bash
sentinel --config config.yaml -- ./my_app
```

### Config File (YAML)

```yaml
timeout: 30m
restart: on-failure
max_retries: 5
grace_period: 10s
health_cmd: "./healthcheck.sh"
health_interval: 30s
alert:
  - stderr
  - webhook
webhook_url: "https://hooks.slack.com/xxx"
alert_on_success: true
log_format: json
success_codes:
  - 0
  - 143
```

## Alert Script Environment

When using `--alert-cmd`, the script receives these environment variables:

- `SENTINEL_EVENT`: Event type (`started`, `exited`, `success`, `timeout`, `restart`, `health_failed`, `killed`)
- `SENTINEL_EXIT_CODE`: Process exit code
- `SENTINEL_MESSAGE`: Event description
- `SENTINEL_COMMAND`: The wrapped command
- `SENTINEL_HOSTNAME`: Host name

> **Note:** The `success` event is only sent when `--alert-on-success` is enabled. By default, only non-success exits trigger the `exited` event.

### Alert Script Example

```bash
#!/bin/bash
case "$SENTINEL_EVENT" in
  success)
    curl -d "Backup completed on $SENTINEL_HOSTNAME" ntfy.sh/my-alerts
    ;;
  exited)
    curl -d "Backup FAILED (exit=$SENTINEL_EXIT_CODE): $SENTINEL_MESSAGE" ntfy.sh/my-alerts
    ;;
  health_failed)
    curl -d "Health check failed on $SENTINEL_HOSTNAME" ntfy.sh/my-alerts
    ;;
esac
```

## Comparison

| Feature | sentinel | tini | supervisord | monit |
|---------|----------|------|-------------|-------|
| PID 1 / init | ✅ | ✅ | ❌ | ❌ |
| Single process | ✅ | ✅ | ❌ | ❌ |
| Restart policy | ✅ | ❌ | ✅ | ✅ |
| Health checks | ✅ | ❌ | ❌ | ✅ |
| Timeout | ✅ | ❌ | ❌ | ✅ |
| Webhooks | ✅ | ❌ | ❌ | ❌ |
| Zero deps | ✅ | ✅ | ❌ | ❌ |

## License

MIT

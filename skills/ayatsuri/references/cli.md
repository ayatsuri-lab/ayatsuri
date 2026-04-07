# Ayatsuri CLI Reference

Global flags on all commands: `--config/-c`, `--ayatsuri-home`, `--quiet/-q`, `--cpu-profile`

## Core Commands

### ayatsuri start

Execute a DAG.

```
ayatsuri start [flags] <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters (key=value or positional)
- `--name/-N` — Override DAG name
- `--run-id/-r` — Custom run ID
- `--from-run-id` — Use a previous run as template for a new run
- `--tags` — Additional tags (comma-separated key=value or key-only)
- `--default-working-dir` — Default working directory for DAGs without explicit workingDir
- `--worker-id` — Worker ID for distributed execution (defaults to `local`)
- `--trigger-type` — How this run was initiated: `manual` (default), `scheduler`, `webhook`, `subdag`, `retry`, `catchup`
- `--parent` — Parent dag-run reference (sub dag-runs only)
- `--root` — Root dag-run reference (sub dag-runs only)

### ayatsuri exec

Run a one-off command as a DAG run without a YAML file.

```
ayatsuri exec [flags] -- <command> [args...]
```

Flags:
- `--name/-N` — Name for the run
- `--run-id/-r` — Custom run ID
- `--workdir` — Working directory (default: current directory)
- `--shell` — Override shell binary
- `--base` — Path to a base DAG YAML whose defaults are applied
- `--env/-E` — Environment variable (KEY=VALUE), repeatable
- `--dotenv` — Path to a dotenv file, repeatable
- `--worker-label` — Worker label selector (key=value) for distributed execution, repeatable

### ayatsuri enqueue

Enqueue a DAG run for later execution.

```
ayatsuri enqueue [flags] <dag> [-- params...]
```

Flags:
- `--params/-p` — Parameters (key=value or positional)
- `--name/-N` — Override DAG name
- `--queue/-u` — Override the DAG-level queue definition
- `--run-id/-r` — Custom run ID
- `--tags` — Additional tags (comma-separated)
- `--default-working-dir` — Default working directory for DAGs without explicit workingDir
- `--trigger-type` — Trigger type (default: `manual`)

### ayatsuri dequeue

Dequeue a DAG run from a queue (marks it as aborted): `ayatsuri dequeue <queue-name> [--dag-run/-d <dag:run-id>]`

### ayatsuri stop

Stop an active DAG run: `ayatsuri stop <dag-name> [--run-id/-r <id>]`

### ayatsuri restart

Stop and restart a DAG run: `ayatsuri restart <dag-name> [--run-id/-r <id>]`

### ayatsuri retry

Retry a previous DAG run using the same run ID.

```
ayatsuri retry <dag> --run-id/-r <id> [--step <name>] [--worker-id <id>]
```

### ayatsuri dry

Dry-run a DAG without executing commands: `ayatsuri dry [--params/-p] [--name/-N] <dag> [-- params...]`

### ayatsuri validate

Validate DAG YAML without executing: `ayatsuri validate <dag>`

### ayatsuri status

Show DAG run status: `ayatsuri status <dag-name> [--run-id/-r <id>] [--sub-run-id/-s <id>]`

### ayatsuri history

Show DAG run history.

```
ayatsuri history [dag-name]
```

Flags:
- `--from` — Start date/time in UTC (format: `2006-01-02` or `2006-01-02T15:04:05Z`)
- `--to` — End date/time in UTC (same formats as `--from`)
- `--last` — Relative time period (e.g. `7d`, `24h`, `1w`). Cannot combine with `--from`/`--to`
- `--status` — Filter by status: `running`, `succeeded`, `failed`, `aborted`, `queued`, `waiting`, `rejected`, `not_started`, `partially_succeeded`
- `--run-id` — Filter by run ID (partial match supported)
- `--tags` — Filter by tags (comma-separated, AND logic)
- `--format/-f` — Output format: `table` (default), `json`, `csv`
- `--limit/-l` — Max results (default 100, max 1000)

Default: shows runs from the last 30 days, newest first.

### ayatsuri cleanup

Remove old DAG run history. Active runs are never deleted.

```
ayatsuri cleanup <dag-name> [--retention-days <n>] [--dry-run] [--yes/-y]
```

### ayatsuri schema

Show JSON schema documentation. Use a dot-separated path to drill into nested sections.

```
ayatsuri schema <dag|config> [path]
```

Examples:
- `ayatsuri schema dag` — All DAG root-level fields
- `ayatsuri schema dag steps` — Step definition structure
- `ayatsuri schema dag steps.container` — Container configuration
- `ayatsuri schema dag steps.retry_policy` — Retry policy fields
- `ayatsuri schema dag steps.agent` — Agent step configuration
- `ayatsuri schema dag handler_on` — Lifecycle event hooks
- `ayatsuri schema config` — All config root-level fields
- `ayatsuri schema config auth` — Authentication configuration

### ayatsuri config

Show resolved configuration paths.

```
ayatsuri config
```

## Server & Scheduling

### ayatsuri start-all

Start server + scheduler + optionally coordinator in one process. Coordinator enabled by default (disable with `AYATSURI_COORDINATOR_ENABLED=false`).

```
ayatsuri start-all [--host/-s <host>] [--port/-p <port>] [--dags/-d <dir>]
```

Also accepts `--coordinator.*` and `--peer.*` flags for distributed setup.

### ayatsuri server

Start web UI + REST API.

```
ayatsuri server [--host/-s <host>] [--port/-p <port>] [--dags/-d <dir>] [--tunnel/-t]
```

### ayatsuri scheduler

Start cron scheduler. Monitors DAGs and triggers runs on schedule; also processes queued runs.

```
ayatsuri scheduler [--dags/-d <dir>]
```

## Distributed Execution

### ayatsuri coordinator

Start gRPC coordinator: `ayatsuri coordinator [--coordinator.host/-H <host>] [--coordinator.port/-P <port>] [--peer.*]`

### ayatsuri worker

Start distributed worker: `ayatsuri worker [--worker.id/-w <id>] [--worker.max-active-runs/-m <n>] [--worker.labels/-l <k=v,...>] [--worker.coordinators <addrs>] [--peer.*]`

## Git Sync

`ayatsuri sync <subcommand>` — Git sync operations for DAG definitions.

| Subcommand | Description |
|------------|-------------|
| `sync status` | Show sync status (repository, branch, per-DAG status) |
| `sync pull` | Pull changes from remote |
| `sync publish [dag] [--message/-m] [--all] [--force/-f]` | Publish local changes to remote |
| `sync discard <dag> [--yes/-y]` | Discard local changes, restore remote version |
| `sync forget <id>... [--yes/-y]` | Remove state entries for missing/untracked items |
| `sync cleanup [--dry-run] [--yes/-y]` | Remove all missing entries from sync state |
| `sync delete <id> [--message/-m] [--force] [--all-missing] [--dry-run] [--yes/-y]` | Delete from remote, local, and sync state |
| `sync mv <old> <new> [--message/-m] [--force] [--dry-run] [--yes/-y]` | Rename across local, remote, and sync state |

## Other Commands

- `ayatsuri ai install [--yes/-y] [--skills-dir <path>]` — Install DAG authoring skill into AI coding tools (Claude Code, Codex, etc.)
- `ayatsuri example [id]` — Show built-in example DAGs (12 available)
- `ayatsuri version` — Show version
- `ayatsuri upgrade [--check] [--version/-v <ver>] [--dry-run] [--yes/-y]` — Self-update binary
- `ayatsuri license <activate|deactivate|check>` — Manage license
- `ayatsuri migrate history` — Migrate data from v1.16 to v1.17+ format

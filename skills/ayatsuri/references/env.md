# Environment Variables

## Execution Variables

Set automatically during DAG execution. Defined in `internal/core/exec/env.go`.

### Always Available (set for every step)

| Variable | Description |
|----------|-------------|
| `DAG_NAME` | Name of the executing DAG |
| `DAG_RUN_ID` | Unique run identifier |
| `DAG_RUN_LOG_FILE` | Path to the main log file for the DAG run |
| `DAG_RUN_STEP_NAME` | Name of the currently executing step |
| `DAG_RUN_STEP_STDOUT_FILE` | Path to the step's stdout log file |
| `DAG_RUN_STEP_STDERR_FILE` | Path to the step's stderr log file |

### Conditionally Set

| Variable | Condition | Description |
|----------|-----------|-------------|
| `DAG_RUN_WORK_DIR` | Only if a per-run working directory is configured | Path to the per-DAG-run working directory |
| `DAG_DOCS_DIR` | Only if `paths.docs_dir` is configured | Per-DAG docs directory (`{docs_dir}/{dag_name}`) |
| `AYATSURI_PARAMS_JSON` | Only if the DAG has parameters | Resolved parameters encoded as JSON |

### Handler-Only Variables

These are only available inside lifecycle handler steps, not during normal step execution.

| Variable | Handler Scope | Description |
|----------|---------------|-------------|
| `DAG_RUN_STATUS` | `onSuccess`, `onFailure`, `onAbort`, `onExit`, `onWait` | Current DAG run status (e.g., `success`, `failed`) |
| `DAG_WAITING_STEPS` | `onWait` only | Comma-separated list of step names that are waiting for approval |

## Param and Env Resolution

- `params:` values are exposed as strings. Pass structured data as JSON strings if a downstream step needs objects or arrays.
- `env:` values can reference `params:` values because parameter resolution happens first.
- Use list-of-maps for `env:` when one env var depends on another. Go maps do not preserve evaluation order.

```yaml
params:
  base: /tmp
env:
  - ROOT: "${base}"
  - OUTPUT_DIR: "${ROOT}/out"
```

## Configuration Variables

All configuration environment variables use the `AYATSURI_` prefix. They map to config keys via viper bindings in `internal/cmn/config/loader.go`.

### Paths

| Variable | Default | Description |
|----------|---------|-------------|
| `AYATSURI_HOME` | XDG dirs | Base directory for all Ayatsuri data. When set, all paths use unified structure under this directory |
| `AYATSURI_DAGS_DIR` | `$AYATSURI_HOME/dags` | DAG YAML files directory |
| `AYATSURI_LOG_DIR` | `$AYATSURI_HOME/logs` | Log files directory |
| `AYATSURI_DATA_DIR` | `$AYATSURI_HOME/data` | Data storage directory |
| `AYATSURI_DOCS_DIR` | — | Documentation directory |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `AYATSURI_HOST` | `127.0.0.1` | Server bind address |
| `AYATSURI_PORT` | `8080` | Server port |
| `AYATSURI_BASE_PATH` | `""` (empty) | URL base path for reverse proxy setups |
| `AYATSURI_TZ` | system | Timezone for schedules |

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `AYATSURI_DEFAULT_SHELL` | — | Default shell for commands |
| `AYATSURI_SKIP_EXAMPLES` | `false` | Skip creating example DAGs |
| `AYATSURI_DEFAULT_EXECUTION_MODE` | `local` | Execution mode: `local` or `distributed` |

### Features

| Variable | Default | Description |
|----------|---------|-------------|
| `AYATSURI_TERMINAL_ENABLED` | `false` | Enable web terminal feature |
| `AYATSURI_QUEUE_ENABLED` | `true` | Enable queue system |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `AYATSURI_AUTH_MODE` | `builtin` | Auth mode: `none`, `basic`, `builtin` |
| `AYATSURI_AUTH_BASIC_USERNAME` | — | Basic auth username (requires `auth.mode=basic`) |
| `AYATSURI_AUTH_BASIC_PASSWORD` | — | Basic auth password (requires `auth.mode=basic`) |

OIDC settings are available under the `AYATSURI_AUTH_OIDC_*` prefix (client ID, secret, issuer, scopes, role mappings, etc.).

### TLS

| Variable | Description |
|----------|-------------|
| `AYATSURI_CERT_FILE` | TLS certificate file path |
| `AYATSURI_KEY_FILE` | TLS key file path |

### Distributed Mode

Coordinator settings use the `AYATSURI_COORDINATOR_*` prefix (host, port, advertise address).

Worker settings use the `AYATSURI_WORKER_*` prefix (worker ID, max active runs, labels, coordinator addresses, PostgreSQL pool settings).

### Other Configuration Prefixes

- **Git Sync**: `AYATSURI_GITSYNC_*` — repository sync settings (repo URL, branch, auth, auto-sync interval)
- **Tunnel**: `AYATSURI_TUNNEL_*` — Tailscale tunnel settings
- **Peer TLS**: `AYATSURI_PEER_*` — gRPC peer TLS settings

## Path Resolution

When `AYATSURI_HOME` is set, all paths use a **unified structure** under that directory:

```
$AYATSURI_HOME/
├── dags/          # DAG definitions
├── data/          # Application data
├── logs/          # Logs
│   └── admin/     # Admin logs
├── suspend/       # Suspend flags
└── base.yaml      # Base configuration
```

When `AYATSURI_HOME` is not set, XDG-compliant paths are used (`$XDG_CONFIG_HOME/ayatsuri/`, `$XDG_DATA_HOME/ayatsuri/`).

Individual path variables (e.g., `AYATSURI_DAGS_DIR`) override the defaults regardless of which resolution mode is active.

# Executor Types

## command / shell (default)

Shell command execution. Uses step `command:`, `script:`, or `shell:` fields.

```yaml
steps:
  - name: example
    command: echo "hello"

  - name: multi-line
    script: |
      echo "step 1"
      echo "step 2"

  - name: custom-shell
    shell: /bin/bash
    script: |
      set -euo pipefail
      echo "running in bash"
```

Aliases: (empty), `command`, `shell`

Step-level fields:
- `command` — Command string to execute
- `args` — Arguments for the command
- `script` — Multi-line shell script content
- `shell` — Shell interpreter (e.g., `/bin/bash`)

Notes:
- Ayatsuri expands `${VAR}` before the shell runs. For large or arbitrary text, prefer `printenv VAR_NAME`, reading `${step_id.stdout}` as a file, or `type: template`.

## dag

Execute another DAG as a sub-step.

```yaml
steps:
  - name: child
    type: dag
    call: child-workflow
    params:
      input: /data/file.csv
```

Aliases: `dag`, `subworkflow`

Uses step `call:` and `params:` fields. Sub-DAGs do not inherit parent env vars.

Notes:
- Pass values explicitly via `params:` when the child needs parent env vars or derived values.
- Child step `output:` variables are not propagated back into the parent DAG output map. Use shared files or another explicit handoff if the parent needs results.

## parallel

Execute same DAG multiple times in parallel. Requires `call:` field.

```yaml
steps:
  # Simple list of items
  - name: fan-out
    call: process-item
    parallel:
      - item1
      - item2
      - item3

  # Object form with concurrency control
  - name: fan-out-limited
    call: process-item
    parallel:
      items:
        - item1
        - item2
        - item3
      max_concurrent: 5

  # Items with key-value parameters
  - name: fan-out-params
    call: process-item
    parallel:
      items:
        - SOURCE: s3://customers
        - SOURCE: s3://products

  # Variable reference (JSON array)
  - name: fan-out-dynamic
    call: process-item
    parallel: ${ITEMS}
```

Config fields:
- `items` — Array of items to process (strings or key-value param maps)
- `max_concurrent` — Max parallel executions (default 10)

Each parallel invocation receives the current item as the `ITEM` variable.

Notes:
- `parallel:` only works with `call:` to a sub-DAG; it does not fan out a normal shell step.
- If an upstream step produced multiline text, read `${step_id.stdout}` from a shell step or convert the data into an array before using `parallel:`.

## ssh / sftp

Remote command execution and file transfer over SSH.

```yaml
steps:
  - name: remote
    type: ssh
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      timeout: 60s
    command: systemctl restart app

  - name: upload
    type: sftp
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      direction: upload
      source: /local/file.tar.gz
      destination: /remote/file.tar.gz
```

Shared SSH config fields: `user`, `host`, `port` (default 22), `key`, `password`, `timeout` (default 30s), `strict_host_key` (default true), `known_host_file`, `shell`, `shell_args`, `bastion` (jump host with `host`, `port`, `user`, `key`, `password`).

SFTP additional fields: `direction` (`upload` or `download`), `source`, `destination`.

## template

Render text using Go `text/template`.

```yaml
steps:
  - id: render
    type: template
    config:
      data:
        name: Alice
    script: |
      Hello, {{ .name }}!
    output: RESULT
```

Behavior:
- `script` is required and is rendered as a template, not executed as a shell script
- Template data comes from `config.data` and is accessed as `{{ .key }}`
- Supports normal Go template control flow plus a safe subset of slim-sprig functions
- Missing keys fail the step
- If `config.output` is set, the rendered result is written to that file instead of stdout
- Relative `config.output` paths are resolved from the step working directory

Config fields:
- `data` — Object exposed to the template as `.`
- `output` — File path for rendered output; if omitted, rendered text is written to stdout

Important: step `output:` and `config.output` are different. Step `output:` captures stdout into a Ayatsuri variable. `config.output` writes the rendered result directly to a file.

Use `template` when you need to generate text files such as Markdown, config files, SQL, JSON, or prompts. It is usually safer and simpler than building files with `echo`, heredocs, or shell string interpolation.

## agent

AI agent loop with tools.

```yaml
steps:
  - name: research
    type: agent
    agent:
      model: claude-sonnet-4-20250514
      tools:
        enabled:
          - web_search
          - bash
      skills:
        - my-skill-id
      prompt: "Research and summarize ${TOPIC}"
      max_iterations: 50
      safe_mode: true
    messages:
      - role: user
        content: "Begin research on ${TOPIC}"
```

Agent config fields (under `agent:`): `model`, `tools` (with `enabled` list and optional `bash_policy`), `skills` (skill IDs), `soul` (soul ID), `memory` (`enabled` bool), `prompt` (appended to system prompt), `max_iterations` (default 50), `safe_mode` (enable command approval, default true), `web_search`. Also accepts `messages:` at step level.

## router

Conditional routing based on expression value. Routes reference existing step names — they do not define inline steps.

```yaml
steps:
  - name: check-status
    command: "curl -s -o /dev/null -w '%{http_code}' https://example.com"
    output: STATUS

  - name: route
    type: router
    value: ${STATUS}
    depends: check-status
    routes:
      "200":
        - handle-ok
      "re:5\\d{2}":
        - handle-error
        - send-alert

  - name: handle-ok
    command: echo "success"

  - name: handle-error
    command: echo "server error occurred"

  - name: send-alert
    command: echo "alerting on-call"
```

Step-level fields:
- `value` — Expression to evaluate (required)
- `routes` — Map of pattern to list of target step names (required)

Pattern matching:
- Exact match: `"200"` matches the literal value `200`
- Regex match: `"re:5\\d{2}"` matches `500`, `502`, etc.
- Catch-all: `"re:.*"` matches anything (sorted last automatically)

Routing rules:
- Routes are evaluated in priority order: exact matches first, then regex, then catch-all
- Each target step can only be targeted by one route pattern
- Multiple targets per route execute in parallel
- Steps not targeted by any matching route are skipped

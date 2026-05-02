# Drone CLI Commands (OSS build)

Kernel-only CLI reference. This is the open-source build of the DevClaw `drone`
binary — it provides the autonomous coding runtime, 13 built-in tools, 5 roles,
worktree isolation, and MCP support.

> Looking for the enterprise integrations (Forge issue tracking, Pheromone
> event reporting, Overlord orchestration)? Those live in the private
> StarClaw monorepo and plug into the same runtime exposed here.

---

## Subcommands

| Command | Description |
|---------|-------------|
| `drone run ...` | Execute a single task (primary command) |
| `drone roles` | List available roles and their default permissions |
| `drone version` | Print build version |
| `drone help` | Print usage |

---

## `drone run` — execute a task

```bash
# Simplest form — task as positional args
drone run "fix the broken link in README.md"

# Explicit task flag (useful in scripts)
drone run --task "fix the broken link in README.md"

# Read task from a file
drone run --task-file ./task.txt

# Pick a role (changes default tools + permission)
drone run --role test --task "add unit tests for util.go"

# Isolated worktree (recommended for non-trivial edits)
drone run --worktree --task "refactor auth module"

# Cap turns (default 50)
drone run --max-turns 20 --task "..."

# Capture trajectory for later analysis / skill distillation
drone run --trajectory ./traces --task "..."
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--task` | Task description (required unless positional args or `--task-file`) | — |
| `--task-file` | Read task from file | — |
| `--role` | Role: `dev`, `test`, `ops`, `sense`, `scout` | `dev` |
| `--model` | Model name (overrides env) | `DRONE_MODEL` |
| `--max-turns` | Max agent loop turns | `50` |
| `--workspace` | Workspace directory | `$PWD` |
| `--worktree` | Use git worktree isolation (bool) | `false` |
| `--permission` | `readonly`, `workspace_write`, `full_access` | role default |
| `--trajectory` | Directory for trajectory logs | disabled |
| `--quiet` | Suppress streaming output | `false` |

---

## `drone roles` — list roles

```bash
drone roles
# dev      Developer role (read + write + exec) (permission: workspace_write)
# test     Tester role (read + write + exec, stricter) (permission: workspace_write)
# ops      Operator role (full system access) (permission: full_access)
# sense    Sensor role (read-only analysis) (permission: readonly)
# scout    Scout role (read + web + light edits) (permission: workspace_write)
```

---

## Environment variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `DRONE_API_KEY` | LLM API key | falls through to `STARAI_API_KEY` / `OPENAI_API_KEY`; finally `"ollama"` |
| `DRONE_BASE_URL` | OpenAI-compatible endpoint | falls through to `OPENAI_BASE_URL`; finally auto-detect |
| `DRONE_MODEL` | Model name | auto: `qwen3-coder-plus` (StarAI) or `qwen3-coder` (Ollama) |

**Auto-detect rule** — if no `DRONE_BASE_URL` is set:
- `STARAI_API_KEY` present → `https://api.star-ai.net/v1`
- otherwise → `http://localhost:11434/v1` (Ollama)

---

## Project knowledge files

Drone reads the following files (if present) and injects them into the system
prompt:

| File | Purpose |
|------|---------|
| `DRONE.md` (in workspace root) | Project-level knowledge — coding conventions, architecture notes, do's and don'ts |
| `.drone/DRONE.md` | Same as above, but inside a hidden folder |
| `.drone/skills/*.md` | Distilled skills / patterns (loaded in addition to DRONE.md) |

See `DRONE.md.example` in the repo root for a starter template.

---

## Permission levels

| Level | What drone can do |
|-------|-------------------|
| `readonly` | Read files, run read-only shell commands (e.g. `ls`, `cat`, `grep`) |
| `workspace_write` | Everything above + write files inside the workspace + run arbitrary commands within workspace |
| `full_access` | Everything above + unrestricted shell, system-wide writes. **Use with caution.** |

Default permission is picked from the role (see `drone roles`). Override with
`--permission` at runtime.

---

## MCP (Model Context Protocol) servers

Drone can attach to MCP servers over stdio to pick up additional tools. Configure
them via the `DRONE_MCP` env var or the `.drone/mcp.json` file (one server per
entry):

```json
{
  "servers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
    }
  }
}
```

Tools exposed by MCP servers show up alongside the 13 built-ins (`bash`,
`file_read`, `file_write`, `multi_edit`, `agent`, `parallel`, …) and respect
the same permission levels.

---

## Typical workflows

### One-shot task
```bash
cd ~/my-project
drone run "add JSDoc comments to all exported functions in src/utils.js"
```

### Safe exploration (read-only)
```bash
drone run --role sense --permission readonly \
  "analyze the auth flow and explain where session state is stored"
```

### Refactor in isolation
```bash
drone run --role dev --worktree --trajectory ./traces \
  --task "extract the logging logic into a separate package"
# Review the worktree branch, then merge when happy.
```

### Testing
```bash
drone run --role test \
  --task "write table-driven tests for internal/parser covering edge cases"
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Task completed successfully |
| `1` | Usage error (e.g. missing `--task`) |
| `2` | Runtime error (LLM failure, tool error, etc.) |
| `130` | Cancelled via `Ctrl-C` |

---

## See also

- [`README.md`](README.md) — project overview and quick start
- [`DRONE.md.example`](DRONE.md.example) — template for project knowledge file
- [Makefile](Makefile) — `make build`, `make test`, `make install`

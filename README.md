# 🐝 DevClaw — Autonomous Coding Kernel

> **A beehive where hundreds of drones program in parallel.**
>
> DevClaw's open-source kernel — an autonomous AI coding agent that takes a task
> description, picks tools, edits files, runs commands, and reports back.
> Single Go binary. Zero dependencies. Cross-platform.

[![CI](https://github.com/yinhe/devclaw/actions/workflows/ci.yml/badge.svg)](https://github.com/yinhe/devclaw/actions/workflows/ci.yml)
[![Release](https://github.com/yinhe/devclaw/actions/workflows/release.yml/badge.svg)](https://github.com/yinhe/devclaw/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/yinhe/devclaw?include_prereleases&sort=semver&color=fbbf24)](https://github.com/yinhe/devclaw/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/yinhe/devclaw.svg)](https://pkg.go.dev/github.com/yinhe/devclaw)
[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Go Report Card](https://goreportcard.com/badge/github.com/yinhe/devclaw)](https://goreportcard.com/report/github.com/yinhe/devclaw)
[![License](https://img.shields.io/badge/license-Apache--2.0-green.svg)](LICENSE)
[![devclaw.me](https://img.shields.io/badge/home-devclaw.me-fbbf24?logo=bee&logoColor=white)](https://devclaw.me)

---

## What is DevClaw?

DevClaw is an open-source autonomous coding agent in the spirit of **Claude Code**
and **Aider**, with three differentiators:

- 🐝 **Swarm-native** — built-in `Agent` and `Parallel` tools let one drone spawn
  sub-drones (up to 5 parallel), forming a task tree.
- 🎭 **Role + permission system** — five roles (`dev`, `test`, `ops`, `sense`,
  `scout`) × three permission tiers (`readonly`, `workspace_write`, `full_access`)
  enforced as hard constraints.
- 🧠 **Project knowledge built in** — `DRONE.md` (like `CLAUDE.md`) and
  `.drone/skills/*.md` are auto-injected into the system prompt.

This repository ships the **kernel** — runtime, tools, MCP client, roles,
worktree, and CLI. The full StarClaw enterprise build adds Forge issue tracking,
Pheromone event reporting, and Overlord fleet orchestration on top of the same
runtime via plugin hooks.

---

## Quick start

### 1. Install

```bash
# From source (requires Go 1.24+)
go install github.com/yinhe/devclaw/cmd/drone@latest

# Or build manually
git clone https://github.com/yinhe/devclaw.git
cd devclaw
go build -o drone ./cmd/drone
```

### 2. Set an API key

```bash
# Option A: OpenAI / StarAI / any OpenAI-compatible endpoint
export DRONE_API_KEY=sk-xxx
export DRONE_BASE_URL=https://api.openai.com/v1   # or your provider
export DRONE_MODEL=gpt-4o

# Option B: Local Ollama (zero cost, offline)
ollama pull qwen3-coder
# DRONE_API_KEY and DRONE_BASE_URL auto-default to Ollama if unset
```

### 3. Run a task

```bash
drone run --task "Add an English README and update the docs link"
drone run --task-file task.md --role dev --worktree
drone run "refactor this function to use generics" --quiet
```

---

## Roles

```bash
$ drone roles
Available roles:
  dev      Software development — architecture, coding, debugging, documentation (permission: workspace_write)
  test     Testing — test creation, regression testing, coverage analysis (permission: readonly)
  ops      Operations — deployment, health checks, infrastructure management (permission: full_access)
  sense    Sensing — feedback collection, anomaly detection, insight generation (permission: readonly)
  scout    Scouting — data collection, competitor analysis, external research (permission: readonly)
```

Each role auto-injects its own system-prompt section. Permission tiers gate
which tools the agent can call (e.g., `readonly` cannot use `Write`/`Edit`).

---

## Built-in tools

| Tool         | Description                                            | Min permission     |
|--------------|--------------------------------------------------------|--------------------|
| `Read`       | Read file with line numbers, offset, limit             | readonly           |
| `ListDir`    | List directory contents                                | readonly           |
| `Glob`       | Pattern-based file search                              | readonly           |
| `Grep`       | Content search using ripgrep semantics                 | readonly           |
| `Bash`       | Run shell command (timeout + output truncation)        | readonly+          |
| `Write`      | Create/overwrite a file                                | workspace_write    |
| `Edit`       | Exact string replace (unique match required)           | workspace_write    |
| `MultiEdit`  | Multiple edits to one file (atomic)                    | workspace_write    |
| `Patch`      | before/after block replace                             | workspace_write    |
| `Undo`       | Revert the most recent file modification               | workspace_write    |
| `Agent`      | Spawn a sub-drone for a focused subtask                | workspace_write    |
| `Parallel`   | Spawn up to 5 sub-drones concurrently                  | workspace_write    |

External MCP tools are loaded from `.drone/mcp.json` (stdio protocol).

---

## Project knowledge: `DRONE.md`

Drop a `DRONE.md` (or `.drone/DRONE.md`) at your project root and DevClaw will
auto-inject it into every task's system prompt. Example:

```markdown
# Project conventions

- Go 1.24, module path: github.com/myorg/myapp
- Test command: `go test ./...`
- Style: gofmt, no global state, table-driven tests
- Deploy: `git push origin main` triggers CI

# Architecture

- `cmd/server`  — HTTP entry point
- `internal/`   — business logic
- `pkg/`        — exported helpers
```

Drop domain-specific skills into `.drone/skills/*.md` — each Markdown file is
appended to the system prompt as a discrete skill block.

---

## MCP integration

Create `.drone/mcp.json`:

```json
{
  "servers": [
    {"name": "fs",     "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]},
    {"name": "github", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"]}
  ]
}
```

DevClaw will spawn each server over stdio and register all of its tools as
namespaced (`fs__read_file`, `github__create_issue`, …).

---

## Swarm parallel — task trees

Inside a task, the model can call `Agent` (one sub-drone) or `Parallel` (up to
five concurrent sub-drones). Sub-drones inherit the parent's tool registry but
get their own scratch context, role, and 20-turn cap.

```text
parent drone (dev)
├── sub-drone (dev)    "implement login API"
├── sub-drone (test)   "write tests for login"
└── sub-drone (scout)  "research bcrypt vs argon2"
```

Combine with `--worktree` and each sub-drone runs in its own git branch — fail
the whole tree with no cleanup needed.

---

## Project layout

```
.
├── cmd/drone/         CLI entry point
├── internal/
│   ├── runtime/       LLM <-> Tool agent loop, context compression, git ctx
│   ├── tool/          13 built-in tools + Agent/Parallel
│   ├── role/          5 roles with system-prompt fragments
│   ├── mcp/           Model Context Protocol stdio client
│   ├── provider/      OpenAI-compatible LLM provider + retry
│   ├── config/        Config + DRONE.md/Skills loading
│   └── worktree/      Git worktree isolation
└── go.mod
```

---

## Roadmap

- [x] **Phase 0–6** — Kernel: runtime, 13 tools, MCP, 5 roles × 3 permissions,
      worktree, agent/parallel, trajectory.
- [ ] **Phase 7** — DevClaw Web Console (https://devclaw.me/console)
- [ ] **Phase 8** — IDE plugins (VSCode / Cursor / Windsurf / JetBrains)
- [ ] **Phase 9** — Skill marketplace (publish/subscribe with star-energy ⚡)
- [ ] **Phase 10** — Abathur evolution engine (trajectory → distilled skills)
- [ ] **Phase 11** — Desktop (Tauri) + Mobile (Flutter) clients
- [ ] **Phase 12** — Enterprise fleet (Overlord orchestration, SSO, RBAC)

See [`devclaw.me/roadmap`](https://devclaw.me/roadmap) for the full panorama.

---

## Brand & domains

| Domain                        | Role                                                  |
|-------------------------------|-------------------------------------------------------|
| **`devclaw.me`**              | Primary product home, console, docs, downloads       |
| **`swarm.autos`**             | Swarm visualization showcase / demo / hackathon site |
| `drone.starclaw.net`          | Internal sub-domain in StarClaw enterprise stack     |

`drone` is the code name and binary name. `DevClaw` is the public brand.

---

## License

Apache-2.0. See [`LICENSE`](LICENSE).

DevClaw is part of the [StarClaw](https://starclaw.net) ecosystem.

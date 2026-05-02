# Contributing to DevClaw

Thanks for your interest in contributing! Please read the whole of this short
document before opening a PR — DevClaw uses a slightly unusual **hybrid monorepo
model** that affects how contributions land.

---

## TL;DR

- This repo (`github.com/yinhe/devclaw`) is a **publish mirror** of the
  DevClaw kernel. The canonical source lives in the private StarClaw
  monorepo, which also contains the enterprise integrations (Forge issue
  tracking, Pheromone eventing, Overlord orchestration) that are **not**
  open-sourced.
- You can still open issues and PRs here. Maintainers review, discuss,
  and **cherry-pick** accepted changes back into the monorepo. The next
  release then flows out to this mirror.
- This means your PR commit hash might not be the one that lands, but
  you will be credited in the release notes and in the squashed commit
  message (`Co-authored-by: …`).

If you want more context, see
[REPO-STRATEGY.md](https://devclaw.me/repo-strategy) (or `e:\devlcaw\REPO-STRATEGY.md`
in the private monorepo).

---

## Ways to contribute

### 1. File a bug report

Use the **Bug report** template. Include:
- Your OS / Go version / `drone version` output
- Exact command you ran
- Expected vs. actual behavior
- Minimal reproduction if possible

### 2. Propose a feature

Use the **Feature request** template. We especially want:
- New **tools** (things a drone can do — HTTP fetch, browser automation,
  screenshot, DB query, cloud API call, etc.)
- New **roles** (opinionated prompt + tool bundle)
- New **providers** (LLM backends — Anthropic native, Gemini, local llama.cpp)
- MCP server integrations

### 3. Open a pull request

Before opening a PR, please:
1. Discuss non-trivial changes in an issue first — saves you time if we
   already have an opinion.
2. Keep the PR focused: one topic per PR.
3. Match the existing code style (run `go fmt`, `go vet`, `goimports`).
4. Add tests for new behavior.
5. Update docs if the user-visible surface changed.

---

## Development setup

```bash
# 1. Clone
git clone https://github.com/yinhe/devclaw.git
cd devclaw

# 2. Build
go build -o drone ./cmd/drone

# 3. Test
go test ./...

# 4. Smoke-run (no LLM call needed)
./drone version
./drone roles
./drone help
```

Actually running a task requires an OpenAI-compatible LLM endpoint. The
easiest path is **local Ollama**:

```bash
# Install Ollama (https://ollama.com)
ollama pull qwen3-coder

# In another terminal
./drone run "list files in the current dir and describe what each does"
```

Or point `drone` at any OpenAI-compatible service:

```bash
export DRONE_API_KEY=sk-...
export DRONE_BASE_URL=https://api.star-ai.net/v1
export DRONE_MODEL=qwen3-coder-plus
./drone run "..."
```

---

## Code style

- **Go**: idiomatic Go. `go fmt`, `go vet` must pass. Prefer standard library
  over third-party deps. Error messages lowercase, no trailing punctuation.
- **Imports**: grouped stdlib / this-module / third-party with blank lines.
- **Tests**: table-driven, subtests with `t.Run`. Aim for parallel safety.
- **Commits**: Conventional Commits preferred (`feat: ...`, `fix: ...`,
  `docs: ...`, `chore: ...`). One logical change per commit.

---

## Security issues

Please **do not** open public issues for security bugs. Email
`security@starclaw.net` (PGP optional) or use GitHub's private vulnerability
reporting feature.

---

## Licensing

By submitting code, you agree to license it under the project's
[Apache-2.0](./LICENSE) terms.

---

## Code of Conduct

Be kind. Assume good intent. No harassment, discrimination, or personal
attacks. Maintainers will enforce this at their discretion — in practice it
almost never comes up.

---

## Why the hybrid model?

Good question. Here's the 30-second version:

- **Monorepo** (private) = the whole product, including the closed-source
  enterprise glue (Forge / Pheromone / Overlord).
- **This mirror** = just the reusable kernel. MIT-style developer-friendly,
  zero surprises.
- **Cherry-pick flow** = lets us accept community contributions without
  forcing us to open-source the commercial layer, and lets us maintain a
  single source of truth that always passes the full enterprise test suite.

This is the same model used by companies like HashiCorp, GitLab, and
(more recently) Cline — open kernel, commercial layer on top. It scales
better than either pure-open or pure-closed for our use case.

If that feels off-putting, no hard feelings — fork away. Apache-2.0 lets you.

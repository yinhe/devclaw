---
name: Bug report
about: Something isn't working the way the docs say it should
title: "[bug] "
labels: ["bug", "triage"]
assignees: []
---

## Summary
<!-- One sentence. What's broken? -->

## Environment
- **OS**:                  <!-- e.g. Windows 11 24H2 / macOS 15.4 / Ubuntu 24.04 -->
- **Architecture**:        <!-- x86_64 / arm64 -->
- **Go version**:          <!-- `go version` — only if building from source -->
- **drone version**:       <!-- paste `drone version` output -->
- **LLM backend**:         <!-- StarAI / Ollama qwen3-coder / OpenAI gpt-4o / other -->
- **Installed via**:       <!-- release archive / go install / built from source -->

## Steps to reproduce
<!-- Minimum command + config -->
```bash
drone run --role dev --task "..."
```

## Expected behavior
<!-- What should have happened -->

## Actual behavior
<!-- What actually happened. Paste full output if practical, or attach as file -->

<details>
<summary>Trajectory (if available)</summary>

```
# Contents of the trajectory log if you ran with `--trajectory ./traces`
```

</details>

## Additional context
<!-- Anything else: workarounds tried, adjacent issues, screenshots, etc. -->

---

- [ ] I searched existing issues and this isn't a duplicate.
- [ ] I've included enough info to reproduce the bug.

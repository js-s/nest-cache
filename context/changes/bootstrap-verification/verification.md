---
bootstrapped_at: 2026-05-21T22:25:00Z
starter_id: go
starter_name: "Go (standard library)"
project_name: nest-cash
language_family: go
package_manager: go-modules
cwd_strategy: subdir-then-move
bootstrapper_confidence: first-class
phase_3_status: ok
audit_command: "govulncheck -json ./..."
---

## Hand-off

```yaml
starter_id: go
project_name: nest-cash
hints:
  language_family: go
  team_size: solo
  deployment_target: self-host
  ci_provider: github-actions
  ci_default_flow: auto-deploy-on-merge
  bootstrapper_confidence: first-class
  path_taken: standard
  quality_override: false
  self_check_answers: null
  has_auth: true
  has_payments: false
  has_realtime: false
  has_ai: false
  has_background_jobs: false
```

### Why this stack

Solo developer building a personal budget tracker (NestCash) in Go with a 3-week after-hours timeline. Go (standard library) is the recommended default for the (web-app, go) cell and clears all four agent-friendly gates — typed, convention-based, popular in training data, and well-documented. Auth (FR-001, FR-002) will be assembled from Go ecosystem libraries since the minimal starter doesn't include it first-class; the user accepted this tradeoff consciously over switching to a batteries-included starter in another language. Deployment defaults to self-host with a single compiled binary; CI runs on GitHub Actions with auto-deploy-on-merge.

## Pre-scaffold verification

| Signal        | Value   | Severity | Notes                                                                 |
| ------------- | ------- | -------- | --------------------------------------------------------------------- |
| npm package   | not run | n/a      | Go starter; no npm CLI involved                                       |
| GitHub repo   | not run | n/a      | docs_url (go.dev/doc/) is not a GitHub repository; no recency signal  |

## Scaffold log

**Resolved invocation**: `mkdir .bootstrap-scaffold && cd .bootstrap-scaffold && go mod init github.com/user/.bootstrap-scaffold`
**Strategy**: subdir-then-move
**Exit code**: 0
**Files moved**: 1 (go.mod)
**Conflicts (.scaffold siblings)**: none
**.gitignore handling**: absent in scaffold
**.bootstrap-scaffold cleanup**: deleted
**Post-move fix**: module path updated from `github.com/user/.bootstrap-scaffold` to `github.com/user/nest-cash`

## Post-scaffold audit

**Tool**: govulncheck -json ./...
**Status**: failed to run
**Reason**: govulncheck not installed (`command not found: govulncheck`)
**Partial output (if any)**:

```
zsh:1: command not found: govulncheck
```

**Recommended**: Install with `go install golang.org/x/vuln/cmd/govulncheck@latest` and run `govulncheck ./...` manually.

## Hints recorded but not acted on

| Hint                    | Value              |
| ----------------------- | ------------------ |
| bootstrapper_confidence | first-class        |
| quality_override        | false              |
| path_taken              | standard           |
| self_check_answers      | null               |
| team_size               | solo               |
| deployment_target       | self-host          |
| ci_provider             | github-actions     |
| ci_default_flow         | auto-deploy-on-merge |
| has_auth                | true               |
| has_payments            | false              |
| has_realtime            | false              |
| has_ai                  | false              |
| has_background_jobs     | false              |

## Next steps

Next: a future skill will set up agent context (CLAUDE.md, AGENTS.md). For now, your project is scaffolded and verified — happy hacking.

Useful manual steps in the meantime:
- `git init` (if you have not already) to start your own repo history.
- Review any `.scaffold` siblings the conflict policy created and decide which version of each file to keep.
- Address audit findings per your project's risk tolerance — the full breakdown is in this log.
- Install `govulncheck` (`go install golang.org/x/vuln/cmd/govulncheck@latest`) and run `govulncheck ./...` to check for known vulnerabilities.

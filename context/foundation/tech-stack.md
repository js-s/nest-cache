---
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
---

## Why this stack

Solo developer building a personal budget tracker (NestCash) in Go with a 3-week after-hours timeline. Go (standard library) is the recommended default for the (web-app, go) cell and clears all four agent-friendly gates — typed, convention-based, popular in training data, and well-documented. Auth (FR-001, FR-002) will be assembled from Go ecosystem libraries since the minimal starter doesn't include it first-class; the user accepted this tradeoff consciously over switching to a batteries-included starter in another language. Deployment defaults to self-host with a single compiled binary; CI runs on GitHub Actions with auto-deploy-on-merge.

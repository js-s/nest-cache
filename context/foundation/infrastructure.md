---
project: NestCash
researched_at: 2026-05-27
recommended_platform: Fly.io
runner_up: Railway
context_type: mvp
tech_stack:
  language: Go
  framework: standard library
  runtime: compiled binary
---

## Recommendation

**Deploy on Fly.io.**

Fly.io is the best fit for a Go compiled binary at MVP scale: it runs full Linux microVMs (closest to "self-host" philosophy declared in tech-stack.md), has first-class Go support with auto-detection, and costs ~$0.50-2/mo at near-zero traffic thanks to auto-stop machines. The excellent `flyctl` CLI covers the entire operational lifecycle without a browser. While it lacks MCP/agent integration (Railway wins there), the CLI's `--json` output mode and deterministic commands keep agent automation viable.

## Alternative Path: Self-Hosted VPS

The user explicitly wants a documented fallback: deploy the Go binary on a self-managed server exposed via DNS.

**How it works:**
- Provision a VPS (Hetzner CX22 ~€4.50/mo, DigitalOcean $6/mo, AWS Lightsail $3.50/mo)
- Deploy the compiled Go binary via SCP/rsync
- Run as a systemd service (auto-restart, logging via journald)
- Database: Postgres in Docker on the same VPS (docker-compose)
- Expose via DNS A-record pointing to VPS IP
- TLS: Let's Encrypt via Caddy or certbot

**Pros:** Full control, easy DB migration (just move the Docker volume), no vendor lock-in, lowest possible cost for always-on.
**Cons:** Manual OS patching, no managed rollbacks, no auto-scaling, agent must SSH to operate.

This path can be adopted at any time — the Go binary is the same artifact regardless of where it runs.

## Platform Comparison

### Scoring Matrix

| Criterion | Fly.io | Railway | Render | AWS App Runner |
|---|---|---|---|---|
| CLI-first maintenance | **Pass** | **Pass** | **Pass** | Partial |
| Managed / serverless | **Pass** | **Pass** | **Pass** | Partial |
| Agent-readable docs | Partial | Partial | Partial | Partial |
| Stable deploy API | **Pass** | **Pass** | **Pass** | **Pass** |
| MCP / agent integration | Fail | **Pass** | **Pass** | Partial |
| **Score (P=2, Partial=1, F=0)** | **8/10** | **9/10** | **9/10** | **7/10** |

### Hard Filters Applied

- Cloudflare Workers: **Dropped** — V8 isolates, cannot run Go binary
- Vercel: **Dropped** — serverless Node/Edge only
- Netlify: **Dropped** — serverless Node only

### Soft-Weight Adjustments

- Cost neutral (no strong preference) → Fly.io's auto-stop advantage preserved
- AWS familiarity → +tie-break for AWS (not enough to overcome complexity gap)
- Single region → no edge-native preference (neutralizes Cloudflare's advantage even if it supported Go)
- DB portability preference → favors platforms allowing arbitrary containers (Fly.io, Railway both pass)

### Shortlisted Platforms

#### 1. Fly.io (Recommended)

First-class Go support with `flyctl launch` auto-detecting `go.mod` and generating a multi-stage Dockerfile. Full Linux microVMs provide real container isolation with persistent processes, WebSocket support, and SSH access. Auto-stop machines reduce cost to near-zero at idle. Managed Postgres available co-located, or attach volumes for self-managed DB. CLI covers deploy, rollback, secrets, logs — full lifecycle without a browser.

**Key strengths:** Lowest cost at MVP traffic (~$0.50-2/mo), closest to "self-host" philosophy (real VMs), first-class Go guide, excellent CLI.
**Gap:** No MCP server or agent integration. Agent must parse CLI text output.

#### 2. Railway (Runner-up)

Best-in-class agent integration with `railway mcp install` (GA). One-click Postgres/Redis templates co-located in the same project. Railpack auto-detects Go, builds, and deploys. CLI is full-featured with JSON output. Simplest DX of all evaluated platforms.

**Key strengths:** First-class MCP/agent support, simplest onboarding, co-located services.
**Gap:** $5/mo minimum even at zero traffic. No auto-stop equivalent — always-on billing.

#### 3. AWS App Runner (Third)

Familiar platform for the user. Full container support, pause feature drops cost to $0 when unused. Enterprise-grade reliability and compliance. VPC connectors to reach RDS Postgres.

**Key strengths:** User familiarity, pause-to-$0, enterprise ecosystem.
**Gap:** IAM complexity, multi-tool CLI surface (aws + copilot), mediocre agent-readability of docs, MCP in preview only.

## Anti-Bias Cross-Check: Fly.io

### Devil's Advocate — Weaknesses

1. **No MCP server or agent integration.** Unlike Railway (`railway mcp install`) or Render (agent skills), Fly.io offers zero structured agent tooling. The AI assistant must parse CLI text output rather than calling typed tools.
2. **No free tier — credit card required upfront.** Railway offers a $5 trial credit without CC; Fly.io requires payment info before first deploy.
3. **Managed Postgres is a separate billing concern.** Pricing is less transparent than Railway's included-in-project model.
4. **Auto-stop cold starts (~200-500ms).** First request after idle wakes the VM. Acceptable for a personal app, but a gotcha for webhooks or time-sensitive background jobs.
5. **Aggressive deprecation velocity.** GPU Machines removed Aug 2025, "unmanaged Postgres" moved to unsupported. Migration pressure can arrive fast.

### Pre-Mortem — How This Could Fail

Six months after deploying NestCash on Fly.io: the initial deploy was smooth — `flyctl launch` auto-detected Go, generated a Dockerfile, deployed in minutes. But three things compounded into pain. First, the managed Postgres instance hit connection limits during a volume expansion, and support took 48 hours (community-forum-first, no SLA on the cheap tier). Data was temporarily inaccessible. Second, adding a background cron job revealed that auto-stop semantics don't mix with scheduled work — a separate always-on machine or external scheduler was needed, adding complexity the $2/mo price point didn't suggest. Third, when the agent needed to automate a rollback during a broken deploy, it parsed `flyctl releases` text output — misread a version number and rolled back to the wrong release. The combination of "no agent tooling" + "operational surprises" + "workarounds for background jobs" turned a simple MVP into a maintenance headache.

### Unknown Unknowns

- Volumes are billed even when machines are stopped ($0.15/GB/mo regardless of machine state).
- IPv4 addresses are a paid add-on ($2/mo each). Fly defaults to shared IPv4 + IPv6.
- Internal `.internal` DNS doesn't work outside Fly's network — local dev needs `flyctl proxy` to reach deployed Postgres.
- Region selection matters: app in Warsaw (waw) + Postgres in Amsterdam (ams) = 10-20ms RTT per DB query.
- Fly.io's support model is community-first (forum). Paid support plans exist but aren't cheap.

## Operational Story

- **Preview deploys**: No built-in PR preview. Workaround: deploy to a separate Fly app per branch (`flyctl deploy --app nestcash-preview-<branch>`). Clean up manually.
- **Secrets**: `flyctl secrets set KEY=value` — stored encrypted in Fly's vault, injected as env vars at runtime. Visible via `flyctl secrets list` (names only, not values). Rotation: `flyctl secrets set` again with new value → triggers redeploy.
- **Rollback**: `flyctl releases rollback` — reverts to previous release image. Typical time-to-revert: ~30 seconds. Caveat: DB migrations don't roll back automatically — schema changes need manual revert.
- **Approval**: Human-only operations: delete app, remove volumes, change billing plan. Agent may: deploy, rollback, set secrets, scale machines, tail logs.
- **Logs**: `fly logs` (live tail), `fly logs --app nestcash` from any terminal. JSON log format available. No built-in log retention beyond real-time — export to external service for history.

## Risk Register

| Risk | Source | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| Agent misparses CLI output during rollback | Devil's advocate | Low | High | Use `--json` flag on all flyctl commands; validate version IDs before acting |
| Managed Postgres outage with slow support response | Pre-mortem | Low | High | Run self-managed Postgres on a Fly Machine with volume backup; or use external DB (Supabase, Neon) |
| Auto-stop cold start delays future webhook integrations | Devil's advocate | Medium | Low | Set `auto_stop_machines = false` for the app machine when background jobs are added; accept higher cost |
| Volume billed while machine stopped (hidden cost) | Unknown unknowns | Medium | Low | Right-size volumes; monitor billing; start with 1GB |
| IPv4 add-on needed for external API callbacks | Unknown unknowns | Low | Low | Budget $2/mo for dedicated IPv4 when needed; use shared IPv4 initially |
| Feature deprecation forces migration | Devil's advocate | Low | Medium | Pin to GA features only; avoid beta/preview offerings; monitor Fly.io changelog |
| Region mismatch between app and DB adds latency | Unknown unknowns | Low | Low | Deploy both app and DB in the same region (waw or fra) |

## Getting Started

1. **Install flyctl:**
   ```bash
   curl -L https://fly.io/install.sh | sh
   ```

2. **Authenticate:**
   ```bash
   flyctl auth signup   # or: flyctl auth login
   ```

3. **Launch the app** (from project root with `go.mod`):
   ```bash
   flyctl launch --name nest-cash --region waw
   ```
   This auto-detects Go, generates a `Dockerfile` and `fly.toml`, and deploys.

4. **Set up Postgres** (co-located):
   ```bash
   flyctl postgres create --name nest-cash-db --region waw --vm-size shared-cpu-1x --volume-size 1
   flyctl postgres attach nest-cash-db --app nest-cash
   ```
   This injects `DATABASE_URL` as a secret into the app.

5. **Deploy subsequent changes:**
   ```bash
   flyctl deploy
   ```

## Out of Scope

The following were not evaluated in this research:
- Docker image configuration (Dockerfile will be created during bootstrap/implementation)
- CI/CD pipeline setup (GitHub Actions auto-deploy-on-merge — separate task)
- Production-scale architecture (multi-region, HA, DR)
- Database schema design or migration tooling
- Frontend deployment (separate repo/deployable)

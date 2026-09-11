---
project: NestCash
researched_at: 2026-09-11
recommended_platform: Fly.io (single application image)
runner_up: Cloudflare Pages + Fly.io API
context_type: mvp
tech_stack:
  frontend_language: TypeScript
  frontend_framework: Vue 3 + Vite 8.2
  backend_language: Go 1.22
  backend_framework: standard library
  runtime: multi-stage container
  database: Neon PostgreSQL
---

## Recommendation

**Deploy NestCash as one Fly.io application image containing the Go API and the built Vue SPA.**

This is technically supported: Fly.io runs arbitrary Docker images, and its static-site guidance confirms that static assets can be shipped in the same image as the web server. The Go server now has an explicit API boundary, static-file serving, and a Vue history fallback; the artifact-first Dockerfile copies the CI-built `web/dist` into the runtime image. For this MVP, that small code change is simpler than operating a second frontend platform.

The decision is driven by the stated priorities: fastest path, fewest moving parts, one region, an existing Fly.io deployment, and Neon remaining the database. Serving the SPA and API from one origin also keeps registration and session-based authentication inside the backend without cross-origin cookie and CORS configuration. The trade-off is that frontend and backend releases become coupled and static assets do not receive a global CDN.

The CI path now removes the registry blocker without changing the committed lockfile: GitHub Actions uses Node 22, rewrites only the checkout's `cdproxy.sportradar.online` tarball URLs to public npm, runs `npm ci` and `npm run build`, scans the public artifact for server-secret names, and uploads `web/dist` before the Fly deploy job. No npm token is stored in the repository. The deploy job downloads that artifact before Fly builds the image, and the runtime image fails fast if `web/dist/index.html` is absent.

## Platform Comparison

The decision record is intentionally limited to the three viable options requested. All three use Neon; the two split options retain the existing Go API on Fly.io.

### Scoring Matrix

| Deployment shape | CLI-first | Managed / serverless | Agent-readable docs | Stable deploy API | MCP / first-class integration | Score |
|---|---:|---:|---:|---:|---:|---:|
| Fly.io: one Go + Vue image | Pass | Pass | Pass | Pass | Partial | 9/10 |
| Cloudflare Pages + Fly.io API | Pass | Pass | Pass | Pass | Pass | 10/10 |
| Render Static Site + Fly.io API | Pass | Pass | Pass | Pass | Pass | 10/10 |

The base criteria score alone does not decide this case. The user's weighting gives substantial value to one origin, one deployment, the existing Fly.io account, and keeping Neon unchanged. Those factors move the unified Fly.io shape ahead of the two technically stronger-but-split static-hosting options.

### Shortlisted Platforms

#### 1. Fly.io — Recommended

Fly.io can build and run the combined container in the existing `fra` region. The existing `fly.toml` already has HTTPS, auto-start, auto-stop, a 256 MB shared machine, and an explicit `/healthz` check. A `shared-cpu-1x` machine is listed at approximately $2.24/month while continuously running; auto-stop removes CPU/RAM charges while stopped, leaving root filesystem charges. One release carries the API and frontend, one hostname avoids CORS for normal browser calls, and the current GitHub Actions deployment remains the production entry point. Fly also provides an official local MCP server for supported administration, logs, status, and machine operations; deployment and image rollback remain explicit CLI workflows.

Evidence: [Dockerfile deployments](https://fly.io/docs/languages-and-frameworks/dockerfile/), [static websites](https://fly.io/docs/languages-and-frameworks/static/), [resource pricing](https://fly.io/docs/about/pricing/), [rollback guide](https://fly.io/docs/blueprints/rollback-guide/).

#### 2. Cloudflare Pages + Fly.io API

Cloudflare Pages is the strongest split frontend option. Vue builds with `npm run build` into `dist`; Git-connected projects provide PR preview URLs, static asset requests are free, and Cloudflare publishes managed MCP servers for API and build-management tools. MCP server portals are still explicitly **open beta** as of the research date, while the individual managed servers have no beta label in the current documentation. Pages Functions use the Workers runtime rather than running the existing Go binary, so the API remains on Fly.io. That creates two deployment control planes and cross-origin authentication concerns: credentialed requests need an explicit origin, `Access-Control-Allow-Credentials`, suitable cookie attributes, and CSRF protection. Preview URLs are public by default and must be protected or kept away from production financial data.

Evidence: [Vue on Pages](https://developers.cloudflare.com/pages/framework-guides/deploy-a-vue-site/), [build configuration](https://developers.cloudflare.com/pages/configuration/build-configuration/), [preview deployments](https://developers.cloudflare.com/pages/configuration/preview-deployments/), [Pages limits and pricing](https://developers.cloudflare.com/pages/platform/limits/), [Cloudflare MCP servers](https://developers.cloudflare.com/agents/model-context-protocol/cloudflare/servers-for-cloudflare/), [MDN CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS).

#### 3. Render Static Site + Fly.io API

Render offers a free static-site product with a global CDN, automatic Git deployments, custom domains, CLI deployment, and an official MCP server. It is a credible low-cost fallback for the SPA while retaining the Go API on Fly.io. Its disadvantages are the same split-auth and two-deployment costs as Cloudflare, plus no existing account familiarity and workspace quotas for included bandwidth and pipeline minutes. It provides no material advantage over Cloudflare for this repository.

Evidence: [Render static sites](https://render.com/docs/static-sites), [Docker services](https://render.com/docs/docker), [Render CLI](https://render.com/docs/cli), [Render MCP](https://render.com/docs/mcp-server), [Render pricing](https://render.com/pricing).

## Anti-Bias Cross-Check: Fly.io Unified Image

### Devil's Advocate — Weaknesses

1. The current Go server is not a static-file server. A catch-all route added too early can return `index.html` for missing `/api` endpoints, hiding backend failures behind an apparently valid HTML response.
2. Frontend and backend releases become coupled. A broken Vite build blocks an API release, while a frontend-only fix still republishes the backend image.
3. Assets are served by the Fly machine rather than a global CDN. This is acceptable for one-region, low-volume use, but it is a weaker performance shape than Pages or Render.
4. `auto_stop_machines = "stop"` can add a cold start to the first request after inactivity. That needs to be checked against the product's two-second response target.
5. Fly's official local MCP server covers a useful but incomplete subset of platform operations; deployment and image rollback still require explicit CLI commands. Automation should use the CLI's JSON output and human approval for destructive actions.

### Pre-Mortem — How This Could Fail

Six months after deployment, the single-image design looks economical but has accumulated avoidable coupling. The first failure happens in CI: the build uses an old Node image, while Vite 8 requires Node `^20.19.0` or `>=22.12.0`. After that is corrected, the remote builder cannot fetch the package-lock dependencies because they resolve to the private `cdproxy.sportradar.online` registry. A rushed workaround changes the registry without a controlled credential path. Later, a broad Vue fallback is placed before `/api`, so failed auth requests return `index.html` and the UI reports misleading login errors. A production fix includes a database migration; rolling back the old image restores the code but not the schema. Finally, auto-stop makes the first request visibly slower, and the lack of automatic PR previews lets a routing regression reach the main branch. The one-image plan did not fail because Fly could not host it; it failed because build access, route boundaries, migration discipline, and release coupling were treated as implementation details.

### Unknown Unknowns

- Vite 8.2.2's lockfile requires Node `^20.19.0` or `>=22.12.0`; a generic older Node image is not valid. The current Cloudflare Pages build image lists Node 22.16.0, but the Fly Docker build must pin a compatible Node 22 image itself.
- `web/package-lock.json` contains resolved tarballs from `cdproxy.sportradar.online` and the repository has no committed `.npmrc`. The production CI workflow handles this in its ephemeral checkout; any future Dockerfile that installs npm dependencies inside the Fly builder must instead consume the uploaded `web/dist` artifact or repeat the same controlled normalization.
- Same-origin sessions are simple now, but moving the SPA to another host later changes the auth contract: browser requests need credentials enabled, exact CORS origins, compatible `SameSite`/`Secure` cookies, and CSRF controls.
- Fly rollback restores an image, not database state, current secrets, or current `fly.toml`; images may also be removed from Fly's registry after retention. The official `fly mcp server --cursor` integration does not remove this distinction.
- The immutable frontend files belong in the image. User data or future uploads must not be written to Fly's root filesystem, which resets across machine replacement.

## Operational Story

- **Preview deploys**: Fly.io has no automatic PR preview equivalent to Pages by default. Use a separate Fly app per pull request, or the documented `superfly/fly-pr-review-apps` GitHub Action, and never point preview apps at production Neon data. A separate Neon branch or a no-database preview is required for financial-data privacy.
- **Secrets**: Store `DATABASE_URL`, session-signing keys, and other runtime secrets with `fly secrets set`; Fly exposes only names and digests through `fly secrets list`, not plaintext values. Keep `FLY_API_TOKEN` in GitHub Actions secrets. `VITE_API_BASE_URL=/api` is build configuration, not a secret, and is embedded into the browser bundle.
- **Rollback**: Run `fly releases --app nest-cash --image`, select a known-good image, then redeploy it with `fly deploy --app nest-cash --image <image>`. The image rollback is normally fast, but it does not reverse Neon migrations, secrets, or configuration; validate with `fly status` and `fly logs` afterward.
- **Approval**: A human approves the production merge that triggers the existing main-branch workflow, destructive Neon migrations, deletion of the Fly app or any volume, billing changes, and rotation of the primary session/database secret. An agent may build a preview, deploy an approved image, inspect status/logs, and perform a non-destructive rollback when explicitly authorized.
- **Logs**: Read runtime logs with `fly logs --app nest-cash --json --no-tail` or tail them with `fly logs --app nest-cash --json`; inspect deployment state with `fly status --app nest-cash --json`. The official local `fly mcp server --cursor` can expose supported logs/status operations, but the documented workflow remains CLI-first so deployment and rollback are explicit. Read GitHub Actions logs for the web build and Fly deploy pipeline.

## Risk Register

| Risk | Source | Likelihood | Impact | Mitigation |
|---|---|---:|---:|---|
| Private npm registry is unreachable from the Fly remote builder | Research finding / Unknown unknowns | Low | High | Build `web/dist` in GitHub Actions with the ephemeral public-npm lockfile normalization and pass that artifact into the image; do not make the Fly builder install npm dependencies. |
| SPA fallback masks `/api` errors | Devil's advocate | Medium | High | Reserve `/api` and `/healthz` before the catch-all, return API 404s as JSON, and add a small route self-check covering API and Vue history URLs. |
| Vite/Node version drift breaks the image build | Unknown unknowns | Medium | Medium | Pin Node 22.16.0 and run `npm ci && npm run build` in the GitHub Actions web-build job before `fly deploy`. |
| Frontend and backend releases are coupled | Devil's advocate / Pre-mortem | Medium | Medium | Keep a single smoke-test gate, use immutable image labels, and allow separate frontend-only deployment only if coupling becomes a real bottleneck. |
| Rollback leaves Neon schema ahead of the application | Pre-mortem / Fly rollback finding | Low | High | Use additive, forward-compatible migrations, back up Neon, and test migration/rollback sequences before production changes. |
| Auto-stop cold start exceeds the response target | Devil's advocate | Medium | Low | Measure the first request in Frankfurt; keep one machine running if the measured user experience is unacceptable. |
| MCP coverage changes or omits a required deployment operation | Devil's advocate / Research finding | Low | Medium | Treat `flyctl` JSON commands as the operational source of truth; use the local MCP only for supported read and administration tasks. |
| Preview deployment exposes or mutates personal financial data | Operational finding | Medium | High | Use a separate Neon branch/database or disable DB access in preview; do not copy production secrets into review apps. |
| Long-term rollback image is no longer retained | Unknown unknowns | Low | Medium | Tag releases with commit IDs and keep important images in GHCR or another controlled registry. |

## Getting Started

1. **Use the CI frontend build** already defined in `.github/workflows/fly.yml`: it pins Node 22.16.0, normalizes the mirror URLs only in the ephemeral runner workspace, and runs `npm ci && npm run build` against public npm. Keep the committed lockfile unchanged.
2. **Use the Go HTTP boundary**: `/api` is reserved for JSON endpoints, `/healthz` remains public, `web/dist` assets are served directly, and `index.html` is used only as the fallback for non-API browser routes.
3. **Use the artifact-first image**: the existing Go 1.22 builder creates the binary, while the runtime stage receives the CI-built `web/dist` and fails if its `index.html` is missing. Validate the image locally with `docker build` and `docker run -p 8080:8080` when a container daemon is available.
4. **Use same-origin API configuration** by building the SPA with `VITE_API_BASE_URL=/api`; keep database and session secrets runtime-only in Fly.
5. **Deploy the existing Fly app** from the repository root with `fly deploy --remote-only`, then verify `fly status`, `/healthz`, authentication, and a Vue history route before enabling the production merge workflow.

## Out of Scope

The following remain outside this implementation:

- Database schema, Neon branching, auth/session implementation, or migration tooling.
- Production-scale architecture, multi-region deployment, high availability, or disaster recovery.

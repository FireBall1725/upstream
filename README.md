# Upstream

Upstream watches a GitOps repo and tells you which of your apps are behind. It reads every app directory the way an Argo CD ApplicationSet does, finds each pinned image tag and Helm chart version, and checks the registries, Helm repos and GitHub releases for anything newer. When you pick the updates you want, it opens one pull request for the lot.

It's one Go binary with the web UI built in. Run one container with a small volume and it scans on a cron schedule.

It scans on a schedule or on demand, renders each app the way Argo CD does to catch rollouts that would hang, checks every pinned image and chart against its registry or Helm repo, lists the updates and the repo hygiene problems it finds, and opens the bump PRs you pick. [docs/plan.md](docs/plan.md) has the design.

## What a bump PR does

Each PR gets its own fresh clone. An image bump rewrites the tag line (quoted), keeps `appVersion` in step and adds one to the chart's own patch version. A chart bump rewrites the dependency's version and runs `helm dependency update`, so `Chart.lock` and the vendored `charts/*.tgz` match. Then Upstream re-reads the clone and refuses to push if a picked pin doesn't read its new version, anything else moved, a file outside the picked apps changed, or a touched chart no longer renders with `helm template`. Majors get their own PR unless you untick that, and auto-merge is off unless you tick it.

## Running it

```sh
docker run -p 8080:8080 -v upstream-data:/data \
  -e UPSTREAM_REPO=https://github.com/you/your-gitops-repo.git \
  -e TZ=America/Toronto \
  ghcr.io/fireball1725/upstream:nightly
```

| Env var | Default | What it does |
|---|---|---|
| `UPSTREAM_REPO` | none, required | Git URL of the GitOps repo to scan |
| `UPSTREAM_BRANCH` | `main` | Branch to scan and to open PRs against |
| `UPSTREAM_APP_GLOB` | `apps/*/*` | One match is one app; the second path segment is the namespace |
| `UPSTREAM_SCHEDULE` | `0 */6 * * *` | Cron expression for scans, in `TZ` |
| `GITHUB_TOKEN` | none | Reads release notes and opens bump PRs |
| `UPSTREAM_GIT_AUTHOR_NAME` | none | Author of bump commits |
| `UPSTREAM_GIT_AUTHOR_EMAIL` | none | Author email of bump commits |
| `UPSTREAM_DATA_DIR` | `/data` in the image | SQLite database, the repo clone, and work clones for PRs |
| `UPSTREAM_ADDR` | `:8080` | Listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

For bump PRs, use a fine-grained token scoped to the GitOps repo only, with Contents and Pull requests set to read and write.

There's no login. Put it behind an internal-only ingress or a VPN.

`/metrics` serves Prometheus gauges: `upstream_pins{update=...}`, `upstream_hygiene_findings{severity=...}`, `upstream_open_prs`, `upstream_last_scan_success` and `upstream_last_scan_timestamp_seconds`.

## Developing

```sh
cd web && npm ci && npm run build   # writes the UI into internal/ui/dist
cd .. && go run ./cmd/upstream
```

For UI work, run `npm run dev` in `web/` alongside the Go server; Vite proxies `/api` to `localhost:8080`.

## Support

Questions, updates, and works in progress: [FireBall Codes on Discord](https://discord.gg/QpV82CFfVD).

If this saved you some time, you can [buy me a sushi roll](https://ko-fi.com/fireball1725).

## Licence

AGPL-3.0-only. Copyright (C) 2026 FireBall1725.

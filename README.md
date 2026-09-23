# Upstream

Upstream watches a GitOps repo and tells you which of your apps are behind. It reads every app directory the way an Argo CD ApplicationSet does, finds each pinned image tag and Helm chart version, and checks the registries, Helm repos and GitHub releases for anything newer. When you pick the updates you want, it opens one pull request for the lot.

It's one Go binary with the web UI built in. Run one container with a small volume and it scans on a cron schedule.

**Status:** early. The server, the UI shell and the release pipeline work. Scanning and bump PRs are being built now; [docs/plan.md](docs/plan.md) has the order and the design.

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
| `UPSTREAM_DATA_DIR` | `/data` in the image | SQLite database and the repo clone |
| `UPSTREAM_ADDR` | `:8080` | Listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

For bump PRs, use a fine-grained token scoped to the GitOps repo only, with Contents and Pull requests set to read and write.

There's no login. Put it behind an internal-only ingress or a VPN.

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

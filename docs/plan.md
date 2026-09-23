# Upstream: build plan

Upstream reads a GitOps repo (for us, `FireBall1725/homelab-applications`), lists every app the ApplicationSet deploys, and checks each one for a newer image tag, chart version or GitHub release on a cron. It can open a bump PR for one app or a batch of them. It ships as one Go binary with the web UI built in, deployed as one pod.

Mockup: https://claude.ai/artifact/LgzXkv5WaoXcap1QQ5HawJ

## What a scan of the real repo showed

A throwaway probe on 2026-09-22 against `origin/main` at `f64f9b5` found 58 apps: 36 with updates (10 major, 19 minor, 6 patch, 1 linuxserver rebuild), 13 current and 9 that can't be checked. The design below is built around what that probe got wrong:

- open-webui and renovate each have more than 21,000 tags. Listing them is slow and a capped listing returned an older version as "latest". GitHub releases answered both in one call.
- tautulli has a stray `2021.12.16` tag that sorts above `2.16.1` if you only compare numbers.
- linuxserver tags end in `-lsNN`, and `ls25` vs `ls41` has to compare as numbers, not text.
- Librarium runs nightlies, FireBin runs rcs, prowlarr runs `-nightly` and music-assistant has shipped betas (`2.9.0b3`). A nightly compared against stable tags always looks out of date.
- The GHCR anonymous token request needs `service=ghcr.io`, and `lscr.io` is `ghcr.io/linuxserver` underneath.

## Nothing specific to our homelab is hardcoded

Other people may run this, so everything tied to our setup is config, read from environment variables with these defaults:

| Setting | Env var | Our value |
|---|---|---|
| Repo | `UPSTREAM_REPO` | `https://github.com/FireBall1725/homelab-applications.git` |
| Branch | `UPSTREAM_BRANCH` | `main` |
| App directories | `UPSTREAM_APP_GLOB` | `apps/*/*` |
| Scan schedule | `UPSTREAM_SCHEDULE` | `0 */6 * * *` |
| Time zone | `TZ` | `America/Toronto` |
| Commit author | `UPSTREAM_GIT_AUTHOR_NAME`, `UPSTREAM_GIT_AUTHOR_EMAIL` | FireBall1725 and the GitHub noreply address, until a bot account exists |
| GitHub token | `GITHUB_TOKEN` | fine-grained PAT, see Deploy |
| Data directory | `UPSTREAM_DATA_DIR` | `/data` |

The namespace is taken from the path segment the glob puts there, the same way the ApplicationSet does it. A repo laid out differently only needs a different glob.

## Stack

| Part | Choice | Why |
|---|---|---|
| Server | Go, stdlib `net/http` and its routing patterns | House style, same as Librarium and FireBin |
| UI | React 19, TypeScript, Vite, Tailwind v4, i18next in `web/` | House style; the build output is embedded with `go:embed` |
| Storage | SQLite through `modernc.org/sqlite` (pure Go, no cgo), raw SQL, golang-migrate | See decision 1 |
| Schedule | `robfig/cron/v3` | Real cron expressions, so the schedule reads the same as the mockup's `0 */6 * * *` |
| Registries | `google/go-containerregistry` | Does the token handshake for Docker Hub, GHCR, lscr.io and any other OCI registry, including OCI Helm charts like n8n |
| Versions | `Masterminds/semver/v3` plus our own parsers | Helm uses the same library; four-part, calver and `-lsNN` tags need custom handling on top |
| Git and Helm | `git` and `helm` binaries in the image | Chart bumps need `helm dependency update`, because `Chart.lock` and `charts/*.tgz` are committed |

No ORM, no router library, no web framework, same as the other Go repos.

## Repo layout

```
cmd/upstream/main.go       config, DB, migrations, scheduler, HTTP server
internal/config/           env parsing
internal/version/          ldflags-injected, 0.0.0-dev locally
internal/inventory/        walks apps/*/*, finds every pinned version and its file and line
internal/versions/         tag parsing, channels, "is this newer" rules
internal/sources/          registry tags, Helm index.yaml, GitHub releases
internal/scan/             runs a scan, bounded concurrency, writes results
internal/hygiene/          the repo hygiene checks
internal/bump/             edits files, runs helm, verifies, pushes, opens PRs
internal/repository/       all SQL
internal/db/migrations/    NNNNNN_name.up.sql and .down.sql
internal/api/              router.go and handlers
web/                       the React app; dist/ is embedded
```

## How it works

### Inventory

Every directory matching `apps/*/*` is one app, the same glob the ApplicationSet uses, so the list can't drift from what Argo deploys. For each app the inventory records every version pin and where it lives, down to the line number, because the bump step edits that exact line:

1. `values.yaml` `image.tag`, including nested blocks like `home-assistant.image.tag`
2. `Chart.yaml` `appVersion`, when the values tag is empty or missing (the linuxserver media apps, blocky, homer, zigbee2mqtt, music-assistant)
3. `Chart.yaml` `dependencies[].version` for upstream charts, skipping `common`
4. `image:` lines in raw manifests (cloudflare-ddns, unpoller)

An app with none of these (ollama is only a Service and Endpoints) is listed as "not a workload".

### Comparing versions

The current tag decides the rules for its candidates. A candidate has to match the current tag's shape: same `v` prefix, same number of numeric parts, same suffix family (`-alpine`, `-lsNN`) and same channel (stable, rc, beta, nightly). Calver and semver never compare against each other, which is what drops the tautulli date tag. Floating tags (`latest`, a bare `3`) are marked "can't check" and show up on the hygiene tab.

Update type comes from the first part that changed. A change only in `-lsNN` is a "rebuild". For a chart the badge takes the bigger of the chart bump and the app bump inside it, so n8n (chart 2.0.1 to 2.1.1, app 1.x to 2.x) reads as major.

### Sources

Registry tags are the default for images and `index.yaml` for Helm repos. An app switches to GitHub releases when its registry has more than 5,000 tags or when a per-app rule says so. The GitHub repo for release notes comes from the image's `org.opencontainers.image.source` label, the chart's `sources:` list, or a manual rule, in that order.

Each scan fetches every source once, 8 at a time, and caches by source so apps sharing an image don't repeat the call. An error on one app gets recorded against that app and the scan carries on.

### Storage

SQLite at `/data/upstream.db` on a small Longhorn PVC. Everything a scan produces can be rebuilt by running another scan. The only data that can't is per-app rules (channel, tag filter, skipped versions, GitHub repo), settings and the PR log.

### Bump PRs

The UI sends the ticked apps. The server works from a fresh `origin/main` worktree under `/data/work/`, and for each app:

- **Image in values.yaml:** rewrites the tag line (quoted), sets `appVersion` to match and bumps the chart `version` patch number. Also updates the README if it names the old version. That's the same three-file change as your manual commits (esphome #181) and the shared `deploy-bump.yml`.
- **Image in appVersion only:** `appVersion`, chart `version` and README.
- **Upstream chart:** rewrites that dependency's version line, runs `helm dependency update` to regenerate `Chart.lock` and `charts/*.tgz`, then bumps the chart `version`.
- **Raw manifest:** rewrites the `image:` line.

Edits are line-anchored text rewrites, never a YAML round-trip. `deploy-bump.yml` does the same for the same reason: a round-trip reflows the whole file and a one-line bump becomes a 100-line diff.

Before pushing, the server re-runs the inventory on the worktree and checks every touched pin now reads the new version and nothing else moved. It also runs `helm template` on every touched chart. A failure stops that PR and shows the error in the UI. Nothing gets pushed.

Majors go in their own PR unless you untick that box. Commits carry DCO sign-off and no AI attribution. The PR body is a table of each app, its old and new version, and a link to the release notes.

The PR panel has an "Auto-merge" checkbox, unticked by default. Ticked, the server merges the PR right after opening it (squash, delete branch). homelab-applications has no CI, so there's nothing to wait for.

### Scheduling

One cron entry runs the scan, `0 */6 * * *` by default, in `America/Toronto`. "Scan now" runs the same job. Only one scan runs at a time, and a second request while one is running gets that scan's status back.

### API

JSON under `/api/`, used only by the embedded UI: apps, one app, hygiene, scans, start a scan, PRs, open PRs, per-app rules, skip a version. `/healthz` answers 200 when the DB opens and the last scan didn't fail completely. `/metrics` exposes update counts by type, so kube-prometheus-stack can alert later without Upstream needing its own notification code.

## Deploy

- Repo `FireBall1725/upstream`, public, AGPL-3.0-only, scaffolded with `~/Repos/workflows/scripts/scaffold.sh`.
- CI and releases through the shared workflows `@v1`. `ci-node.yml` has no `working-directory` input, so the `web/` checks need one added to the shared workflow first.
- Image `ghcr.io/fireball1725/upstream`, multi-stage: Node builds `web/dist`, Go cross-compiles with `--platform=$BUILDPLATFORM`, final stage is Alpine with `git` and `helm`.
- Chart at `homelab-applications/apps/app-upstream/upstream/` on firelabs-helm-common, the same as the other apps. Internal ingress class only at `upstream.k8s.firekatt.ca`, with the Kuma probe annotation pointed at `/healthz`. 1 Gi PVC for `/data`.
- GitHub token as a SealedSecret: a fine-grained PAT scoped to `homelab-applications` alone, with Contents and Pull requests read/write. It's separate from `HOMELAB_DEPLOY_TOKEN` so either can be revoked without breaking the other.
- Upstream will list itself, since it lives under `apps/`.

## Build order

Each step is its own PR in the new repo and ends with something that runs.

1. **Scaffold.** Repo, CI, Dockerfile, version package, Vite app embedded in the binary. Done when `docker run` serves a blank page with the right version in it.
2. **Inventory and hygiene.** Done 2026-09-23, with a "Scan now" button and the cron schedule pulled forward; results live in memory until step 4 adds SQLite. Tested against a snapshot of homelab-applications at `f64f9b5`. Done when it finds 58 apps and the same pins and hygiene items the mockup shows.
3. **Version rules.** Done 2026-09-23. Table tests built from every trap in the list above. Done when the tautulli, open-webui, lidarr, nightly and beta cases all resolve correctly.
4. **Sources, scan, storage, schedule, API.** Done 2026-09-23 except SQLite; results and history are in memory. Registries are always listed in full rather than switching to GitHub releases above 5,000 tags, because linuxserver apps need their `-lsNN` tags (radarr has 15,824) and a full scan still takes about 4 seconds. HTTP clients tested against recorded responses. Done when a live scan agrees with the 2026-09-22 probe, apart from releases that came out since.
5. **UI.** Done 2026-09-23, ported from the mockup; the Open PR button is disabled until step 6. The mockup rebuilt in React against the real API.
6. **Bump PRs.** Edit engine, helm step, verification, GitHub PR, auto-merge checkbox. Tested on a local bare clone first. The first live PR is one patch (spoolman 0.26.0 to 0.26.1) for you to review.
7. **Deploy.** Chart, SealedSecret and ingress in homelab-applications, then the first rc. One rc a day at most.
8. **Later.** Alert rules on `/metrics`, Discord or Home Assistant notices, and per-app rule editing beyond the basics.

Nothing gets pushed Monday to Friday between 07:30 and 17:00 ET. Work done in that window stays as local commits until 17:00.

## Decisions (settled 2026-09-22)

1. **SQLite, not Postgres with River.** A deliberate break from house style. The data is a few hundred rows, almost all rebuilt by the next scan, and the only job is one cron entry. Recorded in the house-style skill as the exception for single-pod tools.
2. **Commit author is config.** FireBall1725 for now; a bot account later only changes two env vars.
3. **Auto-merge is a checkbox on each PR,** unticked by default.
4. **No work-hours queue in the app.** Other people may run it, and homelab bumps aren't a work concern.
5. **No login for now.** Internal ingress only, the same as basic-memory-viewer.

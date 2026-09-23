# Working on Upstream

Read this before changing code. It's short on purpose.

## What this is

A self-hosted update tracker for GitOps repos. One Go binary (stdlib `net/http`, SQLite through `modernc.org/sqlite`, raw SQL) serves a JSON API under `/api/` and the React UI that Vite builds into `internal/ui/dist`. The design and build order are in `docs/plan.md`.

## Layout

- `cmd/upstream/main.go` wires config, the scan service, the cron schedule and the HTTP server, nothing else.
- `internal/config/` reads env vars. Nothing defaults to a particular homelab; our values live in the chart in homelab-applications.
- `internal/inventory/` walks the app glob and records every pinned version with its file and line, plus the hygiene findings. It reads vendored `charts/*.tgz`, because that is what Helm renders, not what `Chart.yaml` declares.
- `internal/scan/` keeps a shallow clone under the data dir, runs one scan at a time and holds the last result in memory until SQLite lands.
- `internal/api/router.go` lists every route, one line each. Handlers decode, call one thing and respond.
- `internal/ui/` embeds `dist/`. The `.gitkeep` there lets a Go-only build compile; the Vite config writes it back after every build.
- `web/` is the React 19, TypeScript, Vite and Tailwind v4 client. Its `go.mod` exists only to keep `go build ./...` out of `node_modules`.

## Rules

- Every source file starts with the SPDX header and `Copyright (C) <year> FireBall1725`.
- No ORM, no router library, no web framework. SQL lives in `internal/repository/`.
- Return an empty slice, never `nil`, from anything that serializes a list. `null` crashes the client on `.length`.
- User-facing strings in the UI go through i18next (`web/src/i18n/locales/en.json`).
- Edits to the GitOps repo are line-anchored text rewrites, never a YAML round-trip, so a one-line bump stays a one-line diff.
- Commits carry a DCO sign-off (`git commit -s`) and no AI attribution.

## Before opening a PR

```sh
go build ./... && go vet ./... && go test ./... && golangci-lint run
cd web && npx tsc -b && npm run lint && npm test && npm run build
```

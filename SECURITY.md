# Security policy

## Reporting a vulnerability

Report privately through [GitHub Security Advisories](https://github.com/FireBall1725/upstream/security/advisories/new) rather than opening a public issue. Include what you did, what happened, and what you expected.

This is a single-maintainer project. Expect an acknowledgement within a week.

## What this server holds

One credential: `GITHUB_TOKEN`. With it, Upstream reads release notes and pushes branches and opens pull requests on the GitOps repo. Use a fine-grained token scoped to that one repo, with Contents and Pull requests read/write and nothing else. The token is never logged or returned by the API.

## Deployment notes

- There is no login. Anyone who can reach the UI can open a bump PR, so serve it on an internal-only ingress or behind a VPN.
- Bump PRs only change version pins to versions a registry, Helm repo or GitHub release actually published. With auto-merge left off, every one still needs a person to merge it.
- The image runs as uid 65532 and writes only to `/data`.

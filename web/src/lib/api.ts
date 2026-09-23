// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

export interface Status {
  version: string
  repo: string
  branch: string
  appGlob: string
  schedule: string
  nextScan?: string
  timeZone: string
  hasToken: boolean
  gitAuthor?: string
  prsReady: boolean
  problems: string[]
}

export interface Skip {
  appDir: string
  field: string
  version: string
  createdAt?: string
}

export type PRState = 'queued' | 'running' | 'open' | 'merged' | 'closed' | 'failed'

export interface BumpItem {
  appDir: string
  app: string
  field: string
  kind: string
  source: string
  from: string
  to: string
  bump: string
}

export interface PullRequest {
  id: number
  createdAt: string
  updatedAt: string
  title: string
  branch: string
  state: PRState
  autoMerge: boolean
  number?: number
  url?: string
  error?: string
  items: BumpItem[]
}

export const prBusy = (p: PullRequest) => p.state === 'queued' || p.state === 'running'

// send posts or deletes JSON and returns the parsed reply, or nothing for a 204.
export async function send<T>(method: 'POST' | 'DELETE', path: string, body: unknown): Promise<T | undefined> {
  const res = await fetch(path, {
    method,
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`.trim()
    try {
      const b = (await res.json()) as { error?: unknown }
      if (typeof b.error === 'string' && b.error) message = b.error
    } catch {
      // Not our JSON; keep the status line.
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined
  return (await res.json()) as T
}

export interface Pin {
  kind: 'image' | 'chart'
  name: string
  image?: string
  chartRepo?: string
  version: string
  declared?: string
  appVersion?: string
  file: string
  line: number
  field: string
  latest?: string
  latestAppVersion?: string
  update?: 'major' | 'minor' | 'patch' | 'rebuild' | 'current' | 'unchecked' | 'error'
  note?: string
}

export type Severity = 'fix' | 'tidy' | 'note' | 'info'

export interface Finding {
  check: string
  severity: Severity
  message: string
  file?: string
  line?: number
}

export interface App {
  namespace: string
  name: string
  dir: string
  pins: Pin[]
  findings: Finding[]
}

export interface ScanSummary {
  startedAt: string
  finishedAt: string
  state: 'ok' | 'failed'
  error?: string
  commit?: string
  apps: number
  checked: number
  updates: number
  errors: number
}

export interface ScanResult {
  state: 'never' | 'running' | 'ok' | 'failed'
  startedAt?: string
  finishedAt?: string
  error?: string
  commit?: string
  commitDate?: string
  apps: App[]
  history: ScanSummary[]
}

// getJSON throws with the server's error message, or its status line when the body isn't ours.
export async function getJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { Accept: 'application/json', ...init?.headers } })
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`.trim()
    try {
      const body = (await res.json()) as { error?: unknown }
      if (typeof body.error === 'string' && body.error) message = body.error
    } catch {
      // Not JSON, e.g. an ingress error page; the status line is the best we have.
    }
    throw new Error(message)
  }
  return (await res.json()) as T
}

// repoLabel turns a git URL into the owner/name shown in the header.
export function repoLabel(url: string): string {
  return url
    .replace(/^[a-z]+:\/\/[^/]+\//, '')
    .replace(/^git@[^:]+:/, '')
    .replace(/\.git$/, '')
}

// formatTime renders an ISO time as YYYY-MM-DD HH:MM on the 24-hour clock in the viewer's zone.
export function formatTime(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// findingFor returns the finding that points at the same line as a pin, if any.
export function findingFor(app: App, pin: Pin): Finding | undefined {
  return app.findings.find((f) => f.file === pin.file && f.line === pin.line)
}

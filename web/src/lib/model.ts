// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import type { App, Finding, Pin, Severity } from './api'

// Bump is the badge an app row shows, worst first.
export type Bump = 'major' | 'minor' | 'patch' | 'rebuild' | 'current' | 'unchecked'
export const BUMPS: Bump[] = ['major', 'minor', 'patch', 'rebuild', 'current', 'unchecked']
export const UPDATABLE: Bump[] = ['major', 'minor', 'patch', 'rebuild']

export interface Row {
  app: App
  key: string
  bump: Bump
  // pin is the one the row shows: the biggest update, or the first pin when there is none.
  pin?: Pin
}

export const rank = (b: Bump) => BUMPS.indexOf(b)

function pinBump(p: Pin): Bump {
  switch (p.update) {
    case 'major':
    case 'minor':
    case 'patch':
    case 'rebuild':
    case 'current':
      return p.update
    default:
      return 'unchecked'
  }
}

export function toRow(app: App): Row {
  let best: Pin | undefined
  let bump: Bump = 'unchecked'
  for (const p of app.pins) {
    const b = pinBump(p)
    if (!best || rank(b) < rank(bump)) {
      best = p
      bump = b
    }
  }
  return { app, key: app.dir, bump, pin: best }
}

export const canBump = (r: Row) => UPDATABLE.includes(r.bump)

// updatesOf lists the pins a bump PR for this app would change.
export const updatesOf = (app: App) => app.pins.filter((p) => UPDATABLE.includes(pinBump(p)))

// channelOf reads the release channel from a tag, for the drawer.
export function channelOf(version: string): string {
  const v = version.toLowerCase()
  if (/nightly|dev/.test(v)) return 'nightly'
  if (/-rc|rc\d|\.rc/.test(v)) return 'rc'
  if (/beta|\db\d/.test(v)) return 'beta'
  if (/alpha|\da\d/.test(v)) return 'alpha'
  return 'stable'
}

// filesFor lists what a bump touches, matching the repo's own three-file bumps.
export function filesFor(p: Pin): string[] {
  const dir = p.file.slice(0, p.file.lastIndexOf('/') + 1)
  if (p.kind === 'chart') return [`${dir}Chart.yaml`, `${dir}Chart.lock`, `${dir}charts/${p.name}-${(p.latest ?? '').replace(/^v/, '')}.tgz`]
  if (p.field === 'appVersion') return [`${dir}Chart.yaml appVersion`, `${dir}README.md`]
  if (p.field.endsWith('image.tag')) return [`${dir}values.yaml`, `${dir}Chart.yaml appVersion`, `${dir}README.md`]
  return [p.file]
}

export const joinAnd = (xs: string[]) => (xs.length < 2 ? xs.join('') : `${xs.slice(0, -1).join(', ')} and ${xs[xs.length - 1]}`)

// titleFor writes a PR title in the repo's style: "app: old -> new" for one, a summary for more.
export function titleFor(rows: Row[]): string {
  if (rows.length === 1) {
    const { app, pin } = rows[0]
    return `${app.name}: ${pin?.version} -> ${pin?.latest}`
  }
  const names = rows.map((r) => r.app.name).sort()
  const namespaces = [...new Set(rows.map((r) => r.app.namespace))]
  if (names.length > 3 && namespaces.length === 1) return `${namespaces[0]}: bump ${names.length} apps`
  if (names.length <= 5) return `Bump ${joinAnd(names)}`
  return `Bump ${names.length} apps: ${names.slice(0, 3).join(', ')} and ${names.length - 3} more`
}

// sourceLabel is the short form shown in the table's Source column.
export function sourceLabel(p: Pin): string {
  if (p.kind === 'chart') return `${p.name} @ ${(p.chartRepo ?? '').replace(/^[a-z]+:\/\//, '').replace(/\/$/, '')}`
  return p.image ?? ''
}

const order: Severity[] = ['fix', 'tidy', 'note', 'info']

// hygieneGroups groups findings by check, the most severe and most common first.
export function hygieneGroups(apps: App[]) {
  const groups = new Map<string, { app: App; finding: Finding }[]>()
  for (const app of apps) {
    for (const finding of app.findings) {
      groups.set(finding.check, [...(groups.get(finding.check) ?? []), { app, finding }])
    }
  }
  return [...groups.entries()].sort(
    ([, a], [, b]) => order.indexOf(a[0].finding.severity) - order.indexOf(b[0].finding.severity) || b.length - a.length,
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { describe, expect, it } from 'vitest'
import type { App, Pin } from './api'
import { channelOf, filesFor, titleFor, toRow } from './model'

const pin = (over: Partial<Pin>): Pin => ({
  kind: 'image',
  name: 'image',
  image: 'example/app',
  version: '1.0.0',
  file: 'apps/ns/app/values.yaml',
  line: 3,
  field: 'image.tag',
  ...over,
})
const app = (name: string, pins: Pin[], namespace = 'app-media'): App => ({ name, namespace, dir: `apps/${namespace}/${name}`, pins, findings: [] })

describe('toRow', () => {
  it('shows the biggest update', () => {
    const r = toRow(app('wikijs', [pin({ update: 'unchecked' }), pin({ kind: 'chart', name: 'wiki', update: 'major', latest: '3.0.0' }), pin({ update: 'patch' })]))
    expect(r.bump).toBe('major')
    expect(r.pin?.name).toBe('wiki')
  })
  it('is current when nothing is newer and something was checked', () => {
    expect(toRow(app('a', [pin({ update: 'unchecked' }), pin({ update: 'current' })])).bump).toBe('current')
  })
  it("can't check an app with nothing pinned or only errors", () => {
    expect(toRow(app('ollama', [])).bump).toBe('unchecked')
    expect(toRow(app('x', [pin({ update: 'error' })])).bump).toBe('unchecked')
  })
})

describe('channelOf', () => {
  it.each([
    ['26.9.1-nightly.202609212159', 'nightly'],
    ['2.3.6-nightly', 'nightly'],
    ['26.8.0-rc.11', 'rc'],
    ['2.9.0b3', 'beta'],
    ['4.0.17.2952-ls306', 'stable'],
    ['v0.35.0', 'stable'],
  ])('%s', (v, want) => expect(channelOf(v)).toBe(want))
})

describe('filesFor', () => {
  it('chart bumps regenerate the lock and vendored chart', () => {
    expect(filesFor(pin({ kind: 'chart', name: 'traefik', latest: '41.6.0', file: 'apps/c/traefik/Chart.yaml', field: 'dependencies[traefik].version' }))).toEqual([
      'apps/c/traefik/Chart.yaml',
      'apps/c/traefik/Chart.lock',
      'apps/c/traefik/charts/traefik-41.6.0.tgz',
    ])
  })
  it('image tag bumps keep appVersion in step', () => {
    expect(filesFor(pin({}))).toEqual(['apps/ns/app/values.yaml', 'apps/ns/app/Chart.yaml appVersion', 'apps/ns/app/README.md'])
  })
})

describe('titleFor', () => {
  const row = (name: string, ns = 'app-media') => toRow(app(name, [pin({ update: 'patch', version: '1.0.0', latest: '1.0.1' })], ns))
  it('one app reads like the repo history', () => expect(titleFor([row('sonarr')])).toBe('sonarr: 1.0.0 -> 1.0.1'))
  it('a namespace batch', () => expect(titleFor(['a', 'b', 'c', 'd'].map((n) => row(n)))).toBe('app-media: bump 4 apps'))
  it('a small mixed batch', () => expect(titleFor([row('b', 'x'), row('a', 'y')])).toBe('Bump a and b'))
})

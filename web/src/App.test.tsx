// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import './i18n'
import App from './App'
import type { ScanResult, Status } from './lib/api'

const status: Status = {
  version: '0.0.0-dev',
  repo: 'https://github.com/FireBall1725/homelab-applications.git',
  branch: 'main',
  appGlob: 'apps/*/*',
  schedule: '0 */6 * * *',
  problems: [],
}

const scanned: ScanResult = {
  state: 'ok',
  finishedAt: '2026-09-23T00:14:00-04:00',
  commit: 'f64f9b5aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  apps: [
    {
      namespace: 'app-ttyd',
      name: 'ttyd',
      dir: 'apps/app-ttyd/ttyd',
      pins: [
        { kind: 'image', name: 'image', image: 'tsl0922/ttyd', version: 'latest', file: 'apps/app-ttyd/ttyd/values.yaml', line: 4, field: 'image.tag' },
      ],
      findings: [
        { check: 'floating-tag', severity: 'fix', message: 'tsl0922/ttyd is pinned to "latest", which moves on its own.', file: 'apps/app-ttyd/ttyd/values.yaml', line: 4 },
      ],
    },
  ],
}

type Routes = Record<string, (init?: RequestInit) => Response>

function mockFetch(routes: Routes) {
  return vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const key = `${init?.method ?? 'GET'} ${String(input)}`
    const route = routes[key]
    if (!route) throw new Error(`unexpected ${key}`)
    return route(init)
  })
}

const json = (body: unknown, code = 200) => () => new Response(JSON.stringify(body), { status: code })

describe('App', () => {
  afterEach(() => vi.restoreAllMocks())

  it('lists setup problems from the server', async () => {
    mockFetch({
      'GET /api/status': json({ ...status, repo: '', problems: ['UPSTREAM_REPO is not set, so there is nothing to scan'] }),
      'GET /api/scan': json({ state: 'never', apps: [] }),
    })
    render(<App />)
    expect(await screen.findByText(/UPSTREAM_REPO is not set/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Scan now' })).toBeDisabled()
  })

  it('says so when the server is unreachable', async () => {
    mockFetch({
      'GET /api/status': () => new Response('', { status: 502, statusText: 'Bad Gateway' }),
      'GET /api/scan': json({ state: 'never', apps: [] }),
    })
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('502 Bad Gateway')
  })

  it('shows pinned versions and hygiene from the last scan', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(scanned) })
    render(<App />)
    expect(await screen.findByText('ttyd')).toBeInTheDocument()
    expect(screen.getByText('Floating tag')).toBeInTheDocument()
    expect(screen.getByText('Last scan', { exact: false })).toHaveTextContent('2026-09-23')
    expect(screen.getByText(/main @ f64f9b5/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: /Repo hygiene/ }))
    expect(screen.getByText(/moves on its own/)).toBeInTheDocument()
  })

  it('starts a scan and shows it running', async () => {
    const spy = mockFetch({
      'GET /api/status': json(status),
      'GET /api/scan': json({ state: 'never', apps: [] }),
      'POST /api/scan': json({ state: 'running', apps: [] }, 202),
    })
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Scan now' }))
    expect(await screen.findByRole('button', { name: 'Scanning…' })).toBeDisabled()
    expect(spy).toHaveBeenCalledWith('/api/scan', expect.objectContaining({ method: 'POST' }))
  })

  it('shows the server error when a scan cannot start', async () => {
    mockFetch({
      'GET /api/status': json(status),
      'GET /api/scan': json({ state: 'never', apps: [] }),
      'POST /api/scan': json({ error: 'UPSTREAM_REPO is not set, so there is nothing to scan' }, 409),
    })
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Scan now' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('UPSTREAM_REPO is not set')
  })
})

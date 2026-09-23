// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import './i18n'
import App from './App'
import type { App as AppT, Pin, ScanResult, Status } from './lib/api'

const status: Status = {
  version: '0.0.0-dev',
  repo: 'https://github.com/FireBall1725/homelab-applications.git',
  branch: 'main',
  appGlob: 'apps/*/*',
  schedule: '0 */6 * * *',
  nextScan: '2026-09-23T06:00:00-04:00',
  timeZone: 'America/Toronto',
  hasToken: true,
  gitAuthor: 'FireBall1725',
  problems: [],
}

const pin = (dir: string, over: Partial<Pin>): Pin => ({
  kind: 'image',
  name: 'image',
  image: 'example/x',
  version: '1.0.0',
  file: `${dir}/values.yaml`,
  line: 4,
  field: 'image.tag',
  update: 'current',
  latest: '1.0.0',
  ...over,
})
const app = (name: string, ns: string, pins: (dir: string) => Pin[], findings: AppT['findings'] = []): AppT => {
  const dir = `apps/${ns}/${name}`
  return { name, namespace: ns, dir, pins: pins(dir), findings }
}

const scanned: ScanResult = {
  state: 'ok',
  commit: 'f64f9b5aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  apps: [
    app('sonarr', 'app-media', (d) => [pin(d, { image: 'lscr.io/linuxserver/sonarr', version: '4.0.17', latest: '4.0.20', update: 'patch' })]),
    app('sabnzbd', 'app-media', (d) => [pin(d, { image: 'lscr.io/linuxserver/sabnzbd', version: '4.5.5', latest: '5.1.3', update: 'major' })]),
    app('radarr', 'app-media', (d) => [pin(d, { image: 'lscr.io/linuxserver/radarr', version: '6.0.4', latest: '6.4.4', update: 'minor' })]),
    app('blocky', 'app-blocky', (d) => [pin(d, { image: 'ghcr.io/0xerr0r/blocky', version: 'v0.35.0', latest: 'v0.35.0' })]),
    app('ttyd', 'app-ttyd', (d) => [pin(d, { image: 'tsl0922/ttyd', version: 'latest', latest: undefined, update: 'unchecked', note: 'Floating tag.' })], [
      { check: 'floating-tag', severity: 'fix', message: 'tsl0922/ttyd is pinned to "latest".', file: 'apps/app-ttyd/ttyd/values.yaml', line: 4 },
    ]),
  ],
  history: [
    { startedAt: '2026-09-23T00:14:00-04:00', finishedAt: '2026-09-23T00:14:04-04:00', state: 'ok', commit: 'f64f9b5aaaaaaaaaaaa', apps: 5, checked: 4, updates: 3, errors: 0 },
  ],
}

function mockFetch(routes: Record<string, () => Response>) {
  return vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const key = `${init?.method ?? 'GET'} ${String(input)}`
    const route = routes[key]
    if (!route) throw new Error(`unexpected ${key}`)
    return route()
  })
}
const json = (body: unknown, code = 200) => () => new Response(JSON.stringify(body), { status: code })
const never = { state: 'never', apps: [], history: [] }

describe('App', () => {
  afterEach(() => vi.restoreAllMocks())

  it('lists setup problems and disables scanning without a repo', async () => {
    mockFetch({
      'GET /api/status': json({ ...status, repo: '', problems: ['UPSTREAM_REPO is not set, so there is nothing to scan'] }),
      'GET /api/scan': json(never),
    })
    render(<App />)
    expect(await screen.findByText(/UPSTREAM_REPO is not set/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Scan now' })).toBeDisabled()
  })

  it('says so when the server is unreachable', async () => {
    mockFetch({ 'GET /api/status': () => new Response('', { status: 502, statusText: 'Bad Gateway' }), 'GET /api/scan': json(never) })
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('502 Bad Gateway')
  })

  it('shows updates worst first and filters with the chips', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(scanned) })
    render(<App />)
    await screen.findByText('sabnzbd')
    expect(screen.getByText('updates available').parentElement).toHaveTextContent('3')
    const names = screen.getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].querySelector('b')?.textContent)
    expect(names).toEqual(['sabnzbd', 'radarr', 'sonarr'])
    expect(screen.queryByText('blocky')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^All/ }))
    expect(screen.getByText('blocky')).toBeInTheDocument()
    expect(screen.getByText('ttyd')).toBeInTheDocument()
  })

  it('opens an app in the side panel', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(scanned) })
    render(<App />)
    fireEvent.click(await screen.findByText('radarr'))
    const panel = screen.getByRole('complementary', { name: 'App detail' })
    expect(within(panel).getByText('values.yaml:4')).toBeInTheDocument()
    expect(within(panel).getByText('6.4.4')).toBeInTheDocument()
    fireEvent.click(within(panel).getByRole('button', { name: 'Add to PR' }))
    expect(screen.getByRole('complementary', { name: 'New pull request' })).toBeInTheDocument()
  })

  it('batches picked apps into a PR preview and splits out the major', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(scanned) })
    render(<App />)
    await screen.findByText('sabnzbd')
    fireEvent.click(screen.getByRole('button', { name: 'all of app-media' }))
    const panel = screen.getByRole('complementary')
    expect(within(panel).getByRole('heading', { name: '2 pull requests' })).toBeInTheDocument()
    expect(within(panel).getByLabelText('Title')).toHaveValue('Bump radarr and sonarr')
    expect(within(panel).getByText(/sabnzbd on its own/)).toBeInTheDocument()
    expect(within(panel).getByRole('checkbox', { name: /Auto-merge/ })).not.toBeChecked()
    expect(within(panel).getByRole('button', { name: /Open 2 PRs/ })).toBeDisabled()

    fireEvent.click(within(panel).getByRole('checkbox', { name: /Give each major its own PR/ }))
    expect(within(panel).getByRole('heading', { name: 'New pull request' })).toBeInTheDocument()
    expect(within(panel).getByLabelText('Title')).toHaveValue('Bump radarr, sabnzbd and sonarr')
  })

  it('groups hygiene findings and lists scan history', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(scanned) })
    render(<App />)
    fireEvent.click(await screen.findByRole('tab', { name: /Repo hygiene/ }))
    expect(screen.getByText('Floating tags')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: /Scans and settings/ }))
    expect(screen.getByText('0 */6 * * *', { selector: '.cron' })).toBeInTheDocument()
    expect(screen.getByText('f64f9b5')).toBeInTheDocument()
  })

  it('starts a scan', async () => {
    const spy = mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(never), 'POST /api/scan': json({ ...never, state: 'running' }, 202) })
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Scan now' }))
    expect(await screen.findByRole('button', { name: 'Scanning…' })).toBeDisabled()
    expect(spy).toHaveBeenCalledWith('/api/scan', expect.objectContaining({ method: 'POST' }))
  })

  it('shows the server error when a scan cannot start', async () => {
    mockFetch({ 'GET /api/status': json(status), 'GET /api/scan': json(never), 'POST /api/scan': json({ error: 'UPSTREAM_REPO is not set' }, 409) })
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Scan now' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('UPSTREAM_REPO is not set')
  })
})

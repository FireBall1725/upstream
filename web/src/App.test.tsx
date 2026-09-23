// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import './i18n'
import App from './App'

describe('App', () => {
  afterEach(() => vi.restoreAllMocks())

  it('lists setup problems from the server', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          version: '0.0.0-dev',
          repo: '',
          branch: 'main',
          appGlob: 'apps/*/*',
          schedule: '0 */6 * * *',
          problems: ['UPSTREAM_REPO is not set, so there is nothing to scan'],
        }),
      ),
    )
    render(<App />)
    expect(await screen.findByText(/UPSTREAM_REPO is not set/)).toBeInTheDocument()
  })

  it('says so when the server is unreachable', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('', { status: 502, statusText: 'Bad Gateway' }))
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('502 Bad Gateway')
  })
})

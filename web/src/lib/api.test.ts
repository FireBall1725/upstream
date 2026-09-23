// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { describe, expect, it } from 'vitest'
import { formatTime, repoLabel } from './api'

describe('repoLabel', () => {
  it.each([
    ['https://github.com/FireBall1725/homelab-applications.git', 'FireBall1725/homelab-applications'],
    ['https://github.com/FireBall1725/homelab-applications', 'FireBall1725/homelab-applications'],
    ['git@github.com:FireBall1725/homelab-applications.git', 'FireBall1725/homelab-applications'],
    ['https://gitea.local/team/gitops.git', 'team/gitops'],
  ])('%s', (url, want) => {
    expect(repoLabel(url)).toBe(want)
  })
})

describe('formatTime', () => {
  it('uses the 24-hour clock', () => {
    const d = new Date(2026, 8, 23, 17, 5)
    expect(formatTime(d.toISOString())).toBe('2026-09-23 17:05')
  })
  it('is empty for missing or bad input', () => {
    expect(formatTime(undefined)).toBe('')
    expect(formatTime('nope')).toBe('')
  })
})

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { describe, expect, it } from 'vitest'
import { repoLabel } from './api'

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

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import type { Severity } from '../lib/api'

const tone: Record<Severity, string> = {
  fix: 'bg-major-bg text-major',
  tidy: 'bg-minor-bg text-minor',
  note: 'bg-rebuild-bg text-rebuild',
  info: 'bg-current-bg text-current',
}

export default function Pill({ severity, children }: { severity: Severity; children: React.ReactNode }) {
  return (
    <span className={`inline-block rounded px-2 py-0.5 text-[11px] font-semibold tracking-[.03em] whitespace-nowrap ${tone[severity]}`}>
      {children}
    </span>
  )
}

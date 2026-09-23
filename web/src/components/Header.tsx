// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { repoLabel, type Status } from '../lib/api'

export default function Header({ status }: { status: Status | null }) {
  const { t } = useTranslation()
  return (
    <header className="flex flex-wrap items-center gap-x-6 gap-y-3 py-4">
      <div className="flex items-baseline gap-2.5">
        <span aria-hidden="true" className="relative top-px inline-block size-3.5 rounded-[3px] border-2 border-accent" />
        <h1 className="font-cond text-[22px] leading-none font-semibold tracking-[.01em]">{t('app.name')}</h1>
      </div>
      {status?.repo && (
        <div className="flex flex-wrap gap-2 font-mono text-xs text-muted">
          <span className="rounded bg-sunk px-1.5 py-0.5">{repoLabel(status.repo)}</span>
          <span className="rounded bg-sunk px-1.5 py-0.5">{status.branch}</span>
          <span className="rounded bg-sunk px-1.5 py-0.5">{status.appGlob}</span>
        </div>
      )}
      <div className="flex-1" />
      <div className="text-right text-xs text-muted">
        {status && <div>{t('status.schedule', { schedule: status.schedule })}</div>}
        <div className="font-mono">{t('app.version', { version: status?.version ?? __APP_VERSION__ })}</div>
      </div>
    </header>
  )
}

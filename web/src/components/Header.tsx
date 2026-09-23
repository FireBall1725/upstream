// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { formatTime, repoLabel, type ScanResult, type Status } from '../lib/api'

interface Props {
  status: Status | null
  scan: ScanResult | null
  onScan: () => void
}

export default function Header({ status, scan, onScan }: Props) {
  const { t } = useTranslation()
  const running = scan?.state === 'running'
  const last = scan?.state === 'ok' || scan?.state === 'failed' ? formatTime(scan.finishedAt) : ''

  return (
    <header className="flex flex-wrap items-center gap-x-6 gap-y-3 py-4">
      <div className="flex items-baseline gap-2.5">
        <span aria-hidden="true" className="relative top-px inline-block size-3.5 rounded-[3px] border-2 border-accent" />
        <h1 className="font-cond text-[22px] leading-none font-semibold tracking-[.01em]">{t('app.name')}</h1>
      </div>
      {status?.repo && (
        <div className="flex flex-wrap gap-2 font-mono text-xs text-muted">
          <span className="rounded bg-sunk px-1.5 py-0.5">{repoLabel(status.repo)}</span>
          <span className="rounded bg-sunk px-1.5 py-0.5">
            {status.branch}
            {scan?.commit && ` @ ${scan.commit.slice(0, 7)}`}
          </span>
          <span className="rounded bg-sunk px-1.5 py-0.5">{status.appGlob}</span>
        </div>
      )}
      <div className="flex-1" />
      <div className="text-right text-xs text-muted">
        <div>{last ? t('scan.last', { time: last }) : t('scan.none')}</div>
        {status && <div className="font-mono">{t('status.schedule', { schedule: status.schedule })}</div>}
      </div>
      <button
        type="button"
        onClick={onScan}
        disabled={running || !status?.repo}
        className="rounded-md border border-line bg-surface px-3.5 py-2 text-[13px] font-semibold hover:bg-sunk focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-not-allowed disabled:opacity-60"
      >
        {running ? t('scan.running') : t('scan.now')}
      </button>
    </header>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { formatTime, repoLabel, type ScanResult, type Status } from '../lib/api'

interface Props {
  status: Status | null
  scan: ScanResult | null
  onScan: () => void
}

export default function TopBar({ status, scan, onScan }: Props) {
  const { t } = useTranslation()
  const running = scan?.state === 'running'
  const last = scan?.history[0]?.finishedAt
  return (
    <header className="top">
      <div className="brand">
        <span className="mark" aria-hidden="true" />
        <h1>{t('app.name')}</h1>
      </div>
      {status?.repo && (
        <div className="repo">
          <span>{repoLabel(status.repo)}</span>
          <span>
            {status.branch}
            {scan?.commit ? ` @ ${scan.commit.slice(0, 7)}` : ''}
          </span>
          <span>{status.appGlob}</span>
        </div>
      )}
      <div className="spacer" />
      {status && (
        <div className="scaninfo">
          {last ? (
            <>
              {t('top.last')} <b>{formatTime(last)}</b>
            </>
          ) : (
            t('top.never')
          )}
          {status.nextScan && (
            <>
              {' · '}
              {t('top.next')} <b>{formatTime(status.nextScan)}</b>
            </>
          )}
          <br />
          <span className="path">{status.schedule}</span>
        </div>
      )}
      <button className="btn ghost" type="button" onClick={onScan} disabled={running || !status?.repo}>
        {running ? t('top.scanning') : t('top.scanNow')}
      </button>
    </header>
  )
}

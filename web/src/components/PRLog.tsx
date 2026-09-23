// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { formatTime, type PRState, type PullRequest } from '../lib/api'

const tone: Record<PRState, string> = {
  queued: 'p-unchecked',
  running: 'p-unchecked',
  open: 'p-patch',
  merged: 'p-rebuild',
  closed: 'p-current',
  failed: 'p-major',
}

export function PRStatePill({ state }: { state: PRState }) {
  const { t } = useTranslation()
  return <span className={`pill ${tone[state]}`}>{t(`prState.${state}`)}</span>
}

export default function PRLog({ prs, empty }: { prs: PullRequest[]; empty?: string }) {
  const { t } = useTranslation()
  if (prs.length === 0) return <p className="muted" style={{ margin: 0 }}>{empty ?? t('prlog.none')}</p>
  return (
    <ul className="edits">
      {prs.map((p) => (
        <li key={p.id}>
          <div>
            <b>{p.title}</b>
            <span className="ns">
              {formatTime(p.createdAt)} · {t('prlog.bumps', { count: p.items.length })}
              {p.autoMerge ? ` · ${t('prlog.autoMerge')}` : ''}
            </span>
          </div>
          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            <PRStatePill state={p.state} />
            {p.url && (
              <a className="linkbtn" href={p.url} target="_blank" rel="noreferrer">
                #{p.number}
              </a>
            )}
          </div>
          {p.error && <div className="callout warn" style={{ gridColumn: '1 / -1' }}>{p.error}</div>}
        </li>
      ))}
    </ul>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { filesFor, joinAnd, rank, titleFor, updatesOf, type Row } from '../lib/model'
import BumpPill from './BumpPill'

export interface PRGroup {
  title: string
  apps: string[]
}

interface Props {
  rows: Row[]
  splitMajors: boolean
  ready: boolean
  onSplitMajors: (on: boolean) => void
  onRemove: (key: string) => void
  onClear: () => void
  onOpen: (groups: PRGroup[], autoMerge: boolean) => Promise<void>
}

function today(): string {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

export default function PRPreview({ rows, splitMajors, ready, onSplitMajors, onRemove, onClear, onOpen }: Props) {
  const { t } = useTranslation()
  const [autoMerge, setAutoMerge] = useState(false)
  const [title, setTitle] = useState<string | null>(null)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const list = [...rows].sort((a, b) => rank(a.bump) - rank(b.bump) || a.app.name.localeCompare(b.app.name))
  const majors = list.filter((r) => r.bump === 'major')
  const rest = list.filter((r) => r.bump !== 'major')
  const groups = splitMajors && majors.length > 0 && rest.length > 0 ? [rest, ...majors.map((m) => [m])] : [list]
  const namespaces = new Set(list.map((r) => r.app.namespace)).size
  const hasChart = list.some((r) => updatesOf(r.app).some((p) => p.kind === 'chart'))

  return (
    <>
      <div className="dh">
        <div>
          <h2>{groups.length === 1 ? t('pr.one') : t('pr.many', { count: groups.length })}</h2>
          <span className="ns">{t('pr.scope', { apps: list.length, namespaces })}</span>
        </div>
      </div>
      <div className="dsec">
        <h3>{t('pr.title')}</h3>
        <input
          className="prtitle"
          id="prTitle"
          value={title ?? titleFor(groups[0])}
          onChange={(e) => setTitle(e.target.value)}
          aria-label={t('pr.title')}
        />
        <p className="muted" style={{ margin: '8px 0 0', fontSize: 12 }}>
          {t('pr.branch')} <span className="path">{`upstream/${today()}-${list.length}-apps`}</span> {t('pr.into')} <span className="path">main</span>.{' '}
          {t('pr.body')}
        </p>
      </div>
      {majors.length > 0 && (
        <div className="dsec">
          <label className="check">
            <input type="checkbox" id="splitMajors" checked={splitMajors} onChange={(e) => onSplitMajors(e.target.checked)} />
            <span>
              <b>{t('pr.splitTitle')}</b>
              <br />
              <span className="muted">{t('pr.splitWhy', { count: majors.length, names: joinAnd(majors.map((m) => m.app.name)) })}</span>
            </span>
          </label>
        </div>
      )}
      <div className="dsec">
        <h3>{t('pr.changes')}</h3>
        {groups.map((g, i) => (
          <div key={g.map((r) => r.key).join()}>
            {groups.length > 1 && (
              <div className="prgroup">{t('pr.group', { n: i + 1, title: i === 0 ? titleFor(g) : t('pr.alone', { name: g[0].app.name }) })}</div>
            )}
            <ul className="edits">
              {g.map((r) =>
                updatesOf(r.app).map((p, j) => (
                  <li key={`${r.key}:${p.file}:${p.line}`}>
                    <div>
                      <b>{r.app.name}</b> <span className="v old">{p.version}</span> <span className="arrow">→</span> <span className="v">{p.latest}</span>
                    </div>
                    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                      <BumpPill bump={(p.update ?? 'unchecked') as Row['bump']} />
                      {j === 0 && (
                        <button className="rm" type="button" aria-label={t('pr.remove', { name: r.app.name })} onClick={() => onRemove(r.key)}>
                          ×
                        </button>
                      )}
                    </div>
                    <div className="files">
                      {filesFor(p).map((f) => (
                        <span key={f}>{f.slice(r.app.dir.length + 1)}</span>
                      ))}
                    </div>
                  </li>
                )),
              )}
            </ul>
          </div>
        ))}
      </div>
      {hasChart && (
        <div className="dsec">
          <div className="callout">{t('pr.helm')}</div>
        </div>
      )}
      <div className="dsec">
        <label className="check" style={{ marginBottom: 12 }}>
          <input type="checkbox" id="autoMerge" checked={autoMerge} onChange={(e) => setAutoMerge(e.target.checked)} />
          <span>
            <b>{t('pr.autoMerge')}</b>
            <br />
            <span className="muted">{t('pr.autoMergeWhy')}</span>
          </span>
        </label>
        <div className="actions">
          <button
            className="btn"
            type="button"
            disabled={!ready || sending}
            onClick={() => {
              setSending(true)
              setError(null)
              const payload = groups.map((g, i) => ({ title: i === 0 ? (title ?? titleFor(g)).trim() || titleFor(g) : titleFor(g), apps: g.map((r) => r.app.dir) }))
              onOpen(payload, autoMerge)
                .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)))
                .finally(() => setSending(false))
            }}
          >
            {sending ? t('pr.sending') : groups.length === 1 ? t('pr.open') : t('pr.openMany', { count: groups.length })}
          </button>
          <button className="btn ghost" type="button" onClick={onClear} disabled={sending}>
            {t('pr.clear')}
          </button>
        </div>
        {!ready && (
          <p className="muted" style={{ margin: '10px 0 0', fontSize: 12 }}>
            {t('pr.notReady')}
          </p>
        )}
        {error && (
          <p role="alert" className="callout warn" style={{ margin: '10px 0 0' }}>
            {error}
          </p>
        )}
      </div>
    </>
  )
}

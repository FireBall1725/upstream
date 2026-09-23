// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import AppsTable from './components/AppsTable'
import Header from './components/Header'
import Hygiene from './components/Hygiene'
import { useScan } from './hooks/useScan'
import { getJSON, type Status } from './lib/api'

type Tab = 'apps' | 'hygiene'

export default function App() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<Status | null>(null)
  const [statusError, setStatusError] = useState<string | null>(null)
  const { scan, error: scanError, start } = useScan()
  const [tab, setTab] = useState<Tab>('apps')
  const [query, setQuery] = useState('')

  useEffect(() => {
    const ctrl = new AbortController()
    getJSON<Status>('/api/status', { signal: ctrl.signal })
      .then(setStatus)
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) setStatusError(e instanceof Error ? e.message : String(e))
      })
    return () => ctrl.abort()
  }, [])

  const apps = useMemo(() => scan?.apps ?? [], [scan])
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return apps
    return apps.filter((a) =>
      [a.name, a.namespace, ...a.pins.map((p) => p.image ?? p.name)].some((s) => s.toLowerCase().includes(q)),
    )
  }, [apps, query])
  const pinCount = apps.reduce((n, a) => n + a.pins.length, 0)
  const findingCount = apps.reduce((n, a) => n + a.findings.length, 0)
  const fixCount = apps.reduce((n, a) => n + a.findings.filter((f) => f.severity === 'fix').length, 0)

  const tabClass = (active: boolean) =>
    `-mb-px border-b-2 px-3.5 py-2.5 font-medium ${active ? 'border-accent text-ink' : 'border-transparent text-muted'}`

  return (
    <div className="mx-auto max-w-[1680px] px-4 pb-8">
      <Header status={status} scan={scan} onScan={() => void start()} />
      <main className="grid gap-4">
        {(statusError || scanError) && (
          <p role="alert" className="rounded-lg bg-major-bg px-4 py-3 text-major">
            {t('status.unreachable', { error: statusError ?? scanError })}
          </p>
        )}
        {status && status.problems.length > 0 && (
          <section className="rounded-lg bg-minor-bg px-4 py-3">
            <h2 className="font-cond text-base font-semibold">{t('status.setupTitle')}</h2>
            <ul className="mt-1 list-disc pl-5 text-sm">
              {status.problems.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          </section>
        )}
        {scan?.state === 'failed' && (
          <p role="alert" className="rounded-lg bg-major-bg px-4 py-3 text-major">
            {t('scan.failed', { error: scan.error })}
          </p>
        )}
        {!status && !statusError && <p className="text-muted">{t('status.loading')}</p>}

        {apps.length > 0 && (
          <>
            <section className="flex flex-wrap items-baseline gap-x-8 gap-y-2 rounded-[10px] border border-line bg-surface px-5 py-4">
              <div className="font-cond text-[44px] leading-none font-semibold tabular-nums">
                {apps.length}
                <small className="mt-1.5 block font-sans text-xs font-medium tracking-[.06em] text-muted uppercase">
                  {t('summary.apps')}
                </small>
              </div>
              <p className="text-muted">{t('summary.detail', { pins: pinCount, findings: findingCount, fix: fixCount })}</p>
              <p className="basis-full text-xs text-muted">{t('summary.noUpdatesYet')}</p>
            </section>

            <nav className="flex gap-1 border-b border-line" role="tablist">
              <button type="button" role="tab" aria-selected={tab === 'apps'} className={tabClass(tab === 'apps')} onClick={() => setTab('apps')}>
                {t('tabs.apps')}
                <span className="ml-1.5 rounded-full bg-sunk px-1.5 font-mono text-[11px]">{apps.length}</span>
              </button>
              <button type="button" role="tab" aria-selected={tab === 'hygiene'} className={tabClass(tab === 'hygiene')} onClick={() => setTab('hygiene')}>
                {t('tabs.hygiene')}
                <span className="ml-1.5 rounded-full bg-sunk px-1.5 font-mono text-[11px]">{findingCount}</span>
              </button>
            </nav>

            {tab === 'apps' ? (
              <>
                <input
                  id="filter"
                  type="search"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t('apps.filter')}
                  aria-label={t('apps.filter')}
                  className="w-full max-w-xs rounded-md border border-line bg-surface px-2.5 py-1.5 text-[13px]"
                />
                <AppsTable apps={shown} />
              </>
            ) : (
              <Hygiene apps={apps} />
            )}
          </>
        )}
        {status && apps.length === 0 && scan?.state !== 'running' && scan?.state !== 'failed' && (
          <p className="rounded-[10px] border border-line bg-surface px-5 py-4 text-muted">{t('scan.emptyPrompt')}</p>
        )}
        {apps.length === 0 && scan?.state === 'running' && (
          <p className="rounded-[10px] border border-line bg-surface px-5 py-4 text-muted">{t('scan.firstRunning')}</p>
        )}
      </main>
    </div>
  )
}

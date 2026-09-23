// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import AppDetail from './components/AppDetail'
import HygieneTab from './components/HygieneTab'
import PRLog from './components/PRLog'
import PRPreview, { type PRGroup } from './components/PRPreview'
import ScansTab from './components/ScansTab'
import Summary from './components/Summary'
import TopBar from './components/TopBar'
import UpdatesTable from './components/UpdatesTable'
import { usePRs } from './hooks/usePRs'
import { useScan } from './hooks/useScan'
import { getJSON, prBusy, send, type Pin, type PullRequest, type Skip, type Status } from './lib/api'
import { BUMPS, canBump, hygieneGroups, rank, toRow, type Bump, type Row } from './lib/model'

type Tab = 'updates' | 'hygiene' | 'scans'
type Filter = 'updates' | 'all' | Bump
type View = { kind: 'none' } | { kind: 'app'; key: string } | { kind: 'pr' } | { kind: 'sent'; ids: number[] }

export default function App() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<Status | null>(null)
  const [statusError, setStatusError] = useState<string | null>(null)
  const { scan, error: scanError, start, reload: reloadScan } = useScan()
  const [statusTick, setStatusTick] = useState(0)
  const { prs, reload: reloadPRs } = usePRs(() => {
    void reloadScan()
    setStatusTick((n) => n + 1)
  })
  const [skips, setSkips] = useState<Skip[]>([])
  const [actionError, setActionError] = useState<string | null>(null)

  const [tab, setTab] = useState<Tab>('updates')
  const [filter, setFilter] = useState<Filter>('updates')
  const [ns, setNs] = useState('')
  const [query, setQuery] = useState('')
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [view, setView] = useState<View>({ kind: 'none' })
  const [splitMajors, setSplitMajors] = useState(true)

  useEffect(() => {
    const ctrl = new AbortController()
    getJSON<Status>('/api/status', { signal: ctrl.signal })
      .then(setStatus)
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) setStatusError(e instanceof Error ? e.message : String(e))
      })
    return () => ctrl.abort()
  }, [scan?.history.length, statusTick])

  const loadSkips = async () => {
    try {
      setSkips(await getJSON<Skip[]>('/api/skips'))
    } catch {
      // Skips only decorate the side panel; the next change reloads them.
    }
  }
  useEffect(() => {
    void loadSkips()
  }, [])

  const rows = useMemo(() => (scan?.apps ?? []).map(toRow), [scan])
  const byKey = useMemo(() => new Map(rows.map((r) => [r.key, r])), [rows])
  const updates = rows.filter(canBump).length
  const namespaces = useMemo(() => [...new Set(rows.map((r) => r.app.namespace))].sort(), [rows])
  const findings = hygieneGroups(scan?.apps ?? []).length

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return rows
      .filter((r) => {
        if (filter === 'updates' && !canBump(r)) return false
        if (filter !== 'updates' && filter !== 'all' && r.bump !== filter) return false
        if (ns && r.app.namespace !== ns) return false
        if (q && ![r.app.name, ...r.app.pins.map((p) => p.image ?? p.name)].some((s) => s.toLowerCase().includes(q))) return false
        return true
      })
      .sort((a, b) => rank(a.bump) - rank(b.bump) || a.app.name.localeCompare(b.app.name))
  }, [rows, filter, ns, query])

  // Picks survive rescans only for apps that still have an update.
  const pickedRows = [...picked].map((k) => byKey.get(k)).filter((r): r is Row => !!r && canBump(r))

  const pickSet = (keys: string[], on: boolean) => {
    const next = new Set(picked)
    for (const k of keys) {
      if (on) next.add(k)
      else next.delete(k)
    }
    setPicked(next)
    if (on && next.size > 0) setView({ kind: 'pr' })
    else if (next.size === 0 && view.kind === 'pr') setView({ kind: 'none' })
  }
  const pickWhere = (pred: (r: Row) => boolean) => pickSet(rows.filter((r) => canBump(r) && pred(r)).map((r) => r.key), true)
  const clear = () => {
    setPicked(new Set())
    setView({ kind: 'none' })
  }

  const openPR = useMemo(() => {
    const m = new Map<string, { number?: number; url?: string }>()
    for (const p of prs) {
      if (p.state !== 'open' && !prBusy(p)) continue
      for (const it of p.items) if (!m.has(it.appDir)) m.set(it.appDir, { number: p.number, url: p.url })
    }
    return m
  }, [prs])

  const openPRs = async (groups: PRGroup[], autoMerge: boolean) => {
    const made: PullRequest[] = []
    for (const g of groups) {
      const pr = await send<PullRequest>('POST', '/api/prs', { title: g.title, autoMerge, apps: g.apps })
      if (pr) made.push(pr)
    }
    await reloadPRs()
    setPicked(new Set())
    setView({ kind: 'sent', ids: made.map((p) => p.id) })
  }

  const skip = async (appDir: string, pin: Pin) => {
    setActionError(null)
    try {
      await send('POST', '/api/skips', { appDir, field: pin.field, version: pin.latest })
      await loadSkips()
      await reloadScan()
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e))
    }
  }
  const unskip = async (s: Skip) => {
    setActionError(null)
    try {
      await send('DELETE', '/api/skips', s)
      await loadSkips()
      await reloadScan()
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e))
    }
  }

  const drawerRow = view.kind === 'app' ? byKey.get(view.key) : undefined
  const showPR = view.kind === 'pr' && pickedRows.length > 0
  const sent = view.kind === 'sent' ? prs.filter((p) => view.ids.includes(p.id)) : []
  const drawerOpen = !!drawerRow || showPR || view.kind === 'sent'
  const onlyNs = ns || (shown.length > 0 && shown.every((r) => r.app.namespace === shown[0].app.namespace) ? shown[0].app.namespace : '')

  const filters: { k: Filter; label: string; n: number }[] = [
    { k: 'updates', label: t('filter.updates'), n: updates },
    { k: 'all', label: t('filter.all'), n: rows.length },
    ...BUMPS.map((b) => ({ k: b as Filter, label: t(`bump.${b}`), n: rows.filter((r) => r.bump === b).length })),
  ]

  return (
    <div className="wrap">
      <TopBar status={status} scan={scan} onScan={() => void start()} />

      {(statusError || scanError) && (
        <div role="alert" className="banner bad">
          {t('status.unreachable', { error: statusError ?? scanError })}
        </div>
      )}
      {status && status.problems.length > 0 && (
        <section className="banner warn">
          <h2>{t('status.setupTitle')}</h2>
          <ul>
            {status.problems.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        </section>
      )}
      {actionError && (
        <div role="alert" className="banner bad">
          {actionError}
        </div>
      )}
      {scan?.state === 'failed' && (
        <div role="alert" className="banner bad">
          {t('status.scanFailed', { error: scan.error })}
        </div>
      )}
      {!status && !statusError && <p className="muted">{t('status.loading')}</p>}
      {status && rows.length === 0 && <p className="empty">{scan?.state === 'running' ? t('status.firstScan') : t('status.noScans')}</p>}

      {rows.length > 0 && (
        <>
          <Summary rows={rows} updates={updates} />

          <nav className="tabs" role="tablist">
            <button className="tab" role="tab" type="button" aria-selected={tab === 'updates'} onClick={() => setTab('updates')}>
              {t('tabs.updates')}
              <span className="n">{updates}</span>
            </button>
            <button className="tab" role="tab" type="button" aria-selected={tab === 'hygiene'} onClick={() => setTab('hygiene')}>
              {t('tabs.hygiene')}
              <span className="n">{findings}</span>
            </button>
            <button className="tab" role="tab" type="button" aria-selected={tab === 'scans'} onClick={() => setTab('scans')}>
              {t('tabs.scans')}
            </button>
          </nav>

          {tab === 'updates' && (
            <section className={`main ${drawerOpen ? '' : 'nodrawer'}`}>
              <div>
                <div className="tools">
                  {filters.map((f) => (
                    <button key={f.k} className="chip" type="button" aria-pressed={filter === f.k} onClick={() => setFilter(f.k)}>
                      {f.label}
                      <span className="c">{f.n}</span>
                    </button>
                  ))}
                  <select className="search" id="nsSel" aria-label={t('filter.namespace')} value={ns} onChange={(e) => setNs(e.target.value)}>
                    <option value="">{t('filter.allNamespaces')}</option>
                    {namespaces.map((n) => (
                      <option key={n}>{n}</option>
                    ))}
                  </select>
                  <input className="search" id="q" type="search" placeholder={t('filter.search')} aria-label={t('filter.search')} value={query} onChange={(e) => setQuery(e.target.value)} />
                </div>

                <div className="selbar">
                  <span className="quick">
                    {t('select.label')}
                    <button className="linkbtn" type="button" onClick={() => pickWhere((r) => r.bump === 'patch' || r.bump === 'rebuild')}>
                      {t('select.patches')}
                    </button>
                    <button className="linkbtn" type="button" onClick={() => pickWhere((r) => r.bump !== 'major')}>
                      {t('select.minorPatch')}
                    </button>
                    <button className="linkbtn" type="button" onClick={() => pickSet(shown.filter(canBump).map((r) => r.key), true)}>
                      {t('select.shown')}
                    </button>
                    {onlyNs && (
                      <button className="linkbtn" type="button" onClick={() => pickWhere((r) => r.app.namespace === onlyNs)}>
                        {t('select.namespace', { name: onlyNs })}
                      </button>
                    )}
                  </span>
                  <span className="selcount">
                    {pickedRows.length > 0 ? (
                      <>
                        <b>{pickedRows.length}</b> {t('select.inPR')}
                        <button className="btn" type="button" onClick={() => setView({ kind: 'pr' })}>
                          {t('select.review')}
                        </button>
                        <button className="btn ghost" type="button" onClick={clear}>
                          {t('select.clear')}
                        </button>
                      </>
                    ) : (
                      t('select.hint')
                    )}
                  </span>
                </div>

                <UpdatesTable
                  rows={shown}
                  openPR={openPR}
                  picked={picked}
                  highlighted={(r) => (view.kind === 'app' ? view.key === r.key : showPR && picked.has(r.key))}
                  onOpen={(key) => setView({ kind: 'app', key })}
                  onPick={(key, on) => pickSet([key], on)}
                  onPickAll={(on) => pickSet(shown.filter(canBump).map((r) => r.key), on)}
                />
              </div>

              {drawerOpen && (
                <aside className="drawer" aria-label={view.kind === 'sent' ? t('sent.label') : showPR ? t('pr.one') : t('detail.label')}>
                  {view.kind === 'sent' ? (
                    <>
                      <div className="dh">
                        <div>
                          <h2>{t('sent.title', { count: sent.length })}</h2>
                          <span className="ns">{sent.some(prBusy) ? t('sent.working') : t('sent.done')}</span>
                        </div>
                        <button className="close" type="button" aria-label={t('detail.close')} onClick={() => setView({ kind: 'none' })}>
                          ×
                        </button>
                      </div>
                      <div className="dsec">
                        <PRLog prs={sent} />
                      </div>
                    </>
                  ) : showPR ? (
                    <PRPreview
                      rows={pickedRows}
                      splitMajors={splitMajors}
                      ready={!!status?.prsReady}
                      onSplitMajors={setSplitMajors}
                      onRemove={(key) => pickSet([key], false)}
                      onClear={clear}
                      onOpen={openPRs}
                    />
                  ) : (
                    drawerRow && (
                      <AppDetail
                        row={drawerRow}
                        inPR={picked.has(drawerRow.key)}
                        skips={skips}
                        prs={prs}
                        onAdd={() => pickSet([drawerRow.key], true)}
                        onClose={() => setView(pickedRows.length > 0 ? { kind: 'pr' } : { kind: 'none' })}
                        onSkip={(pin) => void skip(drawerRow.app.dir, pin)}
                        onUnskip={(s) => void unskip(s)}
                      />
                    )
                  )}
                </aside>
              )}
            </section>
          )}
          {tab === 'hygiene' && <HygieneTab apps={scan?.apps ?? []} />}
          {tab === 'scans' && <ScansTab status={status} scan={scan} prs={prs} />}
        </>
      )}
    </div>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import Header from './components/Header'
import { getJSON, type Status } from './lib/api'

export default function App() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<Status | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const ctrl = new AbortController()
    getJSON<Status>('/api/status', { signal: ctrl.signal })
      .then(setStatus)
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) setError(e instanceof Error ? e.message : String(e))
      })
    return () => ctrl.abort()
  }, [])

  return (
    <div className="mx-auto max-w-[1680px] px-4 pb-8">
      <Header status={status} />
      <main className="grid gap-4">
        {error && (
          <p role="alert" className="rounded-lg bg-major-bg px-4 py-3 text-major">
            {t('status.unreachable', { error })}
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
        {!status && !error && <p className="text-muted">{t('status.loading')}</p>}
        {status && (
          <section className="rounded-[10px] border border-line bg-surface px-5 py-4 text-muted">
            {t('status.noScans')}
          </section>
        )}
      </main>
    </div>
  )
}

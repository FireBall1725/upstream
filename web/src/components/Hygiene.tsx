// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import type { App, Finding, Severity } from '../lib/api'
import Pill from './Pill'

const order: Severity[] = ['fix', 'tidy', 'note', 'info']

interface Item {
  app: App
  finding: Finding
}

export default function Hygiene({ apps }: { apps: App[] }) {
  const { t } = useTranslation()
  const groups = new Map<string, Item[]>()
  for (const app of apps) {
    for (const finding of app.findings) {
      groups.set(finding.check, [...(groups.get(finding.check) ?? []), { app, finding }])
    }
  }
  const sorted = [...groups.entries()].sort(
    ([, a], [, b]) => order.indexOf(a[0].finding.severity) - order.indexOf(b[0].finding.severity) || b.length - a.length,
  )
  if (sorted.length === 0) {
    return <p className="rounded-[10px] border border-line bg-surface px-5 py-4 text-muted">{t('hygiene.clean')}</p>
  }
  return (
    <div className="grid gap-3.5">
      {sorted.map(([check, items]) => (
        <section key={check} className="grid gap-2 rounded-[10px] border border-line bg-surface px-[18px] py-4">
          <h3 className="flex flex-wrap items-center gap-2.5 font-cond text-base font-semibold">
            <Pill severity={items[0].finding.severity}>{t(`severity.${items[0].finding.severity}`)}</Pill>
            {t(`check.${check}`, check)}
            <span className="font-mono text-xs font-normal text-muted">{items.length}</span>
          </h3>
          <ul className="grid gap-1.5">
            {items.map(({ app, finding }) => (
              <li key={`${app.dir}:${finding.file}:${finding.line}`} className="text-[13px]">
                <b className="font-semibold">{app.name}</b>{' '}
                <span className="text-muted">{finding.message}</span>
                {finding.file && (
                  <span className="block font-mono text-[11px] text-muted">
                    {finding.file}:{finding.line}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

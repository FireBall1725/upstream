// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import type { App } from '../lib/api'
import { hygieneGroups } from '../lib/model'

export default function HygieneTab({ apps }: { apps: App[] }) {
  const { t } = useTranslation()
  const groups = hygieneGroups(apps)
  if (groups.length === 0) return <p className="empty">{t('hygiene.clean')}</p>
  return (
    <div className="hyg">
      {groups.map(([check, items]) => (
        <section key={check} className="hcard">
          <h3>
            <span className={`sev p-${items[0].finding.severity}`}>{t(`severity.${items[0].finding.severity}`)}</span>
            {t(`check.${check}.title`, check)}
            <span className="path muted">{items.length}</span>
          </h3>
          <p>{t(`check.${check}.why`, '')}</p>
          <ul className="hlist">
            {items.map(({ app, finding }) => (
              <li key={`${app.dir}:${finding.file}:${finding.line}`}>
                <b>{app.name}</b> <span className="muted">{finding.message}</span>
                {finding.file && (
                  <span className="path">
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

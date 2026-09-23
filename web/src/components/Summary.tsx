// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { BUMPS, type Bump, type Row } from '../lib/model'

const bumpColour: Record<Bump, string> = {
  major: 'var(--color-major)',
  minor: 'var(--color-minor)',
  patch: 'var(--color-patch)',
  rebuild: 'var(--color-rebuild)',
  current: 'var(--color-current)',
  unchecked: 'var(--color-unchecked)',
}

export default function Summary({ rows, updates }: { rows: Row[]; updates: number }) {
  const { t } = useTranslation()
  const count = (b: Bump) => rows.filter((r) => r.bump === b).length
  return (
    <section className="summary" aria-label={t('summary.label')}>
      <div className="big">
        {updates}
        <small>{t('summary.updates')}</small>
      </div>
      <div>
        <div className="dist" role="img" aria-label={t('summary.dist')}>
          {BUMPS.filter((b) => count(b) > 0).map((b) => (
            <i key={b} style={{ flex: count(b), background: bumpColour[b] }} title={`${t(`bump.${b}`)}: ${count(b)}`} />
          ))}
        </div>
        <div className="legend">
          {BUMPS.map((b) => (
            <span key={b}>
              <i className="sw" style={{ background: bumpColour[b] }} />
              {t(`bump.${b}`)} <b>{count(b)}</b>
            </span>
          ))}
          <span>
            {t('summary.of')} <b>{rows.length}</b> {t('summary.apps')}
          </span>
        </div>
      </div>
    </section>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import { findingFor, type App } from '../lib/api'
import Pill from './Pill'

export default function AppsTable({ apps }: { apps: App[] }) {
  const { t } = useTranslation()
  if (apps.length === 0) {
    return <p className="rounded-[10px] border border-line bg-surface px-5 py-4 text-muted">{t('apps.empty')}</p>
  }
  return (
    <div className="overflow-x-auto rounded-[10px] border border-line bg-surface">
      <table className="w-full border-collapse text-[13px]">
        <thead>
          <tr className="text-left text-[11px] font-semibold tracking-[.06em] text-muted uppercase">
            <th className="border-b border-line px-3 py-2.5">{t('apps.col.app')}</th>
            <th className="border-b border-line px-3 py-2.5">{t('apps.col.source')}</th>
            <th className="border-b border-line px-3 py-2.5">{t('apps.col.version')}</th>
            <th className="border-b border-line px-3 py-2.5">{t('apps.col.pinnedIn')}</th>
            <th className="border-b border-line px-3 py-2.5" />
          </tr>
        </thead>
        <tbody>
          {apps.map((app) => {
            const rows = app.pins.length > 0 ? app.pins : [null]
            return (
              <Fragment key={app.dir}>
                {rows.map((pin, i) => {
                  const finding = pin ? findingFor(app, pin) : app.findings[0]
                  const last = i === rows.length - 1
                  return (
                    <tr key={pin ? `${pin.file}:${pin.line}` : 'none'} className={last ? 'border-b border-line last:border-b-0' : ''}>
                      {i === 0 && (
                        <td rowSpan={rows.length} className="px-3 py-2 align-top">
                          <b className="font-semibold">{app.name}</b>
                          <span className="block font-mono text-[11px] text-muted">{app.namespace}</span>
                        </td>
                      )}
                      {pin ? (
                        <>
                          <td className="px-3 py-2 font-mono text-[11px] text-muted">
                            <span className="mr-1.5 inline-block rounded-[3px] border border-line px-1.5 text-ink">{pin.kind}</span>
                            {pin.kind === 'chart' ? pin.name : pin.image}
                          </td>
                          <td className="px-3 py-2 font-mono text-[12.5px] whitespace-nowrap">
                            {pin.version || '""'}
                            {pin.declared && (
                              <span className="block text-[11px] text-muted">{t('apps.declared', { version: pin.declared })}</span>
                            )}
                          </td>
                          <td className="px-3 py-2 font-mono text-[11px] text-muted">
                            {pin.file.slice(app.dir.length + 1)}:{pin.line}
                            <span className="block">{pin.field}</span>
                          </td>
                        </>
                      ) : (
                        <td colSpan={3} className="px-3 py-2 text-muted">
                          {t('apps.nothingPinned')}
                        </td>
                      )}
                      <td className="px-3 py-2">
                        {finding && <Pill severity={finding.severity}>{t(`check.${finding.check}`, finding.check)}</Pill>}
                      </td>
                    </tr>
                  )
                })}
              </Fragment>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { canBump, channelOf, sourceLabel, type Row } from '../lib/model'
import BumpPill from './BumpPill'

interface Props {
  rows: Row[]
  picked: Set<string>
  highlighted: (r: Row) => boolean
  onOpen: (key: string) => void
  onPick: (key: string, on: boolean) => void
  onPickAll: (on: boolean) => void
}

export default function UpdatesTable({ rows, picked, highlighted, onOpen, onPick, onPickAll }: Props) {
  const { t } = useTranslation()
  const bumpable = rows.filter(canBump)
  const all = bumpable.length > 0 && bumpable.every((r) => picked.has(r.key))
  const some = !all && bumpable.some((r) => picked.has(r.key))

  return (
    <div className="tablebox">
      <table>
        <thead>
          <tr>
            <th className="pick">
              <input
                type="checkbox"
                id="pickAll"
                aria-label={t('table.pickAll')}
                checked={all}
                ref={(el) => {
                  if (el) el.indeterminate = some
                }}
                onChange={(e) => onPickAll(e.target.checked)}
              />
            </th>
            <th>{t('table.app')}</th>
            <th className="hide-sm">{t('table.source')}</th>
            <th>{t('table.running')}</th>
            <th />
            <th>{t('table.latest')}</th>
            <th>{t('table.update')}</th>
            <th className="hide-sm">{t('table.channel')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td colSpan={8} className="empty">
                {t('table.nothing')}
              </td>
            </tr>
          )}
          {rows.map((r) => {
            const p = r.pin
            const newer = p?.latest && p.latest !== p.version
            const channel = p ? channelOf(p.version) : 'stable'
            return (
              <tr
                key={r.key}
                tabIndex={0}
                className={highlighted(r) ? 'sel' : ''}
                onClick={() => onOpen(r.key)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onOpen(r.key)
                }}
              >
                <td className="pick" onClick={(e) => e.stopPropagation()}>
                  {canBump(r) && (
                    <input
                      type="checkbox"
                      aria-label={t('table.pick', { name: r.app.name })}
                      checked={picked.has(r.key)}
                      onChange={(e) => onPick(r.key, e.target.checked)}
                    />
                  )}
                </td>
                <td className="app">
                  <b>{r.app.name}</b>
                  <span className="ns">{r.app.namespace}</span>
                </td>
                <td className="kind hide-sm">
                  {p ? (
                    <>
                      <i>{p.kind}</i>
                      {sourceLabel(p)}
                      {r.app.pins.length > 1 && <span className="more">{t('table.morePins', { count: r.app.pins.length - 1 })}</span>}
                    </>
                  ) : (
                    t('table.nothingPinned')
                  )}
                </td>
                <td className="v old">{p?.version || t('table.none')}</td>
                <td className="arrow">{newer ? '→' : ''}</td>
                <td className="v">{newer ? p.latest : ''}</td>
                <td>
                  <BumpPill bump={r.bump} />
                </td>
                <td className="hide-sm">
                  <span className={`chan ${channel !== 'stable' ? 'hl' : ''}`}>{channel}</span>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import type { Pin, PullRequest, Skip } from '../lib/api'
import { canBump, channelOf, UPDATABLE, type Bump, type Row } from '../lib/model'
import BumpPill from './BumpPill'
import { PRStatePill } from './PRLog'

interface Props {
  row: Row
  inPR: boolean
  skips: Skip[]
  prs: PullRequest[]
  onAdd: () => void
  onClose: () => void
  onSkip: (pin: Pin) => void
  onUnskip: (skip: Skip) => void
}

interface PinProps {
  pin: Pin
  dir: string
  skipped: Skip[]
  onSkip: (pin: Pin) => void
  onUnskip: (skip: Skip) => void
}

function PinBlock({ pin, dir, skipped, onSkip, onUnskip }: PinProps) {
  const { t } = useTranslation()
  const newer = pin.latest && pin.latest !== pin.version
  const checked = pin.update !== 'unchecked' && pin.update !== 'error'
  return (
    <div className="pinblock">
      <div className="pinhead">
        <span className="kind">
          <i>{pin.kind}</i>
          {pin.kind === 'chart' ? pin.name : pin.image}
        </span>
      </div>
      {checked ? (
        <div className="vs">
          <div className="vbox">
            <span className="l">{t('detail.running')}</span>
            <span className="v">{pin.version}</span>
          </div>
          <span className="arrow">→</span>
          <div className={`vbox ${newer ? 'new' : ''}`}>
            <span className="l">{newer ? t('detail.latest') : t('detail.latestSame')}</span>
            <span className="v">{pin.latest}</span>
          </div>
        </div>
      ) : (
        <div className="callout warn">{pin.note}</div>
      )}
      {pin.declared && <p className="muted" style={{ margin: '8px 0 0', fontSize: 13 }}>{t('detail.declared', { declared: pin.declared, version: pin.version })}</p>}
      {(pin.appVersion || pin.latestAppVersion) && (
        <p className="muted" style={{ margin: '8px 0 0', fontSize: 13 }}>
          {t('detail.appInChart')} <span className="v">{pin.appVersion || '?'}</span> → <span className="v">{pin.latestAppVersion || '?'}</span>
        </p>
      )}
      <dl>
        <dt>{t('detail.file')}</dt>
        <dd>
          <span className="path">
            {pin.file.slice(dir.length + 1)}:{pin.line}
          </span>
        </dd>
        <dt>{t('detail.field')}</dt>
        <dd>
          <span className="path">{pin.field}</span>
        </dd>
        <dt>{t('detail.source')}</dt>
        <dd>
          <span className="path">{pin.kind === 'chart' ? pin.chartRepo : pin.image}</span>
        </dd>
        <dt>{t('detail.channel')}</dt>
        <dd>{channelOf(pin.version)}</dd>
        {skipped.length > 0 && (
          <>
            <dt>{t('detail.skipped')}</dt>
            <dd>
              {skipped.map((s) => (
                <span key={s.version} style={{ marginRight: 8 }}>
                  <span className="v">{s.version}</span>{' '}
                  <button className="linkbtn" type="button" onClick={() => onUnskip(s)}>
                    {t('detail.unskip')}
                  </button>
                </span>
              ))}
            </dd>
          </>
        )}
      </dl>
      {newer && UPDATABLE.includes(pin.update as Bump) && (
        <div className="actions" style={{ marginTop: 10 }}>
          <button className="btn ghost" type="button" onClick={() => onSkip(pin)}>
            {t('detail.skip', { version: pin.latest })}
          </button>
        </div>
      )}
    </div>
  )
}

export default function AppDetail({ row, inPR, skips, prs, onAdd, onClose, onSkip, onUnskip }: Props) {
  const { t } = useTranslation()
  const { app } = row
  const mine = prs.filter((p) => p.items.some((it) => it.appDir === app.dir)).slice(0, 5)
  return (
    <>
      <div className="dh">
        <div>
          <h2>{app.name}</h2>
          <span className="ns">{app.namespace}</span>
        </div>
        <BumpPill bump={row.bump} />
        <button className="close" type="button" aria-label={t('detail.close')} onClick={onClose}>
          ×
        </button>
      </div>
      <div className="dsec">
        <h3>{t('detail.pins', { count: app.pins.length })}</h3>
        {app.pins.length === 0 && <div className="callout">{t('detail.nothingPinned')}</div>}
        {app.pins.map((p) => (
          <PinBlock
            key={`${p.file}:${p.line}`}
            pin={p}
            dir={app.dir}
            skipped={skips.filter((s) => s.appDir === app.dir && s.field === p.field)}
            onSkip={onSkip}
            onUnskip={onUnskip}
          />
        ))}
      </div>
      {app.findings.length > 0 && (
        <div className="dsec">
          <h3>{t('detail.hygiene')}</h3>
          <ul className="hlist">
            {app.findings.map((f) => (
              <li key={`${f.check}:${f.file}:${f.line}`}>
                <span className={`sev p-${f.severity}`}>{t(`severity.${f.severity}`)}</span> {f.message}
              </li>
            ))}
          </ul>
        </div>
      )}
      {mine.length > 0 && (
        <div className="dsec">
          <h3>{t('detail.prs')}</h3>
          <ul className="hlist">
            {mine.map((p) => (
              <li key={p.id}>
                <PRStatePill state={p.state} />{' '}
                {p.url ? (
                  <a className="linkbtn" href={p.url} target="_blank" rel="noreferrer">
                    #{p.number} {p.title}
                  </a>
                ) : (
                  p.title
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="dsec">
        <h3>{t('detail.actions')}</h3>
        <div className="actions">
          {canBump(row) && (
            <button className="btn" type="button" onClick={onAdd}>
              {inPR ? t('detail.inPR') : t('detail.addToPR')}
            </button>
          )}
          <button className="btn ghost" type="button" disabled>
            {t('detail.editRule')}
            <span className="later">{t('later')}</span>
          </button>
        </div>
      </div>
    </>
  )
}

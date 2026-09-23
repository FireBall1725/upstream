// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import { formatTime, type PullRequest, type ScanResult, type Status } from '../lib/api'
import PRLog from './PRLog'

export default function ScansTab({ status, scan, prs }: { status: Status | null; scan: ScanResult | null; prs: PullRequest[] }) {
  const { t } = useTranslation()
  if (!status) return null
  const history = scan?.history ?? []
  return (
    <div className="scans">
      <div className="panel">
        <h3>{t('scans.schedule')}</h3>
        <div className="cron">{status.schedule}</div>
        <p className="muted" style={{ margin: '10px 0 0' }}>
          {t('scans.scheduleWhy', { tz: status.timeZone })}
          {status.nextScan && ` ${t('scans.next', { time: formatTime(status.nextScan) })}`}
        </p>
      </div>
      <div className="panel">
        <h3>{t('scans.sources')}</h3>
        <dl>
          <dt>{t('scans.repo')}</dt>
          <dd>
            <code>{status.repo || t('scans.notSet')}</code> · <code>{status.branch}</code>
          </dd>
          <dt>{t('scans.paths')}</dt>
          <dd>
            <code>{status.appGlob}</code>
          </dd>
          <dt>{t('scans.token')}</dt>
          <dd>{status.hasToken ? t('scans.tokenSet') : t('scans.notSet')}</dd>
          <dt>{t('scans.author')}</dt>
          <dd>{status.gitAuthor || t('scans.notSet')}</dd>
          <dt>{t('scans.registries')}</dt>
          <dd>{t('scans.anonymous')}</dd>
          <dt>{t('scans.notify')}</dt>
          <dd>
            {t('scans.off')}
            <span className="later">{t('later')}</span>
          </dd>
        </dl>
      </div>
      <div className="panel wide">
        <h3>{t('scans.prs')}</h3>
        <PRLog prs={prs} />
      </div>
      <div className="panel wide">
        <h3>{t('scans.history')}</h3>
        <div className="tablebox" style={{ border: 0 }}>
          <table className="hist">
            <thead>
              <tr>
                <th>{t('scans.started')}</th>
                <th>{t('scans.commit')}</th>
                <th>{t('scans.apps')}</th>
                <th>{t('scans.checked')}</th>
                <th>{t('scans.updates')}</th>
                <th>{t('scans.errors')}</th>
                <th>{t('scans.took')}</th>
              </tr>
            </thead>
            <tbody>
              {history.map((h) => (
                <tr key={h.startedAt}>
                  <td className="v">{formatTime(h.startedAt)}</td>
                  <td className="v">{h.state === 'ok' ? h.commit?.slice(0, 7) : <span className="p-major pill">{t('scans.failed')}</span>}</td>
                  <td className="v">{h.apps}</td>
                  <td className="v">{h.checked}</td>
                  <td className="v">{h.updates}</td>
                  <td className="v">{h.state === 'ok' ? h.errors : h.error}</td>
                  <td className="v">{((new Date(h.finishedAt).getTime() - new Date(h.startedAt).getTime()) / 1000).toFixed(1)} s</td>
                </tr>
              ))}
              <tr>
                <td colSpan={7} className="muted">
                  {history.length === 0 ? t('scans.noneYet') : t('scans.kept')}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

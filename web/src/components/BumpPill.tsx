// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useTranslation } from 'react-i18next'
import type { Bump } from '../lib/model'

export default function BumpPill({ bump }: { bump: Bump }) {
  const { t } = useTranslation()
  return <span className={`pill p-${bump}`}>{t(`bump.${bump}`)}</span>
}

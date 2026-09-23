// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useCallback, useEffect, useRef, useState } from 'react'
import { getJSON, prBusy, type PullRequest } from '../lib/api'

const pollMs = 2000

// usePRs loads the PR log and polls while any PR is still being made; onSettled runs when the last one finishes.
export function usePRs(onSettled: () => void) {
  const [prs, setPRs] = useState<PullRequest[]>([])
  const busy = prs.some(prBusy)
  const wasBusy = useRef(false)
  const settled = useRef(onSettled)
  useEffect(() => {
    settled.current = onSettled
  }, [onSettled])

  const load = useCallback(async () => {
    try {
      setPRs(await getJSON<PullRequest[]>('/api/prs'))
    } catch {
      // The page still works without the PR log; the next poll tries again.
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (wasBusy.current && !busy) settled.current()
    wasBusy.current = busy
    if (!busy) return
    const id = setInterval(() => void load(), pollMs)
    return () => clearInterval(id)
  }, [busy, load])

  return { prs, reload: load }
}

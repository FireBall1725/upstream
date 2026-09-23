// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { useCallback, useEffect, useState } from 'react'
import { getJSON, type ScanResult } from '../lib/api'

const pollMs = 1500

// useScan loads the latest scan and polls while one is running.
export function useScan() {
  const [scan, setScan] = useState<ScanResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setScan(await getJSON<ScanResult>('/api/scan'))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const running = scan?.state === 'running'
  useEffect(() => {
    if (!running) return
    const id = setInterval(() => void load(), pollMs)
    return () => clearInterval(id)
  }, [running, load])

  const start = useCallback(async () => {
    try {
      setScan(await getJSON<ScanResult>('/api/scan', { method: 'POST' }))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  return { scan, error, start }
}

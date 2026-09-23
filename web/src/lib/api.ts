// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

export interface Status {
  version: string
  repo: string
  branch: string
  appGlob: string
  schedule: string
  problems: string[]
}

// getJSON throws with the server's status line, so a failed call reads as a real error in the UI.
export async function getJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { Accept: 'application/json', ...init?.headers } })
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as T
}

// repoLabel turns a git URL into the owner/name shown in the header.
export function repoLabel(url: string): string {
  return url
    .replace(/^[a-z]+:\/\/[^/]+\//, '')
    .replace(/^git@[^:]+:/, '')
    .replace(/\.git$/, '')
}

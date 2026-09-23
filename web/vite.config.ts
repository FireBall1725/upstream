// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const apiTarget = process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080'
const outDir = fileURLToPath(new URL('../internal/ui/dist', import.meta.url))

// Release builds get VERSION from CI; anything else is a local build and says so.
function computeVersion(): string {
  return process.env.VERSION?.trim() || '0.0.0-dev'
}

// emptyOutDir deletes the committed .gitkeep that lets a Go-only build compile, so put it back.
function keepGitkeep(): Plugin {
  return {
    name: 'keep-gitkeep',
    closeBundle() {
      writeFileSync(`${outDir}/.gitkeep`, '')
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepGitkeep()],
  define: {
    __APP_VERSION__: JSON.stringify(computeVersion()),
  },
  build: {
    outDir,
    emptyOutDir: true,
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true },
      '/healthz': { target: apiTarget, changeOrigin: true },
    },
  },
})

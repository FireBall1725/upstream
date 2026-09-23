// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Global Vitest setup. Loaded before each test file via `setupFiles` in
// vitest.config.ts. The jest-dom import extends Vitest's `expect` with
// DOM-aware matchers (`toBeInTheDocument`, `toHaveAttribute`,
// `toHaveTextContent`, and the rest); without it every assertion has to walk
// the element by hand.
import '@testing-library/jest-dom/vitest'

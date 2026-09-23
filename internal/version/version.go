// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package version carries the running build's release string.
package version

// Version is set at link time from the Dockerfile's VERSION build arg.
// It must keep a constant initialiser, or -ldflags -X silently does nothing.
var Version = ""

// String returns the release, or 0.0.0-dev for a build that was never released.
func String() string {
	if Version == "" {
		return "0.0.0-dev"
	}
	return Version
}

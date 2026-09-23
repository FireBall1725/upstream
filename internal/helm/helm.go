// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package helm runs the helm binary with its caches kept under Upstream's data dir.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Run runs helm in dir and returns stdout, or stderr in the error.
func Run(ctx context.Context, dataDir, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = dir
	home := filepath.Join(dataDir, "helm")
	cmd.Env = append(os.Environ(),
		"HELM_CACHE_HOME="+filepath.Join(home, "cache"),
		"HELM_CONFIG_HOME="+filepath.Join(home, "config"),
		"HELM_DATA_HOME="+filepath.Join(home, "data"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("helm %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Available reports whether the helm binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}

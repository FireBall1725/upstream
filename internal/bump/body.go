// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package bump

import (
	"fmt"
	"strings"

	"github.com/fireball1725/upstream/internal/models"
)

func commitBody(items []models.BumpItem) string {
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%s %s: %s -> %s\n", it.App, it.Field, it.From, it.To)
	}
	return strings.TrimRight(b.String(), "\n")
}

func prBody(items []models.BumpItem) string {
	var b strings.Builder
	b.WriteString("| App | Pin | From | To | Update | Notes |\n|---|---|---|---|---|---|\n")
	charts := false
	for _, it := range items {
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | `%s` | %s | %s |\n", it.App, it.Field, it.From, it.To, it.Bump, releaseLink(it))
		if it.Kind == "chart" {
			charts = true
		}
	}
	if charts {
		b.WriteString("\nChart bumps ran `helm dependency update`, so `Chart.lock` and the vendored `charts/*.tgz` match `Chart.yaml`.\n")
	}
	b.WriteString("\nEvery pin was re-read after the edit and every touched chart rendered with `helm template` before this was pushed.\n")
	return b.String()
}

// releaseLink points at where to read about the new version. It's a best guess from the image or chart name.
func releaseLink(it models.BumpItem) string {
	if it.Kind == "chart" {
		repo, _, _ := strings.Cut(it.Source, " ")
		if strings.HasPrefix(repo, "http") {
			return fmt.Sprintf("[chart repo](%s)", repo)
		}
		return ""
	}
	img := it.Source
	switch {
	case strings.HasPrefix(img, "lscr.io/linuxserver/"):
		return fmt.Sprintf("[releases](https://github.com/linuxserver/docker-%s/releases)", strings.TrimPrefix(img, "lscr.io/linuxserver/"))
	case strings.HasPrefix(img, "ghcr.io/"):
		parts := strings.Split(strings.TrimPrefix(img, "ghcr.io/"), "/")
		if len(parts) >= 2 {
			return fmt.Sprintf("[releases](https://github.com/%s/%s/releases)", parts[0], parts[1])
		}
	case !strings.Contains(strings.Split(img, "/")[0], "."), strings.HasPrefix(img, "docker.io/"):
		name := strings.TrimPrefix(img, "docker.io/")
		if !strings.Contains(name, "/") {
			return fmt.Sprintf("[tags](https://hub.docker.com/_/%s/tags)", name)
		}
		return fmt.Sprintf("[tags](https://hub.docker.com/r/%s/tags)", name)
	}
	return ""
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package inventory

import (
	"fmt"
	"regexp"
	"strings"
)

// Severity orders findings on the hygiene tab.
type Severity string

const (
	SeverityFix  Severity = "fix"
	SeverityTidy Severity = "tidy"
	SeverityNote Severity = "note"
	SeverityInfo Severity = "info"
)

// Finding is a problem in how the repo pins versions, found without touching the network.
type Finding struct {
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
}

var (
	floatingWords = map[string]bool{
		"latest": true, "stable": true, "main": true, "master": true, "edge": true,
		"nightly": true, "rc": true, "beta": true, "dev": true, "develop": true,
	}
	// A bare major like "3" or "9-alpine" moves every time upstream ships a minor.
	bareMajor    = regexp.MustCompile(`^v?\d+(-[A-Za-z][A-Za-z0-9]*)?$`)
	versionRange = regexp.MustCompile(`[\^~<>=*|\s]|\.x\b|^x$`)
)

// IsFloating reports whether a tag moves on its own, so it can't be compared or bumped.
func IsFloating(tag string) bool {
	return floatingWords[strings.ToLower(tag)] || bareMajor.MatchString(tag)
}

func check(rel string, c *chartFile, pins []Pin) []Finding {
	out := []Finding{}
	if len(pins) == 0 {
		out = append(out, Finding{
			Check: "not-a-workload", Severity: SeverityInfo,
			Message: "No image or chart version is pinned here, so there's nothing to update.",
		})
	}

	for _, p := range pins {
		switch {
		case p.Kind == KindImage && p.Version == "":
			out = append(out, Finding{
				Check: "no-tag", Severity: SeverityFix, File: p.File, Line: p.Line,
				Message: fmt.Sprintf("%s has no tag, so it runs whatever latest is when the pod starts.", p.Image),
			})
		case p.Kind == KindImage && IsFloating(p.Version):
			out = append(out, Finding{
				Check: "floating-tag", Severity: SeverityFix, File: p.File, Line: p.Line,
				Message: fmt.Sprintf("%s is pinned to %q, which moves on its own. A pod restart can change the version with no commit.", p.Image, p.Version),
			})
		case p.Kind == KindChart && p.Declared != "":
			out = append(out, Finding{
				Check: "stale-vendored-chart", Severity: SeverityFix, File: p.File, Line: p.Line,
				Message: fmt.Sprintf("Chart.yaml asks for %s %s, but charts/ still holds %s, and Helm renders the vendored copy. Run helm dependency update and commit charts/.", p.Name, p.Declared, p.Version),
			})
		}
	}

	if c == nil {
		return out
	}

	for _, d := range c.Dependencies {
		if d.Vendored != nil && d.Vendored.Library {
			continue
		}
		switch {
		case d.Version.Value == "":
			out = append(out, Finding{
				Check: "unpinned-chart", Severity: SeverityTidy, File: rel + "/Chart.yaml", Line: d.Name.Line,
				Message: fmt.Sprintf("The %s dependency has no version, so Helm takes whatever is newest when the lock is rebuilt.", d.Name.Value),
			})
		case versionRange.MatchString(d.Version.Value):
			out = append(out, Finding{
				Check: "chart-range", Severity: SeverityNote, File: rel + "/Chart.yaml", Line: d.Version.Line,
				Message: fmt.Sprintf("The %s dependency asks for the range %s, not a version. Chart.lock decides what runs.", d.Name.Value, d.Version.Value),
			})
		}
	}

	var top *Pin
	for i := range pins {
		if pins[i].Kind == KindImage && pins[i].Name == "image" {
			top = &pins[i]
			break
		}
	}
	if top == nil {
		return out
	}

	if top.Field == "image.tag" && c.AppVersion.Value != "" && trimV(c.AppVersion.Value) != trimV(top.Version) {
		out = append(out, Finding{
			Check: "appversion-drift", Severity: SeverityTidy, File: rel + "/Chart.yaml", Line: c.AppVersion.Line,
			Message: fmt.Sprintf("appVersion says %s but the image tag is %s. The tag is what runs, so anything reading the chart shows the wrong version.", c.AppVersion.Value, top.Version),
		})
	}
	if c.RenovateHint.Value != "" && normalizeImage(c.RenovateHint.Value) != normalizeImage(top.Image) {
		out = append(out, Finding{
			Check: "renovate-hint", Severity: SeverityTidy, File: rel + "/Chart.yaml", Line: c.RenovateHint.Line,
			Message: fmt.Sprintf("The Renovate hint names %s, but the image is %s.", c.RenovateHint.Value, top.Image),
		})
	}
	return out
}

func trimV(s string) string { return strings.TrimPrefix(s, "v") }

// normalizeImage makes "nginx", "library/nginx" and "docker.io/library/nginx" compare equal.
func normalizeImage(s string) string {
	s = strings.TrimPrefix(strings.ToLower(s), "docker.io/")
	s = strings.TrimPrefix(s, "index.docker.io/")
	if !strings.Contains(s, "/") {
		s = "library/" + s
	}
	return s
}

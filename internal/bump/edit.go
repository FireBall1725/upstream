// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package bump turns picked updates into a branch, a commit and a pull request.
package bump

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Every edit rewrites exactly one line and refuses if that line doesn't hold the old value,
// so a repo that moved since the scan fails loudly instead of getting a wrong diff.

var (
	tagLine        = regexp.MustCompile(`^(\s*tag:\s*)(["']?)([^"'\s#]*)(["']?)(.*)$`)
	appVersionLine = regexp.MustCompile(`^(appVersion:\s*)(["']?)([^"'\s#]*)(["']?)(.*)$`)
	depVersionLine = regexp.MustCompile(`^(\s*(?:-\s*)?version:\s*)(["']?)([^"'\s#]*)(["']?)(.*)$`)
	chartVersion   = regexp.MustCompile(`^(version:\s*)(["']?)(\d+)\.(\d+)\.(\d+)(["']?)(.*)$`)
)

// setLine replaces the value on a 1-based line. quote forces quoting when the original had none.
func setLine(content string, line int, re *regexp.Regexp, from, to string, quote bool) (string, error) {
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return "", fmt.Errorf("line %d is past the end of the file", line)
	}
	m := re.FindStringSubmatch(lines[line-1])
	if m == nil {
		return "", fmt.Errorf("line %d doesn't look like the expected field: %q", line, lines[line-1])
	}
	if m[3] != from {
		return "", fmt.Errorf("line %d holds %q, not %q; the repo changed since the scan", line, m[3], from)
	}
	open, closing := m[2], m[4]
	if open == "" && quote {
		// Unquoted, a tag like 26.8 reaches Helm as the float 26.8.
		open, closing = `"`, `"`
	}
	lines[line-1] = m[1] + open + to + closing + m[5]
	return strings.Join(lines, "\n"), nil
}

// SetImageTag rewrites `tag:` in values.yaml, quoting it.
func SetImageTag(content string, line int, from, to string) (string, error) {
	return setLine(content, line, tagLine, from, to, true)
}

// SetDependencyVersion rewrites a dependency's `version:` in Chart.yaml, keeping its quoting.
func SetDependencyVersion(content string, line int, from, to string) (string, error) {
	return setLine(content, line, depVersionLine, from, to, false)
}

// SetAppVersion rewrites the top-level appVersion, whatever it held before.
func SetAppVersion(content, to string) (string, error) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if m := appVersionLine.FindStringSubmatch(l); m != nil {
			lines[i] = m[1] + `"` + to + `"` + m[5]
			return strings.Join(lines, "\n"), nil
		}
	}
	return "", fmt.Errorf("no appVersion line")
}

// AppVersionIs rewrites appVersion only if it holds from, for the pins that live there.
func AppVersionIs(content string, line int, from, to string) (string, error) {
	return setLine(content, line, appVersionLine, from, to, true)
}

// BumpChartVersion adds one to the chart's own patch version, as every hand-made bump in the repo does,
// and returns the old and new versions. A chart version that isn't x.y.z is left alone.
func BumpChartVersion(content string) (next, from, to string) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		m := chartVersion.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		patch, _ := strconv.Atoi(m[5])
		from = fmt.Sprintf("%s.%s.%s", m[3], m[4], m[5])
		to = fmt.Sprintf("%s.%s.%d", m[3], m[4], patch+1)
		lines[i] = m[1] + m[2] + to + m[6] + m[7]
		return strings.Join(lines, "\n"), from, to
	}
	return content, "", ""
}

// SetManifestImage rewrites repo:from to repo:to on one line of a plain manifest.
func SetManifestImage(content string, line int, repo, from, to string) (string, error) {
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return "", fmt.Errorf("line %d is past the end of the file", line)
	}
	old, next := repo+":"+from, repo+":"+to
	if !strings.Contains(lines[line-1], old) {
		return "", fmt.Errorf("line %d doesn't hold %s; the repo changed since the scan", line, old)
	}
	lines[line-1] = strings.Replace(lines[line-1], old, next, 1)
	return strings.Join(lines, "\n"), nil
}

// ReplaceVersionMentions updates a README's mentions of the old version, whole tokens only, on
// table rows and on lines that name one of the hints (the image). Prose is left alone: the spoolman
// README explains tag naming with an example version that isn't a pin.
// Short versions are left alone, because "1.0" matches too much.
func ReplaceVersionMentions(content, from, to string, hints ...string) string {
	if len(from) < 5 {
		return content
	}
	// A full stop after the version still counts as its end when a sentence ends there.
	re := regexp.MustCompile(`(^|[^\w.-])` + regexp.QuoteMeta(from) + `(\.?(?:$|[^\w.-]))`)
	repl := "${1}" + strings.ReplaceAll(to, "$", "$$") + "${2}"
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "|") && !containsAny(l, hints) {
			continue
		}
		// Two passes, because adjacent matches share the boundary character.
		for range 2 {
			l = re.ReplaceAllString(l, repl)
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// ReplaceChartVersionRow updates a README table row that names the chart version.
func ReplaceChartVersionRow(content, from, to string) string {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "|") && strings.Contains(strings.ToLower(l), "chart version") {
			lines[i] = strings.Replace(l, "`"+from+"`", "`"+to+"`", 1)
		}
	}
	return strings.Join(lines, "\n")
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

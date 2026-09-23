// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package bump

import (
	"strings"
	"testing"
)

func TestSetImageTag(t *testing.T) {
	in := "image:\n  repository: ghcr.io/donkie/spoolman\n  # -- pinned\n  tag: \"0.26.0\"\n  pullPolicy: IfNotPresent\n"
	got, err := SetImageTag(in, 4, "0.26.0", "0.26.1")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(in, `"0.26.0"`, `"0.26.1"`, 1); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	// Unquoted tags get quoted; single quotes stay single; a trailing comment survives.
	for _, tc := range []struct{ line, want string }{
		{"    tag: 2026.8.3", `    tag: "2026.9.3"`},
		{"  tag: '2026.8.3'", "  tag: '2026.9.3'"},
		{"  tag: 2026.8.3 # pinned by hand", `  tag: "2026.9.3" # pinned by hand`},
	} {
		got, err := SetImageTag(tc.line, 1, "2026.8.3", "2026.9.3")
		if err != nil || got != tc.want {
			t.Errorf("%q -> %q (%v), want %q", tc.line, got, err, tc.want)
		}
	}
}

func TestEditsRefuseTheWrongLine(t *testing.T) {
	in := "image:\n  repository: x\n  tag: \"0.26.1\"\n"
	if _, err := SetImageTag(in, 3, "0.26.0", "0.26.2"); err == nil || !strings.Contains(err.Error(), "changed since the scan") {
		t.Errorf("stale from: %v", err)
	}
	if _, err := SetImageTag(in, 2, "x", "y"); err == nil {
		t.Error("a repository line isn't a tag line")
	}
	if _, err := SetImageTag(in, 9, "0.26.1", "0.26.2"); err == nil {
		t.Error("line past the end")
	}
}

func TestChartYAMLEdits(t *testing.T) {
	in := `apiVersion: v2
name: headlamp
version: 1.0.0
appVersion: "1.0.0"
dependencies:
  - name: headlamp
    version: 0.41.0
    repository: https://kubernetes-sigs.github.io/headlamp/
  - name: common
    version: "5.0.3"
`
	got, err := SetDependencyVersion(in, 7, "0.41.0", "0.45.0")
	if err != nil {
		t.Fatal(err)
	}
	got, from, to := BumpChartVersion(got)
	if from != "1.0.0" || to != "1.0.1" {
		t.Errorf("chart version %s -> %s", from, to)
	}
	want := strings.Replace(strings.Replace(in, "version: 0.41.0", "version: 0.45.0", 1), "version: 1.0.0", "version: 1.0.1", 1)
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}

	// Quoted dependency versions keep their quotes.
	if got, _ := SetDependencyVersion(in, 10, "5.0.3", "5.0.4"); !strings.Contains(got, `version: "5.0.4"`) {
		t.Errorf("quotes lost:\n%s", got)
	}
}

func TestAppVersion(t *testing.T) {
	in := "name: sonarr\nversion: 4.0.16\n# renovate: image=lscr.io/linuxserver/sonarr\nappVersion: \"4.0.17.2952-ls306\"\n"
	got, err := AppVersionIs(in, 4, "4.0.17.2952-ls306", "4.0.20.3014-ls325")
	if err != nil || !strings.Contains(got, `appVersion: "4.0.20.3014-ls325"`) {
		t.Errorf("%s %v", got, err)
	}
	// SetAppVersion fixes drift: it doesn't care what the old value was.
	got, err = SetAppVersion("appVersion: 26.8.0-rc.10\n", "26.8.0-rc.12")
	if err != nil || got != "appVersion: \"26.8.0-rc.12\"\n" {
		t.Errorf("%q %v", got, err)
	}
}

func TestBumpChartVersionLeavesOddVersions(t *testing.T) {
	for _, in := range []string{"version: 1.0\n", "name: x\n"} {
		if got, from, _ := BumpChartVersion(in); got != in || from != "" {
			t.Errorf("%q changed to %q", in, got)
		}
	}
	if got, _, _ := BumpChartVersion("version: 4.0.16\n"); got != "version: 4.0.17\n" {
		t.Errorf("got %q", got)
	}
}

func TestSetManifestImage(t *testing.T) {
	in := "        - name: poller\n          image: docker.io/golift/unifi-poller:v2.11.2\n"
	got, err := SetManifestImage(in, 2, "docker.io/golift/unifi-poller", "v2.11.2", "v2.11.3")
	if err != nil || !strings.Contains(got, "unifi-poller:v2.11.3") {
		t.Errorf("%s %v", got, err)
	}
	if _, err := SetManifestImage(in, 1, "docker.io/golift/unifi-poller", "v2.11.2", "v2.11.3"); err == nil {
		t.Error("wrong line should fail")
	}
}

func TestReplaceVersionMentions(t *testing.T) {
	// The real spoolman README: table rows update, the prose example about tag naming doesn't.
	in := "| **Image** | `ghcr.io/donkie/spoolman:0.26.0` (public) |\n" +
		"| **Chart Version** | `0.1.0` |\n" +
		"| **App Version** | `0.26.0` |\n" +
		"\n" +
		"Image tags carry no leading `v`: git `v0.26.0` publishes as image `0.26.0`.\n" +
		"Pull ghcr.io/donkie/spoolman:0.26.0 to test. Not 10.26.0 or 0.26.0.1.\n"
	got := ReplaceVersionMentions(in, "0.26.0", "0.26.1", "ghcr.io/donkie/spoolman")
	got = ReplaceChartVersionRow(got, "0.1.0", "0.1.1")
	want := "| **Image** | `ghcr.io/donkie/spoolman:0.26.1` (public) |\n" +
		"| **Chart Version** | `0.1.1` |\n" +
		"| **App Version** | `0.26.1` |\n" +
		"\n" +
		"Image tags carry no leading `v`: git `v0.26.0` publishes as image `0.26.0`.\n" +
		"Pull ghcr.io/donkie/spoolman:0.26.1 to test. Not 10.26.0 or 0.26.0.1.\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := ReplaceVersionMentions("| v1.0 | 1.0 |", "1.0", "1.1"); got != "| v1.0 | 1.0 |" {
		t.Errorf("short versions should be left alone: %s", got)
	}
	if got := ReplaceVersionMentions("| x | 0.26.0. |", "0.26.0", "0.26.1"); got != "| x | 0.26.1. |" {
		t.Errorf("sentence-ending full stop: %s", got)
	}
}

func TestREADMERowsWhenChartAndAppVersionMatch(t *testing.T) {
	// radarr before #221: chart and app were both 6.0.4.
	in := "| **Chart Version** | `6.0.4` |\n| **App Version** | `6.0.4` |\n"
	got := ReplaceVersionMentions(in, "6.0.4", "6.4.4", "lscr.io/linuxserver/radarr")
	got = SetAppVersionRow(got, "6.4.4")
	got = ReplaceChartVersionRow(got, "6.0.4", "6.0.5")
	if want := "| **Chart Version** | `6.0.5` |\n| **App Version** | `6.4.4` |\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSetAppVersionRowFixesDrift(t *testing.T) {
	// sonarr's row said 4.0.16 while the chart said 4.0.17.2952-ls306.
	in := "| **Helm Chart** | `common` |\n| **App Version** | `4.0.16` |\nApp Version `4.0.16` in prose stays.\n"
	want := "| **Helm Chart** | `common` |\n| **App Version** | `4.0.20.3014-ls325` |\nApp Version `4.0.16` in prose stays.\n"
	if got := SetAppVersionRow(in, "4.0.20.3014-ls325"); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

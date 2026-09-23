// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package versions

import "testing"

// Every case here is a tag list the 2026-09-22 probe of homelab-applications got wrong or had to handle.
func TestLatest(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		candidates []string
		want       string
		bump       Bump
	}{
		{"tautulli date tag is a different scheme", "2.16.1", []string{"2.16.1", "2.18.1", "2021.12.16", "v2.18.1-ls244"}, "2.18.1", Minor},
		{"linuxserver build numbers compare as numbers", "3.1.0.4875-ls25", []string{"3.1.0.4875-ls11", "3.1.0.4875-ls41", "3.1.0.4875-ls9", "latest", "develop"}, "3.1.0.4875-ls41", Rebuild},
		{"linuxserver app update", "4.0.17.2952-ls306", []string{"4.0.20.3014-ls325", "4.0.20.3014-ls324", "5.0.0.1-nightly"}, "4.0.20.3014-ls325", Patch},
		{"v prefix must match", "v0.8.12", []string{"v0.11.4", "0.11.4", "v0.6.22", "main", "git-abc123"}, "v0.11.4", Minor},
		{"nightly stays on nightlies", "26.9.1-nightly.202609212159", []string{"26.9.0", "26.9.1-rc.1", "26.9.1-nightly.202609230016", "26.9.1-nightly.202609200101"}, "26.9.1-nightly.202609230016", Patch},
		{"prowlarr nightly suffix without a number", "2.3.6-nightly", []string{"2.6.5-nightly", "2.7.0", "2.3.6"}, "2.6.5-nightly", Minor},
		{"rc users are told about the stable release", "26.8.0-rc.11", []string{"26.8.0-rc.10", "26.8.0", "26.8.0-nightly.202608010000"}, "26.8.0", Patch},
		{"rc to newer rc", "26.8.0-rc.7", []string{"26.8.0-rc.8"}, "26.8.0-rc.8", Patch},
		{"stable ignores pre-releases", "2.10.4", []string{"2.11.0b0", "2.11.0rc1", "2.10.5"}, "2.10.5", Patch},
		{"beta users see betas and stable", "2.9.0b3", []string{"2.9.0b4", "2.10.4", "2.11.0b0"}, "2.11.0b0", Minor},
		{"suffix family must match", "2.1.2-alpine", []string{"2.1.3", "2.1.3-alpine", "2.2.0-openssl"}, "2.1.3-alpine", Patch},
		{"calver", "2026.8.1", []string{"2026.9.0", "2026.9.0b1", "2026.8.3"}, "2026.9.0", Minor},
		{"homer yy.mm", "v25.05.2", []string{"v26.08.3", "v25.10.1"}, "v26.08.3", Major},
		{"current when nothing is newer", "v0.35.0", []string{"v0.34.0", "v0.35.0"}, "v0.35.0", ""},
		{"current missing from the list still compares", "8.1.2", []string{"8.1.1"}, "8.1.2", ""},
		{"four-part versions", "1.2.5.1", []string{"1.2.5.5", "1.2.5"}, "1.2.5.5", Patch},
		{"chart v prefix", "v1.19.2", []string{"v1.21.2", "v1.20.0", "1.22.0"}, "v1.21.2", Minor},
		{"major jump", "v2.11.2", []string{"v5.2.7", "v2.11.3"}, "v5.2.7", Major},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cur, ok := Parse(tt.current)
			if !ok {
				t.Fatalf("Parse(%q) failed", tt.current)
			}
			got := Latest(cur, tt.candidates)
			if got.Raw != tt.want {
				t.Errorf("Latest = %q, want %q", got.Raw, tt.want)
			}
			if b := BumpOf(cur, got); b != tt.bump {
				t.Errorf("BumpOf = %q, want %q", b, tt.bump)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for _, tag := range []string{"latest", "main", "english-locale", "sha256:abc", ""} {
		if _, ok := Parse(tag); ok {
			t.Errorf("Parse(%q) should fail", tag)
		}
	}
}

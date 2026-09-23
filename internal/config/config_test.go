// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package config

import (
	"strings"
	"testing"
)

func TestProblems(t *testing.T) {
	ok := Config{Repo: "r", Schedule: "0 */6 * * *", GitHubToken: "t", GitAuthorName: "a", GitAuthorEmail: "a@example.com"}
	if p := ok.Problems(); len(p) != 0 {
		t.Errorf("complete config has problems: %v", p)
	}

	bad := ok
	bad.Schedule = "every six hours"
	p := bad.Problems()
	if len(p) != 1 || !strings.Contains(p[0].Error(), "UPSTREAM_SCHEDULE") {
		t.Errorf("bad schedule: %v", p)
	}
}

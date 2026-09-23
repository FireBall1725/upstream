// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package versions decides whether one tag is a newer release of the same thing as another.
package versions

import (
	"regexp"
	"strconv"
	"strings"
)

// Channel is how stable a release claims to be.
type Channel string

const (
	Stable  Channel = "stable"
	RC      Channel = "rc"
	Beta    Channel = "beta"
	Alpha   Channel = "alpha"
	Nightly Channel = "nightly"
)

// Bump is how big a step an update is.
type Bump string

const (
	Major   Bump = "major"
	Minor   Bump = "minor"
	Patch   Bump = "patch"
	Rebuild Bump = "rebuild"
)

// Version is a parsed tag. Two versions only compare when they share a Shape.
type Version struct {
	Raw     string
	V       bool
	Nums    []int
	Channel Channel
	// Pre orders releases within a pre-release channel: rc.11, b3, nightly.202609212159.
	Pre int64
	// LS is the linuxserver build number from a -lsNN suffix; the app itself didn't change.
	LS int
	// Suffix is anything else after the version, like "-alpine" or "-debian".
	Suffix string
}

var (
	lead    = regexp.MustCompile(`^(v?)(\d+(?:\.\d+)*)(.*)$`)
	lsBuild = regexp.MustCompile(`^-ls(\d+)$`)
	pre     = regexp.MustCompile(`(?i)^[-.]?(alpha|a|beta|b|rc|pre|nightly|dev)[.-]?(\d*)(.*)$`)
)

// Parse reads a tag. ok is false for anything without a leading version number, like "latest".
func Parse(tag string) (Version, bool) {
	m := lead.FindStringSubmatch(tag)
	if m == nil {
		return Version{}, false
	}
	v := Version{Raw: tag, V: m[1] == "v", Channel: Stable}
	for part := range strings.SplitSeq(m[2], ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, false
		}
		v.Nums = append(v.Nums, n)
	}

	rest := m[3]
	if ls := lsBuild.FindStringSubmatch(rest); ls != nil {
		v.LS, _ = strconv.Atoi(ls[1])
		return v, true
	}
	if p := pre.FindStringSubmatch(rest); p != nil {
		switch strings.ToLower(p[1]) {
		case "alpha", "a":
			v.Channel = Alpha
		case "beta", "b":
			v.Channel = Beta
		case "rc", "pre":
			v.Channel = RC
		default:
			v.Channel = Nightly
		}
		if p[2] != "" {
			v.Pre, _ = strconv.ParseInt(p[2], 10, 64)
		}
		rest = p[3]
	}
	v.Suffix = rest
	return v, true
}

// calver reports a year-first version like 2026.9.0 or 2021.12.16.
func (v Version) calver() bool { return v.Nums[0] >= 1000 }

// sameShape is the rule that keeps "2021.12.16" from beating "2.16.1" and "-alpine" from
// replacing "-debian": same prefix, same count of numbers, same suffix, same numbering scheme.
func (v Version) sameShape(o Version) bool {
	return v.V == o.V && len(v.Nums) == len(o.Nums) && v.Suffix == o.Suffix &&
		v.calver() == o.calver() && (v.LS > 0) == (o.LS > 0)
}

// accepts reports whether someone running v would take o: nightly users see nightlies,
// and rc or beta users also see the stable release they were waiting for.
func (v Version) accepts(o Version) bool {
	switch v.Channel {
	case Stable:
		return o.Channel == Stable
	case Nightly:
		return o.Channel == Nightly
	default:
		return o.Channel == v.Channel || o.Channel == Stable
	}
}

var channelRank = map[Channel]int{Alpha: 0, Beta: 1, RC: 2, Nightly: 2, Stable: 3}

// Compare orders two versions of the same shape.
func Compare(a, b Version) int {
	for i := range a.Nums {
		if a.Nums[i] != b.Nums[i] {
			return sign(a.Nums[i] - b.Nums[i])
		}
	}
	if a.Channel != b.Channel {
		return sign(channelRank[a.Channel] - channelRank[b.Channel])
	}
	if a.Pre != b.Pre {
		if a.Pre < b.Pre {
			return -1
		}
		return 1
	}
	return sign(a.LS - b.LS)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// Latest returns the newest candidate someone on current would upgrade to, or current itself.
func Latest(current Version, candidates []string) Version {
	best := current
	for _, c := range candidates {
		v, ok := Parse(c)
		if !ok || !current.sameShape(v) || !current.accepts(v) {
			continue
		}
		if Compare(v, best) > 0 {
			best = v
		}
	}
	return best
}

// BumpOf says how big the step from a to b is. It's empty when b isn't newer.
func BumpOf(a, b Version) Bump {
	if Compare(b, a) <= 0 {
		return ""
	}
	for i := range a.Nums {
		if a.Nums[i] != b.Nums[i] {
			switch i {
			case 0:
				return Major
			case 1:
				return Minor
			}
			return Patch
		}
	}
	if a.Channel != b.Channel || a.Pre != b.Pre {
		return Patch
	}
	return Rebuild
}

// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package sources lists the versions a registry or Helm repo has published.
package sources

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ChartVersion is one entry from a Helm repo index.
type ChartVersion struct {
	Version    string
	AppVersion string
}

// Lookup fetches each source once and remembers it, so one Lookup should live for one scan.
type Lookup struct {
	http      *http.Client
	userAgent string

	mu    sync.Mutex
	cache map[string]*entry
}

type entry struct {
	once   sync.Once
	tags   []string
	charts map[string][]ChartVersion
	err    error
}

func New(userAgent string) *Lookup {
	return &Lookup{
		http:      &http.Client{Timeout: 60 * time.Second},
		userAgent: userAgent,
		cache:     map[string]*entry{},
	}
}

func (l *Lookup) get(key string, fill func(*entry)) *entry {
	l.mu.Lock()
	e, ok := l.cache[key]
	if !ok {
		e = &entry{}
		l.cache[key] = e
	}
	l.mu.Unlock()
	e.once.Do(func() { fill(e) })
	return e
}

// registryName maps an image to where its tags can be listed. lscr.io is a redirect
// in front of GHCR and has no tag list of its own.
func registryName(image string) string {
	if rest, ok := strings.CutPrefix(image, "lscr.io/"); ok {
		return "ghcr.io/" + rest
	}
	return image
}

// ImageTags lists every tag of an image repository, following pagination to the end.
func (l *Lookup) ImageTags(ctx context.Context, image string) ([]string, error) {
	e := l.get("image|"+image, func(e *entry) {
		e.tags, e.err = l.listTags(ctx, registryName(image))
	})
	return e.tags, e.err
}

func (l *Lookup) listTags(ctx context.Context, repo string) ([]string, error) {
	r, err := name.NewRepository(repo)
	if err != nil {
		return nil, fmt.Errorf("image name %q: %w", repo, err)
	}
	tags, err := remote.List(r, remote.WithContext(ctx), remote.WithAuth(authn.Anonymous), remote.WithUserAgent(l.userAgent))
	if err != nil {
		return nil, fmt.Errorf("list tags for %s: %w", repo, err)
	}
	return tags, nil
}

// ChartVersions lists the published versions of a chart, from index.yaml or an OCI registry.
func (l *Lookup) ChartVersions(ctx context.Context, repoURL, chart string) ([]ChartVersion, error) {
	if rest, ok := strings.CutPrefix(repoURL, "oci://"); ok {
		ref := strings.TrimSuffix(rest, "/") + "/" + chart
		e := l.get("image|"+ref, func(e *entry) { e.tags, e.err = l.listTags(ctx, ref) })
		if e.err != nil {
			return nil, e.err
		}
		out := make([]ChartVersion, 0, len(e.tags))
		for _, t := range e.tags {
			// OCI tags can't hold "+", so Helm pushes build metadata with "_" instead.
			out = append(out, ChartVersion{Version: strings.ReplaceAll(t, "_", "+")})
		}
		return out, nil
	}

	e := l.get("helm|"+repoURL, func(e *entry) { e.charts, e.err = l.fetchIndex(ctx, repoURL) })
	if e.err != nil {
		return nil, e.err
	}
	versions, ok := e.charts[chart]
	if !ok {
		return nil, fmt.Errorf("%s has no chart named %s", repoURL, chart)
	}
	return versions, nil
}

func (l *Lookup) fetchIndex(ctx context.Context, repoURL string) (map[string][]ChartVersion, error) {
	url := strings.TrimSuffix(repoURL, "/") + "/index.yaml"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", l.userAgent)
	res, err := l.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, res.Status)
	}
	return parseIndex(res.Body)
}

var indexKey = regexp.MustCompile(`^(version|appVersion):\s*["']?([^"'\s]+)["']?\s*$`)

// parseIndex reads index.yaml line by line. prometheus-community's is over 6 MB, and
// decoding the whole document would hold all of it in memory for two fields per entry.
// Indentation is learned from the file: Helm writes 2 spaces, tailscale's index uses 4 and 8.
func parseIndex(r io.Reader) (map[string][]ChartVersion, error) {
	out := map[string][]ChartVersion{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4<<20)

	inEntries := false
	chart := ""
	chartIndent, itemIndent := -1, -1
	var cur *ChartVersion
	flush := func() {
		if cur != nil && cur.Version != "" && chart != "" {
			out[chart] = append(out[chart], *cur)
		}
		cur = nil
	}
	key := func(text string) {
		if m := indexKey.FindStringSubmatch(text); m != nil {
			if m[1] == "version" {
				cur.Version = m[2]
			} else {
				cur.AppVersion = m[2]
			}
		}
	}
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		indent := len(line) - len(trimmed)
		if indent == 0 {
			flush()
			inEntries = strings.HasPrefix(line, "entries:")
			chart, chartIndent, itemIndent = "", -1, -1
			continue
		}
		if !inEntries {
			continue
		}
		if chartIndent < 0 {
			chartIndent = indent
		}
		isItem := strings.HasPrefix(trimmed, "- ")
		switch {
		case indent == chartIndent && !isItem && strings.HasSuffix(trimmed, ":"):
			flush()
			chart, itemIndent = strings.Trim(strings.TrimSuffix(trimmed, ":"), `"'`), -1
		case isItem && (indent == itemIndent || (itemIndent < 0 && indent >= chartIndent)):
			flush()
			cur, itemIndent = &ChartVersion{}, indent
			key(trimmed[2:])
		case cur != nil && indent == itemIndent+2 && !isItem:
			key(trimmed)
		}
	}
	flush()
	return out, sc.Err()
}

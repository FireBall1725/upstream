// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package bump

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHub is the handful of REST calls a bump PR needs.
type GitHub struct {
	BaseURL string
	Token   string
	Owner   string
	Repo    string
	http    *http.Client
}

// ParseGitHubRepo reads owner and name from an https://github.com/owner/name(.git) URL.
func ParseGitHubRepo(repoURL string) (owner, name string, ok bool) {
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func NewGitHub(token, owner, repo string) *GitHub {
	return &GitHub{BaseURL: "https://api.github.com", Token: token, Owner: owner, Repo: repo, http: &http.Client{Timeout: 30 * time.Second}}
}

func (g *GitHub) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.BaseURL+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(data, &e)
		msg := e.Message
		for _, x := range e.Errors {
			if x.Message != "" {
				msg += ": " + x.Message
			}
		}
		return fmt.Errorf("GitHub %s %s: %s %s", method, path, res.Status, msg)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// PR is what Upstream reads back about a pull request.
type PR struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Merged  bool   `json:"merged"`
}

func (g *GitHub) CreatePR(ctx context.Context, title, body, head, base string) (PR, error) {
	var pr PR
	err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls", g.Owner, g.Repo),
		map[string]any{"title": title, "body": body, "head": head, "base": base}, &pr)
	return pr, err
}

func (g *GitHub) MergePR(ctx context.Context, number int) error {
	return g.do(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", g.Owner, g.Repo, number),
		map[string]any{"merge_method": "squash"}, nil)
}

func (g *GitHub) GetPR(ctx context.Context, number int) (PR, error) {
	var pr PR
	err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls/%d", g.Owner, g.Repo, number), nil, &pr)
	return pr, err
}

// DeleteBranch cleans up after a merge; repos with auto-delete turned on return 422, which is fine.
func (g *GitHub) DeleteBranch(ctx context.Context, branch string) {
	_ = g.do(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", g.Owner, g.Repo, branch), nil, nil)
}

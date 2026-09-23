// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package inventory

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeTree creates each file under root, making directories as needed.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeChartTgz packs a vendored chart the way `helm dependency update` does: <name>/Chart.yaml inside.
func writeChartTgz(t *testing.T, path, name string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for f, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name + "/" + f, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

const commonChart = "name: common\nversion: 5.0.3\ntype: library\n"
const commonDep = "  - name: common\n    repository: https://example.com/common\n    version: 5.0.3\n"

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		// Tag empty, so appVersion is the pin; Renovate hint misspells the image.
		"apps/app-media/sonarr/Chart.yaml":  "apiVersion: v2\nname: sonarr\nversion: 4.0.16\n# renovate: image=lscr.io/linuxserver/sonar\nappVersion: \"4.0.17.2952-ls306\"\ndependencies:\n" + commonDep,
		"apps/app-media/sonarr/values.yaml": "image:\n  repository: lscr.io/linuxserver/sonarr\n  tag: \"\"\n",

		// Tag set and appVersion stale; this app doesn't vendor common, so only another app proves it's a library.
		"apps/app-firebin/firebin-api/Chart.yaml":  "apiVersion: v2\nname: firebin-api\nversion: 1.0.0\nappVersion: \"26.8.0-rc.10\"\ndependencies:\n" + commonDep,
		"apps/app-firebin/firebin-api/values.yaml": "image:\n  repository: ghcr.io/fireball1725/firebin-api\n  tag: \"26.8.0-rc.11\"\n",

		// v prefix differs only, which isn't drift.
		"apps/app-kometa/kometa/Chart.yaml":  "apiVersion: v2\nname: kometa\nversion: 1.0.0\nappVersion: \"2.1.0\"\n",
		"apps/app-kometa/kometa/values.yaml": "image:\n  repository: kometateam/kometa\n  tag: v2.1.0\n",

		// Upstream chart whose vendored copy is older than declared, plus an image override with no repository.
		"apps/app-home-assistant/home-assistant/Chart.yaml":  "apiVersion: v2\nname: home-assistant\nversion: 1.0.0\nappVersion: \"1.0.0\"\ndependencies:\n  - name: home-assistant\n    repository: http://example.com/ha\n    version: 0.3.53\n",
		"apps/app-home-assistant/home-assistant/values.yaml": "home-assistant:\n  image:\n    tag: 2026.8.3\n",

		// Two images under local keys, both floating.
		"apps/app-terminus/terminus/Chart.yaml":  "apiVersion: v2\nname: terminus\nversion: 1.0.0\nappVersion: \"latest\"\n",
		"apps/app-terminus/terminus/values.yaml": "web:\n  image:\n    repository: ghcr.io/usetrmnl/terminus\n    tag: \"latest\"\nvalkey:\n  image:\n    repository: valkey/valkey\n    tag: \"9-alpine\"\n",

		// Unpinned and ranged chart dependencies.
		"apps/app-openclaw/openclaw/Chart.yaml":     "apiVersion: v2\nname: openclaw\nversion: 1.0.0\ndependencies:\n  - name: openclaw\n    repository: https://example.com/openclaw\n",
		"apps/app-cribl-edge/cribl-edge/Chart.yaml": "apiVersion: v2\nname: cribl-edge\nversion: 1.0.0\ndependencies:\n  - name: edge\n    repository: https://example.com/cribl\n    version: \"^4.12.1\"\n",

		// Raw manifests, one with a registry port.
		"apps/app-unpoller/unpoller/deployment.yaml": "spec:\n  template:\n    spec:\n      containers:\n        - name: unpoller\n          image: docker.io/golift/unifi-poller:v2.11.2\n        - name: side\n          image: \"registry.local:5000/tools/side:1.0\"\n",

		// Nothing pinned at all.
		"apps/app-ai/ollama/Chart.yaml":             "apiVersion: v2\nname: ollama\nversion: 1.0.0\nappVersion: \"1.0.0\"\n",
		"apps/app-ai/ollama/values.yaml":            "ollama:\n  ip: 10.0.1.191\n",
		"apps/app-ai/ollama/templates/service.yaml": "kind: Service\n",

		// A file matching the glob is not an app.
		"apps/app-ai/README.md": "not an app\n",
	})
	writeChartTgz(t, filepath.Join(root, "apps/app-media/sonarr/charts/common-5.0.3.tgz"), "common", map[string]string{"Chart.yaml": commonChart})
	writeChartTgz(t, filepath.Join(root, "apps/app-home-assistant/home-assistant/charts/home-assistant-0.3.32.tgz"), "home-assistant", map[string]string{
		"Chart.yaml":  "name: home-assistant\nversion: 0.3.32\nappVersion: \"2026.4.1\"\n",
		"values.yaml": "image:\n  repository: ghcr.io/home-assistant/home-assistant\n",
	})
	return root
}

func byName(t *testing.T, apps []App) map[string]App {
	t.Helper()
	m := map[string]App{}
	for _, a := range apps {
		m[a.Name] = a
	}
	return m
}

func checks(a App) []string {
	out := []string{}
	for _, f := range a.Findings {
		out = append(out, f.Check)
	}
	sort.Strings(out)
	return out
}

func TestScan(t *testing.T) {
	apps, err := Scan(fixture(t), "apps/*/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 9 {
		t.Fatalf("got %d apps, want 9 (README.md is not an app)", len(apps))
	}
	got := byName(t, apps)

	t.Run("empty tag falls back to appVersion", func(t *testing.T) {
		a := got["sonarr"]
		if a.Namespace != "app-media" || len(a.Pins) != 1 {
			t.Fatalf("pins %+v", a.Pins)
		}
		p := a.Pins[0]
		if p.Version != "4.0.17.2952-ls306" || p.File != "apps/app-media/sonarr/Chart.yaml" || p.Line != 5 || p.Field != "appVersion" {
			t.Errorf("pin %+v", p)
		}
		if strings.Join(checks(a), ",") != "renovate-hint" {
			t.Errorf("findings %v", checks(a))
		}
	})

	t.Run("library chart learned from another app", func(t *testing.T) {
		a := got["firebin-api"]
		if len(a.Pins) != 1 || a.Pins[0].Kind != KindImage {
			t.Fatalf("common should be dropped, pins %+v", a.Pins)
		}
		if p := a.Pins[0]; p.Line != 3 || p.Field != "image.tag" {
			t.Errorf("pin %+v", p)
		}
		if strings.Join(checks(a), ",") != "appversion-drift" {
			t.Errorf("findings %v", checks(a))
		}
	})

	t.Run("a v prefix alone is not drift", func(t *testing.T) {
		if c := checks(got["kometa"]); len(c) != 0 {
			t.Errorf("findings %v", c)
		}
	})

	t.Run("vendored chart wins over Chart.yaml", func(t *testing.T) {
		a := got["home-assistant"]
		if len(a.Pins) != 2 {
			t.Fatalf("pins %+v", a.Pins)
		}
		chart, img := a.Pins[0], a.Pins[1]
		if chart.Version != "0.3.32" || chart.Declared != "0.3.53" || chart.AppVersion != "2026.4.1" || chart.Line != 8 {
			t.Errorf("chart pin %+v", chart)
		}
		if img.Image != "ghcr.io/home-assistant/home-assistant" || img.Version != "2026.8.3" || img.Field != "home-assistant.image.tag" || img.Line != 3 {
			t.Errorf("image pin %+v", img)
		}
		if strings.Join(checks(a), ",") != "stale-vendored-chart" {
			t.Errorf("findings %v", checks(a))
		}
	})

	t.Run("images under local keys", func(t *testing.T) {
		a := got["terminus"]
		if len(a.Pins) != 2 || a.Pins[0].Name != "web.image" || a.Pins[1].Name != "valkey.image" {
			t.Fatalf("pins %+v", a.Pins)
		}
		if strings.Join(checks(a), ",") != "floating-tag,floating-tag" {
			t.Errorf("findings %v", checks(a))
		}
	})

	t.Run("unpinned and ranged dependencies", func(t *testing.T) {
		if c := checks(got["openclaw"]); strings.Join(c, ",") != "unpinned-chart" {
			t.Errorf("openclaw findings %v", c)
		}
		if c := checks(got["cribl-edge"]); strings.Join(c, ",") != "chart-range" {
			t.Errorf("cribl-edge findings %v", c)
		}
	})

	t.Run("raw manifests", func(t *testing.T) {
		a := got["unpoller"]
		if len(a.Pins) != 2 {
			t.Fatalf("pins %+v", a.Pins)
		}
		if p := a.Pins[0]; p.Image != "docker.io/golift/unifi-poller" || p.Version != "v2.11.2" || p.Line != 6 || p.File != "apps/app-unpoller/unpoller/deployment.yaml" || p.Field != "deployment.yaml image" {
			t.Errorf("pin %+v", p)
		}
		if p := a.Pins[1]; p.Image != "registry.local:5000/tools/side" || p.Version != "1.0" || p.Field != "deployment.yaml image[2]" {
			t.Errorf("port pin %+v", p)
		}
	})

	t.Run("nothing pinned", func(t *testing.T) {
		if c := checks(got["ollama"]); strings.Join(c, ",") != "not-a-workload" {
			t.Errorf("findings %v", c)
		}
	})
}

func TestIsFloating(t *testing.T) {
	for tag, want := range map[string]bool{
		"latest": true, "LATEST": true, "3": true, "v2": true, "9-alpine": true, "nightly": true,
		"2.1.2-alpine": false, "26.9.1-nightly.202609212159": false, "v0.35.0": false, "english-locale": false,
	} {
		if got := IsFloating(tag); got != want {
			t.Errorf("IsFloating(%q) = %v, want %v", tag, got, want)
		}
	}
}

// TestSnapshot runs against a real checkout of homelab-applications at f64f9b5, the commit
// the mockup was built from: UPSTREAM_TEST_REPO=/path/to/checkout go test ./internal/inventory
func TestSnapshot(t *testing.T) {
	root := os.Getenv("UPSTREAM_TEST_REPO")
	if root == "" {
		t.Skip("UPSTREAM_TEST_REPO not set")
	}
	apps, err := Scan(root, "apps/*/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 58 {
		t.Fatalf("got %d apps, want 58", len(apps))
	}

	byCheck := map[string][]string{}
	for _, a := range apps {
		for _, p := range a.Pins {
			if p.Kind == KindChart && p.Name == "common" {
				t.Errorf("%s: common library listed as a chart pin", a.Name)
			}
		}
		for _, f := range a.Findings {
			byCheck[f.Check] = append(byCheck[f.Check], a.Name)
		}
	}
	for check, want := range map[string]string{
		"stale-vendored-chart": "cert-manager,cloudnative-pg,headlamp,homarr,home-assistant,kube-prometheus-stack,open-webui,openclaw,tailscale,traefik,wikijs",
		"floating-tag":         "error-pages,ha-mcp,homebridge,lazylibrarian,pinchflat,terminus,terminus,ttyd,wikijs",
		"appversion-drift":     "firebin-api,firebin-mcp,firebin-web,pcexpress-mcp,pinchflat,renovate,ttyd",
		"renovate-hint":        "homer",
		"chart-range":          "cribl-edge",
		"not-a-workload":       "ollama",
	} {
		got := byCheck[check]
		sort.Strings(got)
		if strings.Join(got, ",") != want {
			t.Errorf("%s:\n got  %s\n want %s", check, strings.Join(got, ","), want)
		}
	}
}

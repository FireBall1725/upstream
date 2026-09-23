// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package inventory walks a GitOps repo and records every pinned version and where it lives.
package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Kind is what a pin points at.
type Kind string

const (
	KindImage Kind = "image"
	KindChart Kind = "chart"
)

// Pin is one version the repo commits to, with the exact line a bump has to edit.
type Pin struct {
	Kind Kind `json:"kind"`
	// Name is the values path of an image ("image", "web.image") or a chart dependency's name.
	Name string `json:"name"`
	// Image is the repository without a tag; empty for charts.
	Image string `json:"image,omitempty"`
	// ChartRepo is the Helm repository URL; empty for images.
	ChartRepo string `json:"chartRepo,omitempty"`
	// Version is what actually gets deployed.
	Version string `json:"version"`
	// Declared is set when the file asks for something other than Version, like a stale vendored chart.
	Declared string `json:"declared,omitempty"`
	// AppVersion is the application version inside a chart, from the vendored chart.
	AppVersion string `json:"appVersion,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Field      string `json:"field"`
}

// App is one directory the ApplicationSet turns into an Argo CD Application.
type App struct {
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Dir       string    `json:"dir"`
	Pins      []Pin     `json:"pins"`
	Findings  []Finding `json:"findings"`

	// libraries are the repo|name keys of vendored library charts seen in this app.
	libraries []string
}

// Scan reads every directory under root that matches glob. The app is the directory's
// name and the namespace its parent's, the same way the homelab ApplicationSet names them.
func Scan(root, glob string) ([]App, error) {
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(glob)))
	if err != nil {
		return nil, fmt.Errorf("app glob %q: %w", glob, err)
	}
	sort.Strings(matches)

	apps := []App{}
	for _, dir := range matches {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		app, err := readApp(dir, filepath.ToSlash(rel))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		apps = append(apps, app)
	}
	dropLibraries(apps)
	return apps, nil
}

// dropLibraries removes library charts from apps that don't vendor them. Only a vendored
// copy says `type: library`, so one app that vendors common marks it for every app.
func dropLibraries(apps []App) {
	libs := map[string]bool{}
	for _, a := range apps {
		for _, l := range a.libraries {
			libs[l] = true
		}
	}
	for i := range apps {
		kept := apps[i].Pins[:0]
		for _, p := range apps[i].Pins {
			if p.Kind == KindChart && libs[p.ChartRepo+"|"+p.Name] {
				continue
			}
			kept = append(kept, p)
		}
		apps[i].Pins = kept
	}
}

func readApp(dir, rel string) (App, error) {
	app := App{
		Namespace: filepath.Base(filepath.Dir(dir)),
		Name:      filepath.Base(dir),
		Dir:       rel,
		Pins:      []Pin{},
		Findings:  []Finding{},
	}

	chart, err := readChart(dir)
	if err != nil {
		return app, err
	}
	if chart == nil {
		pins, err := readManifests(dir, rel)
		if err != nil {
			return app, err
		}
		app.Pins = pins
	} else {
		values, err := readValues(dir)
		if err != nil {
			return app, err
		}
		app.Pins = chartPins(rel, chart, values)
		for _, d := range chart.Dependencies {
			if d.Vendored != nil && d.Vendored.Library {
				app.libraries = append(app.libraries, d.Repository+"|"+d.Name.Value)
			}
		}
	}

	app.Findings = check(rel, chart, app.Pins)
	return app, nil
}

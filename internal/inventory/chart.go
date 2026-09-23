// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package inventory

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// scalar is a YAML value and the 1-based line it sits on.
type scalar struct {
	Value string
	Line  int
}

type dependency struct {
	Name       scalar
	Version    scalar
	Repository string
	// Vendored is the chart in charts/, which is what Helm actually renders.
	Vendored *vendoredChart
}

type chartFile struct {
	AppVersion   scalar
	Dependencies []dependency
	// RenovateHint is the image named in a "# renovate: image=" comment, if any.
	RenovateHint scalar
}

type vendoredChart struct {
	Version    string
	AppVersion string
	Library    bool
	// ImageRepo is image.repository from the vendored chart's own values.yaml.
	ImageRepo string
}

var renovateHint = regexp.MustCompile(`#\s*renovate:\s*image=(\S+)`)

// readChart returns nil for a directory with no Chart.yaml.
func readChart(dir string) (*chartFile, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse Chart.yaml: %w", err)
	}
	root := mappingRoot(&doc)
	if root == nil {
		return nil, errors.New("parse Chart.yaml: not a mapping")
	}

	c := &chartFile{AppVersion: scalarAt(root, "appVersion")}
	if deps := child(root, "dependencies"); deps != nil && deps.Kind == yaml.SequenceNode {
		for _, d := range deps.Content {
			dep := dependency{
				Name:       scalarAt(d, "name"),
				Version:    scalarAt(d, "version"),
				Repository: scalarAt(d, "repository").Value,
			}
			if dep.Version.Line == 0 {
				// No version key at all; point edits at the dependency's name line instead.
				dep.Version.Line = dep.Name.Line
			}
			dep.Vendored, err = readVendored(dir, dep.Name.Value)
			if err != nil {
				return nil, err
			}
			c.Dependencies = append(c.Dependencies, dep)
		}
	}

	for i, line := range strings.Split(string(raw), "\n") {
		if m := renovateHint.FindStringSubmatch(line); m != nil {
			c.RenovateHint = scalar{Value: m[1], Line: i + 1}
			break
		}
	}
	return c, nil
}

// readVendored opens charts/<name>-*.tgz and reads the chart's own Chart.yaml and values.yaml.
func readVendored(dir, name string) (*vendoredChart, error) {
	matches, _ := filepath.Glob(filepath.Join(dir, "charts", name+"-*.tgz"))
	for _, path := range matches {
		v, err := readTgz(path, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		// A glob for "common-*" also matches "common-extras-1.0.0.tgz"; keep the one that names this chart.
		if v != nil {
			return v, nil
		}
	}
	return nil, nil
}

func readTgz(path, name string) (*vendoredChart, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	var chartRaw, valuesRaw []byte
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch h.Name {
		case name + "/Chart.yaml":
			chartRaw, err = io.ReadAll(io.LimitReader(tr, 1<<20))
		case name + "/values.yaml":
			valuesRaw, err = io.ReadAll(io.LimitReader(tr, 4<<20))
		}
		if err != nil {
			return nil, err
		}
	}
	if chartRaw == nil {
		return nil, nil
	}

	var meta struct {
		Name       string `yaml:"name"`
		Version    string `yaml:"version"`
		AppVersion string `yaml:"appVersion"`
		Type       string `yaml:"type"`
	}
	if err := yaml.Unmarshal(chartRaw, &meta); err != nil {
		return nil, fmt.Errorf("parse Chart.yaml: %w", err)
	}
	if meta.Name != name {
		return nil, nil
	}
	v := &vendoredChart{Version: meta.Version, AppVersion: meta.AppVersion, Library: meta.Type == "library"}

	var values struct {
		Image struct {
			Repository string `yaml:"repository"`
		} `yaml:"image"`
	}
	// A subchart's values can be large or odd; a parse failure only costs the image name.
	if yaml.Unmarshal(valuesRaw, &values) == nil {
		v.ImageRepo = values.Image.Repository
	}
	return v, nil
}

func mappingRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		return doc.Content[0]
	}
	return nil
}

func child(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalarAt(m *yaml.Node, key string) scalar {
	n := child(m, key)
	if n == nil || n.Kind != yaml.ScalarNode {
		return scalar{}
	}
	return scalar{Value: strings.TrimSpace(n.Value), Line: n.Line}
}

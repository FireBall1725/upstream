// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// imageBlock is an `image:` mapping in values.yaml, at the top level or one key down.
type imageBlock struct {
	// Parent is the key the block sits under, empty for a top-level `image:`.
	Parent string
	Repo   scalar
	Tag    scalar
	// Line is where `image:` itself is, for a block that has no tag key to edit.
	Line int
}

func readValues(dir string) ([]imageBlock, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "values.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse values.yaml: %w", err)
	}
	root := mappingRoot(&doc)
	if root == nil {
		return nil, nil
	}

	var blocks []imageBlock
	add := func(parent string, m *yaml.Node) {
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			if k.Value == "image" && v.Kind == yaml.MappingNode {
				blocks = append(blocks, imageBlock{
					Parent: parent,
					Repo:   scalarAt(v, "repository"),
					Tag:    scalarAt(v, "tag"),
					Line:   k.Line,
				})
			}
		}
	}
	add("", root)
	for i := 0; i+1 < len(root.Content); i += 2 {
		if v := root.Content[i+1]; v.Kind == yaml.MappingNode {
			add(root.Content[i].Value, v)
		}
	}
	return blocks, nil
}

// chartPins turns a chart's dependencies and image blocks into pins.
func chartPins(rel string, c *chartFile, blocks []imageBlock) []Pin {
	pins := []Pin{}
	deps := map[string]dependency{}

	for _, d := range c.Dependencies {
		if d.Vendored != nil && d.Vendored.Library {
			continue
		}
		deps[d.Name.Value] = d
		p := Pin{
			Kind:      KindChart,
			Name:      d.Name.Value,
			ChartRepo: d.Repository,
			Version:   d.Version.Value,
			File:      rel + "/Chart.yaml",
			Line:      d.Version.Line,
			Field:     "dependencies[" + d.Name.Value + "].version",
		}
		if d.Vendored != nil {
			p.AppVersion = d.Vendored.AppVersion
			if d.Vendored.Version != d.Version.Value {
				p.Version, p.Declared = d.Vendored.Version, d.Version.Value
			}
		}
		pins = append(pins, p)
	}

	for _, b := range blocks {
		if b.Parent == "" {
			if b.Repo.Value == "" {
				continue
			}
			p := Pin{Kind: KindImage, Name: "image", Image: b.Repo.Value}
			if b.Tag.Value != "" {
				p.Version, p.File, p.Line, p.Field = b.Tag.Value, rel+"/values.yaml", b.Tag.Line, "image.tag"
			} else {
				// The common chart falls back to the chart's appVersion when the tag is empty.
				p.Version, p.File, p.Line, p.Field = c.AppVersion.Value, rel+"/Chart.yaml", c.AppVersion.Line, "appVersion"
			}
			pins = append(pins, p)
			continue
		}

		// Under a subchart's key with no tag, the subchart's own appVersion runs, and the chart pin covers it.
		if b.Tag.Value == "" {
			continue
		}
		repo := b.Repo.Value
		if d, ok := deps[b.Parent]; ok && repo == "" && d.Vendored != nil {
			repo = d.Vendored.ImageRepo
		}
		if repo == "" {
			continue
		}
		pins = append(pins, Pin{
			Kind:    KindImage,
			Name:    b.Parent + ".image",
			Image:   repo,
			Version: b.Tag.Value,
			File:    rel + "/values.yaml",
			Line:    b.Tag.Line,
			Field:   b.Parent + ".image.tag",
		})
	}
	return pins
}

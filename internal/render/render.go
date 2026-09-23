// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

// Package render renders an app the way Argo CD would and checks the result for rollouts that can hang.
package render

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/fireball1725/upstream/internal/helm"
	"github.com/fireball1725/upstream/internal/inventory"
)

// CheckRWORolling is the hygiene check this package adds.
const CheckRWORolling = "rwo-rolling-update"

// Manifests renders a chart with helm template, or concatenates the plain YAML of an app without one.
// Charts whose dependencies aren't vendored get `helm dependency update` first; the scan resets the clone after.
// Not `dependency build`, which needs every chart repo added with `helm repo add`.
func Manifests(ctx context.Context, dataDir, repoDir string, app inventory.App) (string, error) {
	dir := filepath.Join(repoDir, filepath.FromSlash(app.Dir))
	if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err == nil {
		args := []string{"template", app.Name, ".", "--namespace", app.Namespace}
		out, err := helm.Run(ctx, dataDir, dir, args...)
		if err != nil && strings.Contains(err.Error(), "missing in charts/ directory") {
			if _, err := helm.Run(ctx, dataDir, dir, "dependency", "update", "."); err != nil {
				return "", err
			}
			out, err = helm.Run(ctx, dataDir, dir, args...)
		}
		return out, err
	}

	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ext := filepath.Ext(path); d.IsDir() || (ext != ".yaml" && ext != ".yml") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.WriteString("\n---\n")
		b.Write(raw)
		return nil
	})
	return b.String(), err
}

type pvc struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		AccessModes []string `yaml:"accessModes"`
	} `yaml:"spec"`
}

type deployment struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Strategy struct {
			Type          string `yaml:"type"`
			RollingUpdate struct {
				MaxSurge any `yaml:"maxSurge"`
			} `yaml:"rollingUpdate"`
		} `yaml:"strategy"`
		Template struct {
			Spec struct {
				Volumes []struct {
					Name string `yaml:"name"`
					PVC  *struct {
						ClaimName string `yaml:"claimName"`
					} `yaml:"persistentVolumeClaim"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// RWORolling finds Deployments that roll pods (new one first) while mounting a ReadWriteOnce claim
// the same render creates. When the new pod lands on another node it can't attach the volume the
// old pod holds, and neither moves: homarr hung like this on 2026-09-23.
func RWORolling(manifests string) ([]inventory.Finding, error) {
	pvcs := map[string][]string{}
	var deps []deployment

	dec := yaml.NewDecoder(strings.NewReader(manifests))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read rendered YAML: %w", err)
		}
		var head struct {
			Kind string `yaml:"kind"`
		}
		if doc.Decode(&head) != nil {
			continue
		}
		switch head.Kind {
		case "PersistentVolumeClaim":
			var p pvc
			if doc.Decode(&p) == nil {
				pvcs[p.Metadata.Name] = p.Spec.AccessModes
			}
		case "Deployment":
			var d deployment
			if doc.Decode(&d) == nil {
				deps = append(deps, d)
			}
		}
	}

	out := []inventory.Finding{}
	for _, d := range deps {
		if d.Spec.Strategy.Type == "Recreate" || surgeIsZero(d.Spec.Strategy.RollingUpdate.MaxSurge) {
			continue
		}
		var claims []string
		for _, v := range d.Spec.Template.Spec.Volumes {
			if v.PVC == nil {
				continue
			}
			for _, mode := range pvcs[v.PVC.ClaimName] {
				if mode == "ReadWriteOnce" || mode == "ReadWriteOncePod" {
					claims = append(claims, v.PVC.ClaimName)
					break
				}
			}
		}
		if len(claims) == 0 {
			continue
		}
		sort.Strings(claims)
		out = append(out, inventory.Finding{
			Check:    CheckRWORolling,
			Severity: inventory.SeverityFix,
			Message: fmt.Sprintf("Deployment %s mounts the ReadWriteOnce volume %s and replaces pods with a rolling update. "+
				"When the new pod lands on another node it can't attach the volume the old pod holds, and the rollout hangs. "+
				"Set the strategy to Recreate.", d.Metadata.Name, strings.Join(claims, ", ")),
		})
	}
	return out, nil
}

// surgeIsZero is the one rolling update that stops the old pod before starting the new one.
func surgeIsZero(v any) bool {
	switch s := v.(type) {
	case int:
		return s == 0
	case string:
		return s == "0" || s == "0%"
	}
	return false
}

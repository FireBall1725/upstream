// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package render

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fireball1725/upstream/internal/helm"
	"github.com/fireball1725/upstream/internal/inventory"
)

const pvcRWO = `---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: homarr-database
spec:
  accessModes: ["ReadWriteOnce"]
`

const pvcRWX = `---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: shared-media
spec:
  accessModes:
    - ReadWriteMany
`

func deploy(strategy string) string {
	return `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: homarr
spec:
` + strategy + `
  template:
    spec:
      volumes:
        - name: db
          persistentVolumeClaim:
            claimName: homarr-database
        - name: media
          persistentVolumeClaim:
            claimName: shared-media
        - name: tmp
          emptyDir: {}
`
}

func TestRWORolling(t *testing.T) {
	tests := []struct {
		name      string
		manifests string
		want      int
	}{
		{"default strategy is a rolling update", pvcRWO + deploy(""), 1},
		{"explicit rolling update", pvcRWO + deploy("  strategy:\n    type: RollingUpdate"), 1},
		{"recreate is fine", pvcRWO + deploy("  strategy:\n    type: Recreate"), 0},
		{"maxSurge 0 stops the old pod first", pvcRWO + deploy("  strategy:\n    type: RollingUpdate\n    rollingUpdate:\n      maxSurge: 0\n      maxUnavailable: 1"), 0},
		{"maxSurge 0% too", pvcRWO + deploy("  strategy:\n    rollingUpdate:\n      maxSurge: \"0%\""), 0},
		{"maxSurge 1 still starts the new pod first", pvcRWO + deploy("  strategy:\n    rollingUpdate:\n      maxSurge: 1"), 1},
		{"ReadWriteMany is shared", pvcRWX + deploy(""), 0},
		{"a claim made outside the chart can't be judged", deploy(""), 0},
		{"a StatefulSet replaces pods in order", pvcRWO + "---\nkind: StatefulSet\nmetadata:\n  name: db\n", 0},
		{"empty and comment-only documents", "---\n# Source: x\n---\n" + pvcRWO + deploy(""), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RWORolling(tt.manifests)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tt.want, got)
			}
			if tt.want == 1 {
				f := got[0]
				if f.Check != CheckRWORolling || f.Severity != inventory.SeverityFix || !strings.Contains(f.Message, "homarr-database") || strings.Contains(f.Message, "shared-media") {
					t.Errorf("finding %+v", f)
				}
			}
		})
	}
}

func TestManifestsRendersAChart(t *testing.T) {
	if !helm.Available() {
		t.Skip("helm not installed")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "app-home", "dash")
	files := map[string]string{
		"Chart.yaml":             "apiVersion: v2\nname: dash\nversion: 1.0.0\n",
		"values.yaml":            "strategy: RollingUpdate\n",
		"templates/pvc.yaml":     pvcRWO,
		"templates/deploy.yaml":  deploy("  strategy:\n    type: {{ .Values.strategy }}"),
		"templates/_helpers.tpl": "",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := inventory.App{Namespace: "app-home", Name: "dash", Dir: "apps/app-home/dash"}
	out, err := Manifests(context.Background(), t.TempDir(), root, app)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RWORolling(out)
	if err != nil || len(got) != 1 {
		t.Fatalf("findings %+v %v\nrendered:\n%s", got, err, out)
	}
}

func TestManifestsReadsPlainYAML(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "app-ddns", "ddns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pvc.yaml"), []byte(pvcRWO), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(deploy("")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Manifests(context.Background(), t.TempDir(), root, inventory.App{Dir: "apps/app-ddns/ddns"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := RWORolling(out); len(got) != 1 {
		t.Errorf("findings %+v", got)
	}
}

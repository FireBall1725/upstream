// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 FireBall1725

package inventory

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var imageLine = regexp.MustCompile(`^\s*(?:-\s*)?image:\s*["']?([^"'\s#]+)`)

// readManifests finds `image:` lines in the plain YAML of an app that has no chart.
func readManifests(dir, rel string) ([]Pin, error) {
	pins := []Pin{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if d.IsDir() || (ext != ".yaml" && ext != ".yml") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		sub, _ := filepath.Rel(dir, path)
		sub = filepath.ToSlash(sub)
		n := 0
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for line := 1; sc.Scan(); line++ {
			m := imageLine.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			repo, tag := splitRef(m[1])
			n++
			// Field names each image by file and position, so two in one app stay distinct.
			field := sub + " image"
			if n > 1 {
				field = fmt.Sprintf("%s image[%d]", sub, n)
			}
			pins = append(pins, Pin{
				Kind:    KindImage,
				Name:    "image",
				Image:   repo,
				Version: tag,
				File:    rel + "/" + sub,
				Line:    line,
				Field:   field,
			})
		}
		return sc.Err()
	})
	return pins, err
}

// splitRef separates "ghcr.io/a/b:1.2" into repo and tag, leaving a registry port alone.
func splitRef(ref string) (repo, tag string) {
	if repo, digest, ok := strings.Cut(ref, "@"); ok {
		return repo, digest
	}
	slash := strings.LastIndex(ref, "/")
	if colon := strings.LastIndex(ref, ":"); colon > slash {
		return ref[:colon], ref[colon+1:]
	}
	return ref, ""
}

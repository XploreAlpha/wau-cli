// Package manifest — pack.go
//
// Pack a directory containing manifest.yaml + skill files into a
// deterministic tar.gz suitable for upload to wau-registry.
//
// Deterministic ordering: file paths are sorted alphabetically so the
// resulting tarball hash is stable across runs (helps with deduplication
// and reproducibility).
//
// The tarball layout keeps the top-level directory name (e.g. "weather-bot")
// so extract-side can re-root the bundle cleanly.
package manifest

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PackDir tars + gzips srcDir into a tarball at dstPath.
//
// srcDir must contain manifest.yaml. The top-level directory name in the
// resulting tarball equals filepath.Base(srcDir).
func PackDir(srcDir, dstPath string) (*Manifest, error) {
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("abs src: %w", err)
	}
	m, err := LoadFromDir(absSrc)
	if err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}

	// Confirm entrypoint exists relative to srcDir.
	epFull := filepath.Join(absSrc, m.Entrypoint)
	if _, err := os.Stat(epFull); err != nil {
		return nil, fmt.Errorf("entrypoint %q not found in %s: %w",
			m.Entrypoint, absSrc, err)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("create tarball %s: %w", dstPath, err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := writeTar(tw, absSrc, filepath.Base(absSrc)); err != nil {
		return nil, fmt.Errorf("write tarball: %w", err)
	}
	return m, nil
}

// writeTar walks srcDir and writes each file/dir into tw with the given
// prefix as the top-level directory name. Paths use forward slashes
// regardless of OS (tar convention).
func writeTar(tw *tar.Writer, srcDir, topName string) error {
	type entry struct {
		absPath string
		relPath string
		info    os.FileInfo
	}
	var entries []entry

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for tar convention.
		rel = strings.ReplaceAll(rel, string(os.PathSeparator), "/")
		entries = append(entries, entry{
			absPath: path,
			relPath: rel,
			info:    info,
		})
		return nil
	})
	if err != nil {
		return err
	}

	// Sort entries by relPath for deterministic order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	for _, e := range entries {
		// Tar header path: topName + "/" + relPath
		var name string
		if e.relPath == "." {
			name = topName
		} else {
			name = topName + "/" + e.relPath
		}

		hdr, err := tar.FileInfoHeader(e.info, "")
		if err != nil {
			return fmt.Errorf("header for %s: %w", e.relPath, err)
		}
		hdr.Name = name
		// Stable mode (no exec bits by default for safety).
		if e.info.IsDir() {
			hdr.Mode = 0o755
		} else {
			hdr.Mode = 0o644
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %s: %w", e.relPath, err)
		}
		if e.info.IsDir() {
			continue
		}

		f, err := os.Open(e.absPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", e.relPath, err)
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return fmt.Errorf("copy %s: %w", e.relPath, err)
		}
		f.Close()
	}
	return nil
}

// BundleFileName returns the conventional tarball filename for a manifest.
//
// Format: {name}-{version}.tar.gz
func BundleFileName(m *Manifest) string {
	return fmt.Sprintf("%s-%s.tar.gz", m.Name, m.Version)
}
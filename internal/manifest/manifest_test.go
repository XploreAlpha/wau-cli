package manifest

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a tiny helper to create a file under root with given content.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestLoad_ValidManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "manifest.yaml", `name: weather-bot
version: 0.1.0
entrypoint: skills/weather/main.py
description: 天气查询机器人
universes: [default]
skills: [weather]
`)
	m, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Name != "weather-bot" {
		t.Errorf("Name=%q", m.Name)
	}
	if m.Version != "0.1.0" {
		t.Errorf("Version=%q", m.Version)
	}
	if m.Entrypoint != "skills/weather/main.py" {
		t.Errorf("Entrypoint=%q", m.Entrypoint)
	}
	if m.Universe != "default" {
		t.Errorf("Universe=%q (expected default)", m.Universe)
	}
	if len(m.Universes) != 1 || m.Universes[0] != "default" {
		t.Errorf("Universes=%v", m.Universes)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "manifest.yaml", `name: minimal-agent
entrypoint: main.py
`)
	m, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Version != DefaultVersion {
		t.Errorf("Version default=%q, want %q", m.Version, DefaultVersion)
	}
	if m.Universe != DefaultUniverse {
		t.Errorf("Universe default=%q, want %q", m.Universe, DefaultUniverse)
	}
}

func TestValidate_BadName(t *testing.T) {
	cases := []struct {
		name string
		want string // substring of expected error
	}{
		{"Weather-Bot", "name"},       // uppercase
		{"123-starts-with-digit", ""}, // starts with digit
		{"", "name"},
		{"a/b", "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Name: tc.name, Entrypoint: "main.py", Version: "0.1.0"}
			err := m.Validate()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err=%q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidate_BadVersion(t *testing.T) {
	m := &Manifest{Name: "ok", Entrypoint: "main.py", Version: "v1.0"}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "semver") {
		t.Fatalf("expected semver error, got %v", err)
	}
}

func TestValidate_AbsoluteEntrypoint(t *testing.T) {
	m := &Manifest{Name: "ok", Entrypoint: "/etc/passwd", Version: "0.1.0"}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("expected relative error, got %v", err)
	}
}

func TestPackDir_RoundTrip(t *testing.T) {
	// t.TempDir basename is random; rename to fixed name for stable tarball.
	parent := t.TempDir()
	dir := filepath.Join(parent, "test-bundle")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, dir, "manifest.yaml", `name: test-bundle
version: 1.2.3
entrypoint: skills/test/main.py
`)
	writeFile(t, dir, "skills/test/main.py", "# test skill\n")
	writeFile(t, dir, "README.md", "hello\n")

	tarball := filepath.Join(t.TempDir(), "out.tar.gz")
	m, err := PackDir(dir, tarball)
	if err != nil {
		t.Fatalf("PackDir: %v", err)
	}
	if m.Name != "test-bundle" {
		t.Errorf("manifest name=%q", m.Name)
	}

	// Inspect tarball contents.
	f, err := os.Open(tarball)
	if err != nil {
		t.Fatalf("open tarball: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	got := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Typeflag == tar.TypeReg {
			buf := make([]byte, hdr.Size)
			if _, err := io.ReadFull(tr, buf); err != nil {
				t.Fatalf("read %s: %v", hdr.Name, err)
			}
			got[hdr.Name] = string(buf)
		}
	}

	expected := []string{
		"test-bundle/manifest.yaml",
		"test-bundle/skills/test/main.py",
		"test-bundle/README.md",
	}
	for _, want := range expected {
		if _, ok := got[want]; !ok {
			t.Errorf("missing entry in tarball: %s (got=%v)", want, got)
		}
	}
}

func TestPackDir_MissingEntrypoint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "manifest.yaml", `name: bad
entrypoint: nonexistent/main.py
`)
	tarball := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := PackDir(dir, tarball); err == nil {
		t.Fatal("expected entrypoint-not-found error")
	}
}

func TestBundleFileName(t *testing.T) {
	m := &Manifest{Name: "weather-bot", Version: "0.1.0"}
	if got := BundleFileName(m); got != "weather-bot-0.1.0.tar.gz" {
		t.Errorf("BundleFileName=%q", got)
	}
}
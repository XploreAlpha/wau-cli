package agent

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wau/wau-cli/internal/manifest"
)

func TestSwapPort(t *testing.T) {
	cases := []struct {
		in, wantPort string
	}{
		{"http://localhost:18400", "18401"},
		{"http://staging.example.com:18400", "18401"},
		{"http://no-port", "9000"},
	}
	for _, tc := range cases {
		got := swapPort(tc.in, tc.wantPort)
		if !strings.HasSuffix(got, ":"+tc.wantPort) {
			t.Errorf("swapPort(%q, %q) = %q", tc.in, tc.wantPort, got)
		}
	}
}

func TestHasPort(t *testing.T) {
	if !hasPort("http://localhost:18400", "18400") {
		t.Error("expected true for matching port")
	}
	if hasPort("http://localhost:18400", "18401") {
		t.Error("expected false for non-matching port")
	}
	if hasPort("http://no-port", "18400") {
		t.Error("expected false when no port present")
	}
}

// TestPublishToRegistry_Multipart verifies multipart payload shape:
// - Content-Type: multipart/form-data with boundary
// - manifest field: JSON of manifest
// - bundle field: raw tarball bytes
func TestPublishToRegistry_Multipart(t *testing.T) {
	var gotContentType string
	var gotManifest string
	var gotBundle []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		files := r.MultipartForm.File["manifest"]
		if len(files) != 1 {
			http.Error(w, "want 1 manifest part", 400)
			return
		}
		f, _ := files[0].Open()
		buf, _ := io.ReadAll(f)
		gotManifest = string(buf)
		f.Close()

		bundles := r.MultipartForm.File["bundle"]
		if len(bundles) != 1 {
			http.Error(w, "want 1 bundle part", 400)
			return
		}
		bf, _ := bundles[0].Open()
		gotBundle, _ = io.ReadAll(bf)
		bf.Close()

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"test","version":"0.1.0"}`))
	}))
	defer ts.Close()

	// Build a dummy tarball (just bytes — server doesn't parse).
	dir := t.TempDir()
	tarball := filepath.Join(dir, "x.tar.gz")
	if err := os.WriteFile(tarball, []byte("fake-tarball-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &manifest.Manifest{
		Name:       "test-bundle",
		Version:    "1.0.0",
		Entrypoint: "main.py",
		Universes:  []string{"default"},
	}

	if err := publishToRegistry(context.Background(), ts.URL, tarball, m); err != nil {
		t.Fatalf("publishToRegistry: %v", err)
	}

	// Verify Content-Type.
	mediaType, params, err := mime.ParseMediaType(gotContentType)
	if err != nil {
		t.Fatalf("parse Content-Type %q: %v", gotContentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Errorf("Content-Type media=%q, want multipart/form-data", mediaType)
	}
	if params["boundary"] == "" {
		t.Error("missing multipart boundary")
	}

	// Verify manifest JSON round-tripped.
	var gotM manifest.Manifest
	if err := json.Unmarshal([]byte(gotManifest), &gotM); err != nil {
		t.Fatalf("manifest not valid JSON: %v body=%q", err, gotManifest)
	}
	if gotM.Name != "test-bundle" {
		t.Errorf("manifest.Name=%q, want test-bundle", gotM.Name)
	}

	// Verify bundle bytes round-tripped.
	wantBundle, _ := os.ReadFile(tarball)
	if string(gotBundle) != string(wantBundle) {
		t.Errorf("bundle mismatch: got=%q want=%q", gotBundle, wantBundle)
	}
}

func TestPublishToRegistry_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "validation failed", 400)
	}))
	defer ts.Close()

	dir := t.TempDir()
	tarball := filepath.Join(dir, "x.tar.gz")
	if err := os.WriteFile(tarball, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := publishToRegistry(context.Background(), ts.URL, tarball, &manifest.Manifest{
		Name: "x", Version: "0.1.0", Entrypoint: "main.py",
	})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected 400 error, got %v", err)
	}
}

func TestPublishToRegistry_BadTarballPath(t *testing.T) {
	m := &manifest.Manifest{Name: "x", Version: "0.1.0", Entrypoint: "main.py"}
	err := publishToRegistry(context.Background(), "http://localhost:1", "/nonexistent/path.tar.gz", m)
	if err == nil {
		t.Fatal("expected error for nonexistent tarball")
	}
}
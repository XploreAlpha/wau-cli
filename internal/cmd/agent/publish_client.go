package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/wau/wau-cli/internal/manifest"
)

// publishToRegistry POSTs the bundle to /registry/skills/publish on the
// given wau-registry base URL (e.g. http://localhost:18401).
//
// Wire format: multipart/form-data with two fields:
//   - manifest: JSON-encoded manifest.Manifest
//   - bundle:   raw tar.gz bytes
//
// Response shape: 201 Created with body { name, version, entrypoint, ... }.
func publishToRegistry(ctx context.Context, registry, tarball string, m *manifest.Manifest) error {
	tarData, err := readFile(tarball)
	if err != nil {
		return fmt.Errorf("read tarball: %w", err)
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)

	// Field 1: manifest as JSON.
	mfPart, err := mw.CreateFormFile("manifest", "manifest.json")
	if err != nil {
		return fmt.Errorf("create manifest part: %w", err)
	}
	mJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if _, err := mfPart.Write(mJSON); err != nil {
		return fmt.Errorf("write manifest part: %w", err)
	}

	// Field 2: tarball.
	bPart, err := mw.CreateFormFile("bundle", "bundle.tar.gz")
	if err != nil {
		return fmt.Errorf("create bundle part: %w", err)
	}
	if _, err := bPart.Write(tarData); err != nil {
		return fmt.Errorf("write bundle part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart: %w", err)
	}

	url := strings.TrimRight(registry, "/") + "/registry/skills/publish"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Agent-Role", "external_agent")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("registry returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// readFile is a small wrapper to make the imports above cleaner.
func readFile(path string) ([]byte, error) {
	return readWholeFile(path)
}

// swapPort / hasPort helpers.
func swapPort(addr, newPort string) string {
	// Replace :<digits> at end with new port.
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr + ":" + newPort
	}
	return addr[:i+1] + newPort
}

func hasPort(addr, port string) bool {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return false
	}
	return addr[i+1:] == port
}
package agent

import "os"

// readWholeFile reads a file in full. Kept as a separate function so it
// can be mocked or replaced in tests (and to avoid pulling in the
// "os" import directly from publish_client.go).
func readWholeFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
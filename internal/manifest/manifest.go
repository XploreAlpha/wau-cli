// Package manifest provides parsing and validation of WAU agent manifests.
//
// Manifest format follows agentskills.io v1 spec (D69=A) plus WAU-specific
// extensions (universes field, is_builtin flag, etc.).
//
// Typical layout:
//
//	weather-bot/
//	├── manifest.yaml       # this package parses it
//	├── skills/weather/main.py
//	└── README.md
//
// Reference: memory/project-v1-0-0-M11-wau-agent-2026-07-04.md §"B 端 SDK manifest"
package manifest

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest is the parsed contents of a WAU agent manifest.yaml.
//
// JSON tags are present for upload to wau-registry; YAML tags for parsing.
type Manifest struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string            `json:"version" yaml:"version"`
	Author      string            `json:"author,omitempty" yaml:"author,omitempty"`
	Universe    string            `json:"universe,omitempty" yaml:"universe,omitempty"`
	Universes   []string          `json:"universes,omitempty" yaml:"universes,omitempty"`
	Entrypoint  string            `json:"entrypoint" yaml:"entrypoint"`
	Parameters  map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Skills      []string          `json:"skills,omitempty" yaml:"skills,omitempty"`
	SourceURL   string            `json:"source_url,omitempty" yaml:"source_url,omitempty"`
}

// nameRe matches the agentskills.io name rule: lowercase letters, digits,
// dash, underscore. Must start with a letter. Max 64 chars total.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// semverRe matches MAJOR.MINOR.PATCH (per agentskills.io v1).
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// DefaultVersion is used when manifest.yaml omits the version field.
const DefaultVersion = "0.1.0"

// DefaultUniverse is used when manifest.yaml omits both universe/universes.
const DefaultUniverse = "default"

// ManifestFilename is the conventional manifest file name (D69=A).
const ManifestFilename = "manifest.yaml"

// Load reads and parses a manifest file at the given path.
//
// It does NOT validate entrypoint existence — call Validate for that.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	m.applyDefaults()
	return &m, nil
}

// LoadFromDir looks for manifest.yaml in dir and parses it.
func LoadFromDir(dir string) (*Manifest, error) {
	return Load(dir + "/" + ManifestFilename)
}

// applyDefaults sets zero-value fields to safe defaults.
func (m *Manifest) applyDefaults() {
	if m.Version == "" {
		m.Version = DefaultVersion
	}
	if m.Universe == "" && len(m.Universes) == 0 {
		m.Universe = DefaultUniverse
	}
	if m.Universe != "" && len(m.Universes) == 0 {
		m.Universes = []string{m.Universe}
	}
	if len(m.Universes) > 0 && m.Universe == "" {
		m.Universe = m.Universes[0]
	}
}

// Validate checks that the manifest meets agentskills.io v1 + WAU rules.
// It does NOT check that Entrypoint points to an existing file — that
// requires the surrounding directory and is the caller's responsibility.
func (m *Manifest) Validate() error {
	var errs []string

	if !nameRe.MatchString(m.Name) {
		errs = append(errs, fmt.Sprintf(
			"name %q must match ^[a-z][a-z0-9_-]{0,63}$", m.Name))
	}
	if m.Version != "" && !semverRe.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf(
			"version %q must be semver MAJOR.MINOR.PATCH", m.Version))
	}
	if strings.TrimSpace(m.Entrypoint) == "" {
		errs = append(errs, "entrypoint is required")
	}
	if strings.HasPrefix(m.Entrypoint, "/") {
		errs = append(errs, "entrypoint must be relative (no leading /)")
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.New("manifest validation failed: " + strings.Join(errs, "; "))
}
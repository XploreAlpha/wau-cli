package version

import (
	"strings"
	"testing"
)

// TestVersion_NonEmpty ensures Version is set (guard against accidental
// empty-string bumps).
func TestVersion_NonEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must be non-empty")
	}
}

// TestVersion_PrefixV ensures Version follows SemVer v-prefix convention
// (matches `git tag v1.3.4` lookup pattern).
func TestVersion_PrefixV(t *testing.T) {
	if !strings.HasPrefix(Version, "v") {
		t.Errorf("Version = %q, want 'v' prefix (SemVer convention)", Version)
	}
}

// TestVersion_AlignedV134 locks the alignment target. Per D92 / sub-item
// 4.2, all 14 server 仓 + wau-cli share this value to avoid SDK silent
// failures (per homerail Plan C v1.4-academic lesson).
func TestVersion_AlignedV134(t *testing.T) {
	if Version != "v1.3.4" {
		t.Errorf("Version = %q, want %q (sub-item 4.2 alignment target)", Version, "v1.3.4")
	}
}

// TestReleaseName_NonEmpty ensures ReleaseName is set.
func TestReleaseName_NonEmpty(t *testing.T) {
	if ReleaseName == "" {
		t.Fatal("ReleaseName must be non-empty")
	}
}

// TestReleaseName_Jade locks the codename matching v1.3.4 alignment.
// Codename convention is in CHANGELOG.md table; update in lockstep.
func TestReleaseName_Jade(t *testing.T) {
	if ReleaseName != "Jade" {
		t.Errorf("ReleaseName = %q, want %q (v1.3.4 codename)", ReleaseName, "Jade")
	}
}
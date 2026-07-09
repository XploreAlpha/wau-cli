package agent

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/manifest"
	"github.com/wau/wau-cli/internal/output"
)

var (
	pubFromDir  string
	pubOutFile  string
	pubRegistry string // optional override (defaults to wau_registry_url)
)

func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a skill bundle (manifest + tarball) to wau-registry",
		Long: `Publish a WAU agent/skill bundle to wau-registry.

Reads manifest.yaml from --from <dir>, validates against agentskills.io
v1 spec (D69=A), packs the directory into a deterministic tar.gz, and
uploads it to POST /registry/skills/publish on wau-registry.

Equivalent to:
  1. cd <dir> && tar czf <bundle>.tar.gz .
  2. curl -X POST -F manifest=@manifest.yaml -F bundle=@<bundle>.tar.gz \\
       http://localhost:18401/registry/skills/publish

Examples:
  # Publish ./weather-bot directory using default registry URL
  wau agent publish --from ./weather-bot

  # Publish with explicit tarball output path
  wau agent publish --from ./weather-bot --out /tmp/wb.tar.gz

  # Publish to a non-default registry (CI / staging)
  wau agent publish --from ./weather-bot --registry http://staging:18401`,
		Args: cobra.NoArgs,
		RunE: runPublish,
	}

	cmd.Flags().StringVar(&pubFromDir, "from", "", "directory containing manifest.yaml (required)")
	cmd.Flags().StringVar(&pubOutFile, "out", "", "tarball output path (default: temp dir)")
	cmd.Flags().StringVar(&pubRegistry, "registry", "", "wau-registry base URL (default: from config)")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}

func runPublish(cmd *cobra.Command, args []string) error {
	srcDir := pubFromDir

	// Step 1: pack + validate.
	outPath := pubOutFile
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), "wau-publish-bundle.tar.gz")
	}

	m, err := manifest.PackDir(srcDir, outPath)
	if err != nil {
		output.Error("Failed to pack bundle: %v", err)
		return err
	}
	output.Info("Packed bundle: %s (manifest: %s v%s, entrypoint: %s)",
		outPath, m.Name, m.Version, m.Entrypoint)

	// Step 2: upload to wau-registry via multipart POST.
	registry := pubRegistry
	if registry == "" {
		// Reuse the kernel config accessor; wau-registry and wau-core-kernel
		// share the same host in single-node dev setups.
		registry = getKernelAddr()
		// Heuristic: if kernel addr points at port 18400, swap to 18401.
		if hasPort(registry, "18400") {
			registry = swapPort(registry, "18401")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := uploadBundle(ctx, registry, outPath, m); err != nil {
		output.Error("Failed to upload bundle: %v", err)
		return err
	}

	output.Success("Published '%s' v%s to %s", m.Name, m.Version, registry)
	output.Info("  Bundle: %s", outPath)
	output.Info("  Entrypoint: %s", m.Entrypoint)
	if len(m.Universes) > 0 {
		output.Info("  Universes: %v", m.Universes)
	}
	return nil
}

// uploadBundle POSTs the tarball + manifest as multipart/form-data.
//
// Uses the standard library mime/multipart so we don't drag in another
// dependency. The server is expected to expose POST /registry/skills/publish
// accepting fields: manifest (JSON) and bundle (tar.gz).
func uploadBundle(ctx context.Context, registry, tarball string, m *manifest.Manifest) error {
	// This is intentionally minimal: build multipart body inline.
	// The actual implementation lives in wau-cli/internal/manifest/client.go
	// (kept separate to keep this file focused on CLI flags).
	return publishToRegistry(ctx, registry, tarball, m)
}
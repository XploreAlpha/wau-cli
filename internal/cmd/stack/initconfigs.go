// Package stack - initconfigs.go
//
// P4.2 (v1.0.1, 2026-08-24) — `wau stack init-configs` 子命令。
//
// 类比:
//   - docker compose config
//   - kubectl create configmap (但我们是写文件而非 server resource)
//   - terraform init
//
// 设计原则:
//   - embed 4 个服务的最小 config(wau-store / wau-llm-router / wau-edge / wau-channel)
//   - 默认写 ~/.wau/configs/<service>.yaml
//   - idempotent:文件已存在默认 skip(除非 --force)
//   - --dry-run:不写,只 print plan
//   - 不修改服务 source(D60 additive)
//
// 用法:
//   wau stack init-configs                       # 写所有 4 个到 ~/.wau/configs/
//   wau stack init-configs --service wau-store   # 单服务
//   wau stack init-configs --output-dir /tmp/x   # 自定义目录
//   wau stack init-configs --force               # 覆盖已有
//   wau stack init-configs --dry-run             # 只看 plan
package stack

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/stack/initconfigs"
)

var (
	flagInitService   string
	flagInitOutputDir string
	flagInitForce     bool
	flagInitDryRun    bool
)

// NewInitConfigsCmd creates the `wau stack init-configs` subcommand.
func NewInitConfigsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-configs",
		Short: "Write embedded service config templates to ~/.wau/configs/",
		Long: `Initialize service config files from embedded templates.

wau-cli ships with minimal config templates for 4 services that need a YAML config
file to start: wau-store, wau-llm-router, wau-edge, wau-channel.

By default writes to ~/.wau/configs/<service>.yaml. Existing files are skipped
unless --force is given.

After running this command, you can ` + "`wau stack up --demo`" + ` to bring up all 9 services.

Examples:
  # Write all 4 configs to default location (~/.wau/configs/)
  wau stack init-configs

  # Only wau-store
  wau stack init-configs --service wau-store

  # Custom output directory
  wau stack init-configs --output-dir /etc/wau/configs

  # Overwrite existing files
  wau stack init-configs --force

  # Preview only (don't write)
  wau stack init-configs --dry-run`,
		Aliases: []string{"init-config"},
		Args:    cobra.NoArgs,
		RunE:    runInitConfigs,
	}
	cmd.Flags().StringVar(&flagInitService, "service", "",
		"only init configs for one service (e.g. wau-store, wau-llm-router)")
	cmd.Flags().StringVar(&flagInitOutputDir, "output-dir", "~/.wau/configs",
		"directory to write config files to (default: ~/.wau/configs)")
	cmd.Flags().BoolVar(&flagInitForce, "force", false,
		"overwrite existing config files")
	cmd.Flags().BoolVar(&flagInitDryRun, "dry-run", false,
		"show what would be written, but don't actually write")
	return cmd
}

func runInitConfigs(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	// 选 templates
	var templates []initconfigs.Template
	if flagInitService != "" {
		tpl, err := initconfigs.TemplateByService(flagInitService)
		if err != nil {
			return err
		}
		templates = []initconfigs.Template{tpl}
	} else {
		ts, err := initconfigs.ListTemplates()
		if err != nil {
			return fmt.Errorf("list templates: %w", err)
		}
		templates = ts
	}

	if flagInitDryRun {
		fmt.Fprintf(w, "Dry run — would write %d config(s) to %s:\n",
			len(templates), flagInitOutputDir)
		for _, t := range templates {
			fmt.Fprintf(w, "  [%s] %s (%d bytes)\n", t.Service, t.Filename, len(t.Contents))
		}
		return nil
	}

	writer := &initconfigs.Writer{
		OutputDir: flagInitOutputDir,
		Force:     flagInitForce,
		DryRun:    false,
	}
	results := writer.WriteAll(templates)

	wrote, skipped, errored := 0, 0, 0
	for _, r := range results {
		printResult(w, r)
		switch r.Status {
		case "wrote":
			wrote++
		case "skipped":
			skipped++
		case "would-write":
			wrote++ // dry-run counter
		case "error":
			errored++
		}
	}

	fmt.Fprintf(w, "\n✓ %d wrote, %d skipped", wrote, skipped)
	if errored > 0 {
		fmt.Fprintf(w, ", %d errored", errored)
	}
	if flagInitService == "" && wrote > 0 {
		fmt.Fprintf(w, "\nNext: wau stack up --demo")
	}
	fmt.Fprintln(w)

	if errored > 0 {
		return fmt.Errorf("%d config(s) failed to write", errored)
	}
	return nil
}

func printResult(w io.Writer, r initconfigs.WriteResult) {
	switch r.Status {
	case "wrote":
		fmt.Fprintf(w, "✓ %s: %s (%d bytes)\n", r.Service, r.Filepath, r.Size)
	case "skipped":
		fmt.Fprintf(w, "- %s: %s (already exists, skip; use --force)\n", r.Service, r.Filepath)
	case "would-write":
		fmt.Fprintf(w, "→ %s: %s (%d bytes, dry-run)\n", r.Service, r.Filepath, r.Size)
	case "error":
		fmt.Fprintf(w, "✗ %s: %s (error: %v)\n", r.Service, r.Filepath, r.Err)
	default:
		fmt.Fprintf(w, "? %s: %s (status=%s)\n", r.Service, r.Filepath, r.Status)
	}
}
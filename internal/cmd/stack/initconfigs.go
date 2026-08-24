// Package stack - initconfigs.go
//
// P4.2 + P4.5 (v1.0.1, 2026-08-24) — `wau stack init-configs` 子命令。
//
// P4.5 加 --envsubst:写 yaml 前用 os.ExpandEnv 替换 $VAR 占位符。
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
//   - --envsubst:写文件前用 os.ExpandEnv 替换 $VAR(per P4.5)
//   - 不修改服务 source(D60 additive)
//
// 用法:
//   wau stack init-configs                          # 写所有 4 个到 ~/.wau/configs/(保留 $VAR)
//   wau stack init-configs --envsubst --force      # 用 env var 替换占位符(visa demo)
//   wau stack init-configs --service wau-store     # 单服务
//   wau stack init-configs --output-dir /tmp/x     # 自定义目录
//   wau stack init-configs --force                 # 覆盖已有
//   wau stack init-configs --dry-run               # 只看 plan
package stack

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/wau/wau-cli/internal/stack/initconfigs"
)

var (
	flagInitService   string
	flagInitOutputDir string
	flagInitForce     bool
	flagInitDryRun    bool
	flagInitEnvSubst  bool // P4.5
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

Templates contain $VAR placeholders for secrets (PG DSN, Redis password, admin
token). Use --envsubst to substitute them from your environment before writing
(useful for visa demo / local dev); production deployments should leave them
literal so the deploy script (wau-deploy) can substitute.

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
  wau stack init-configs --dry-run

  # P4.5: replace $VAR with env values (local visa demo)
  export WAU_STORE_PG_DSN="postgres://x:y@localhost/wau_store"
  wau stack init-configs --envsubst --force`,
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
	cmd.Flags().BoolVar(&flagInitEnvSubst, "envsubst", false,
		"P4.5: substitute $VAR placeholders from env before writing (use for local dev/visa demo)")
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
		// P4.5 dry-run 也提示 $VAR 占位符
		if flagInitEnvSubst {
			fmt.Fprintln(w, "\nWith --envsubst, $VAR placeholders will be expanded from env.")
			allVars := map[string]bool{}
			for _, t := range templates {
				for _, v := range initconfigs.ExtractEnvVars(t.Contents) {
					allVars[v] = true
				}
			}
			if len(allVars) > 0 {
				fmt.Fprint(w, "Variables referenced: ")
				first := true
				for v := range allVars {
					if !first {
						fmt.Fprint(w, ", ")
					}
					first = false
					set := os.Getenv(v) != ""
					marker := "✓"
					if !set {
						marker = "✗" // 未 set → 替换成 ""
					}
					fmt.Fprintf(w, "%s$%s", marker, v)
				}
				fmt.Fprintln(w)
			}
		}
		return nil
	}

	writer := &initconfigs.Writer{
		OutputDir: flagInitOutputDir,
		Force:     flagInitForce,
		DryRun:    false,
		EnvSubst:  flagInitEnvSubst,
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

	// P4.5 envsubst warning:写完后检查哪些 template 里的 $VAR 对应**空** env var
	// (os.ExpandEnv 把空 env 替换成 "",所以写完后文件里没有 $VAR 字面值,只能从 template 反推)
	unsubstituted := 0
	if flagInitEnvSubst && wrote > 0 {
		for _, r := range results {
			if r.Status != "wrote" {
				continue
			}
			// 从原 template 拿 var 列表(而不是从写完文件读)
			var tplVars []string
			for _, t := range templates {
				if t.Service == r.Service {
					tplVars = initconfigs.ExtractEnvVars(t.Contents)
					break
				}
			}
			emptyVars := []string{}
			for _, v := range tplVars {
				if os.Getenv(v) == "" {
					emptyVars = append(emptyVars, v)
				}
			}
			if len(emptyVars) > 0 {
				unsubstituted++
				fmt.Fprintf(w, "⚠ %s: %d env var(s) were empty (will be replaced with \"\"): %v\n",
					r.Service, len(emptyVars), emptyVars)
			}
		}
	}

	if flagInitService == "" && wrote > 0 {
		fmt.Fprintf(w, "\nNext: wau stack up --demo")
	}
	fmt.Fprintln(w)

	if errored > 0 {
		return exitCodeError(1)
	}
	if unsubstituted > 0 {
		// exit code 2 — 警告级(进程可能能起但密码/token 是空)
		return exitCodeError(2)
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
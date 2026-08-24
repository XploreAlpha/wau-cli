// Package defaults — 4.1.2 (2026-08-24, v1.1.0 子项 4.1) — 内嵌 default wau-stack.yml。
//
// 跟 v1.0.1 的 internal/stack/default.go 镜像,迁到 v1.1 schema:
//   - 老 9-service programmatic default 保留(0 改,D60)
//   - 新 10-service YAML embed 提供给 --file 路径 / ParseV11 schema 校验
//
// 用 `//go:embed` 在编译期把 wau-stack.yml 注入 binary,运行时 DefaultStackYAMLBytes()
// 返回字节流 → ParseV11 解析 → 跟用户写的 wau-stack.yml 走完全相同的解析路径,
// 0 字节 hardcode(单一 schema 真源)。
package defaults

import (
	_ "embed"
)

//go:embed wau-stack.yml
var defaultStackYAML []byte

// DefaultStackYAMLBytes 返回内嵌 default wau-stack.yml v1.1 字节流。
//
// 调用方通常: stack, _ := stack.ParseV11(defaults.DefaultStackYAMLBytes())
func DefaultStackYAMLBytes() []byte {
	// 防御性 copy — caller 万一 mutate 不影响下次 embed 调用
	out := make([]byte, len(defaultStackYAML))
	copy(out, defaultStackYAML)
	return out
}
// Package version declares the canonical wau version constants.
//
// Per D92 (v1.1.0 子项 4.1 plan) — all 14 server 仓 + 5 SDK align at v1.3.4
// so cross-repo references (e.g. wau-stack.yml release pin) resolve to a
// single version. Bumping this const propagates to the runtime `--version`
// output and is the authoritative source for `git tag v1.3.4` matching.
//
// 4.1.0 (2026-08-24, 子项 4.2 version alignment kickoff).
package version

// Version is the wau release version (SemVer). Bump in lockstep across all
// 14 server 仓 + wau-cli per sub-item 4.2 alignment.
//
// v1.0.1 (Iris) → v1.3.4 (Jade) — skipped v1.1.0 Granite / v1.2.0 to align
// with the 5 SDK which already sit at v1.3.4 (per memory
// project-wau-v1-1-0-deployment-plan-main-2026-08-19 §四).
const Version = "v1.3.4"

// ReleaseName is the codename for this version. Per convention, codenames
// follow natural/mineral themes inspired by HashiCorp's naming style
// (Genesis → Coral → Dolphin → Emerald → Falcon → Phoenix → Granite →
// Iris → Jade).
const ReleaseName = "Jade"
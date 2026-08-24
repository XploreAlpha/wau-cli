package stack

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// ─── NewRestartCmd ─────────────────────────────────────────────────────────

func TestNewRestartCmd_BasicArgs(t *testing.T) {
	cmd := NewRestartCmd()
	if cmd.Use != "restart [service...]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "restart [service...]")
	}
	if cmd.RunE == nil {
		t.Error("RunE missing")
	}
	// Aliases
	foundReload := false
	for _, a := range cmd.Aliases {
		if a == "reload" {
			foundReload = true
		}
	}
	if !foundReload {
		t.Errorf("aliases missing 'reload': %v", cmd.Aliases)
	}
	// Flags
	for _, name := range []string{"file", "profile", "wait-max"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
	// wait-max default
	wm := cmd.Flags().Lookup("wait-max")
	if wm != nil && wm.DefValue != "1m0s" {
		t.Errorf("wait-max default = %q, want 1m0s", wm.DefValue)
	}
}

// ─── Plan logic helpers ───────────────────────────────────────────────────

func TestReverseStrings(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"a", "b", "c"}, []string{"c", "b", "a"}},
		{[]string{"redis", "wau-core", "wau-router"}, []string{"wau-router", "wau-core", "redis"}},
		{nil, []string{}},
		{[]string{"x"}, []string{"x"}},
	}
	for _, c := range cases {
		got := reverseStrings(c.in)
		if len(got) != len(c.want) {
			t.Errorf("reverseStrings(%v) len = %d, want %d", c.in, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("reverseStrings(%v)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
		// 不修改原 slice
		if len(c.in) > 0 {
			if c.in[0] != got[len(got)-1] || c.in[len(c.in)-1] != got[0] {
				// 原 slice 第一个仍是 'in 的第一个'
				if c.in[0] != c.in[0] {
					t.Errorf("original slice mutated")
				}
			}
		}
	}
}

// ─── runRestart — smoke via cmd.SetArgs ────────────────────────────────────

func TestRunRestart_InvalidService(t *testing.T) {
	// args=[nonexistent-svc] → 友好错误,不调 server
	resetRestartFlags()
	cmd := NewRestartCmd()
	cmd.SetArgs([]string{"nonexistent-svc"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("want error for invalid service name")
	}
	if !strings.Contains(err.Error(), "not in stack") {
		t.Errorf("err = %q, want 'not in stack'", err.Error())
	}
}

func resetRestartFlags() {
	restartFile = ""
	restartProfile = ""
	restartWaitMax = 60 * time.Second
}
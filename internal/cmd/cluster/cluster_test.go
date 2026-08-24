package cluster

import (
	"testing"
)

func TestNewClusterCmd_BasicArgs(t *testing.T) {
	cmd := NewClusterCmd()
	if cmd.Use != "cluster" {
		t.Errorf("Use = %q, want cluster", cmd.Use)
	}
	// 2 子命令
	wantCmds := map[string]bool{"status": false, "agents": false}
	for _, sub := range cmd.Commands() {
		if _, ok := wantCmds[sub.Name()]; ok {
			wantCmds[sub.Name()] = true
		}
	}
	for name, found := range wantCmds {
		if !found {
			t.Errorf("subcommand %q missing", name)
		}
	}
	// Aliases
	foundCl := false
	for _, a := range cmd.Aliases {
		if a == "cl" {
			foundCl = true
		}
	}
	if !foundCl {
		t.Errorf("alias 'cl' missing: %v", cmd.Aliases)
	}
}

func TestNewStatusCmd_BasicArgs(t *testing.T) {
	cmd := NewStatusCmd()
	if cmd.Use != "status" {
		t.Errorf("Use = %q, want status", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE missing")
	}
	for _, name := range []string{"json", "timeout"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
}

func TestNewAgentsCmd_BasicArgs(t *testing.T) {
	cmd := NewAgentsCmd()
	if cmd.Use != "agents" {
		t.Errorf("Use = %q, want agents", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE missing")
	}
	for _, name := range []string{"json", "page", "page-size", "skill", "status", "search"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q missing", name)
		}
	}
	// alias
	foundLs := false
	for _, a := range cmd.Aliases {
		if a == "ls" {
			foundLs = true
		}
	}
	if !foundLs {
		t.Errorf("alias 'ls' missing: %v", cmd.Aliases)
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		seconds float64
		want    string
	}{
		{0.5, "0.5s"},    // < 1min → "%.1fs"
		{45, "45.0s"},    // < 1min → "%.1fs"
		{125, "2m 5s"},   // 2 min 5 sec
		{3700, "1h 1m"},  // 1 hour 1 min
		{90000, "1d 1h"}, // 1 day 1 hour
	}
	for _, c := range cases {
		got := formatUptime(c.seconds)
		if got != c.want {
			t.Errorf("formatUptime(%v) = %q, want %q", c.seconds, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-10", 10, "exactly-10"},
		{"this-is-a-long-string", 10, "this-is..."},
		{"abc", 2, "ab"},   // n < 3 时不补 "..."
		{"abcdef", 4, "a..."},
	}
	for _, c := range cases {
		got := truncate(c.in, c.n)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestJoinStrings(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a, b"},
		{[]string{"x", "y", "z"}, "x, y, z"},
	}
	for _, c := range cases {
		got := joinStrings(c.in)
		if got != c.want {
			t.Errorf("joinStrings(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
package stack

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuntime_LoadAndSave(t *testing.T) {
	dir := t.TempDir()
	rt, err := LoadRuntime(dir, "test")
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if len(rt.Services) != 0 {
		t.Errorf("fresh runtime should be empty, got %d services", len(rt.Services))
	}

	// SetStatus first time auto-creates entry
	if err := rt.SetStatus("svc-a", "running", 12345, map[string]interface{}{
		"binary":   "wau-core",
		"httpPort": 18400,
	}); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	// Reload and verify
	rt2, err := LoadRuntime(dir, "test")
	if err != nil {
		t.Fatalf("LoadRuntime reload: %v", err)
	}
	svcA, ok := rt2.Services["svc-a"]
	if !ok {
		t.Fatal("svc-a not persisted")
	}
	if svcA.Status != "running" || svcA.PID != 12345 {
		t.Errorf("svc-a state wrong: %+v", svcA)
	}
	if svcA.Binary != "wau-core" || svcA.HTTPPort != 18400 {
		t.Errorf("extras not applied: %+v", svcA)
	}
	if svcA.StartedAt.IsZero() {
		t.Error("StartedAt should be set on running")
	}
}

func TestRuntime_Remove(t *testing.T) {
	dir := t.TempDir()
	rt, _ := LoadRuntime(dir, "test")
	_ = rt.SetStatus("svc-a", "running", 100, nil)
	if err := rt.Remove("svc-a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := rt.Services["svc-a"]; ok {
		t.Error("svc-a should be removed")
	}
}

func TestRuntime_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	rt, err := LoadRuntime(dir, "ghost")
	if err != nil {
		t.Errorf("LoadRuntime should not error on missing file: %v", err)
	}
	if rt == nil || len(rt.Services) != 0 {
		t.Error("should return empty runtime")
	}
}

func TestPIDFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	if err := WritePIDFile(path, 99999); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	pid, err := ReadPIDFile(path)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if pid != 99999 {
		t.Errorf("pid=%d, want 99999", pid)
	}
}

func TestIsAlive(t *testing.T) {
	myPID := os.Getpid()
	if !IsAlive(myPID) {
		t.Error("current process should be alive")
	}
	if IsAlive(999999999) {
		t.Error("nonexistent pid should not be alive")
	}
	if IsAlive(0) {
		t.Error("pid 0 should not be alive")
	}
	if IsAlive(-1) {
		t.Error("negative pid should not be alive")
	}
}

func TestBinaryLookup_NotFound(t *testing.T) {
	lookup := BinaryLookup{
		Home:      "/nonexistent/path",
		GOBIN:     "",
		ExtraDirs: []string{},
	}
	_, err := lookup.Resolve("definitely-not-a-real-binary-xyz")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
	if !containsAny(err.Error(), []string{"go install", "not found"}) {
		t.Errorf("error message should hint go install or say not found, got: %v", err)
	}
}

func TestProcessManager_StartAndStop(t *testing.T) {
	pm := NewProcessManager()
	dir := t.TempDir()

	// 用 /bin/sleep 作为 long-running binary
	svc := &Service{
		Name:   "test-svc",
		Binary: "sleep",
		Args:   []string{"30"},
	}

	pid, err := pm.Start(context.Background(), svc, dir)
	if err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	defer func() { _ = pm.Stop(context.Background(), pid) }()

	if pid <= 0 {
		t.Errorf("got pid %d, want >0", pid)
	}
	if !IsAlive(pid) {
		t.Errorf("started pid %d not alive", pid)
	}

	// Stop should kill it
	if err := pm.Stop(context.Background(), pid); err != nil {
		t.Errorf("Stop: %v", err)
	}
	// 等待 OS 回收
	time.Sleep(100 * time.Millisecond)
	if IsAlive(pid) {
		t.Errorf("pid %d still alive after stop", pid)
	}
}

func TestResolveDirs_HomeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	s := &Stack{
		Stack: StackMeta{
			DataDir: "~/.wau/test/run",
			LogDir:  "~/.wau/test/log",
		},
	}
	dataDir, logDir, err := s.ResolvedDirs()
	if err != nil {
		t.Fatalf("ResolvedDirs: %v", err)
	}
	if dataDir != filepath.Join(home, ".wau", "test", "run") {
		t.Errorf("dataDir=%q, want %q", dataDir, filepath.Join(home, ".wau", "test", "run"))
	}
	if logDir != filepath.Join(home, ".wau", "test", "log") {
		t.Errorf("logDir=%q", logDir)
	}
	// cleanup
	_ = os.RemoveAll(filepath.Join(home, ".wau", "test"))
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOfSub(s, sub) >= 0
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

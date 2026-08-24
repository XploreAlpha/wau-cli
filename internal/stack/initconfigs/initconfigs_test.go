package initconfigs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListTemplates(t *testing.T) {
	ts, err := ListTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 4 {
		t.Errorf("want 4 templates, got %d", len(ts))
	}
	wantServices := map[string]bool{
		"wau-store": false, "wau-llm-router": false, "wau-edge": false, "wau-channel": false,
	}
	for _, tpl := range ts {
		if _, ok := wantServices[tpl.Service]; !ok {
			t.Errorf("unexpected service %q", tpl.Service)
		}
		wantServices[tpl.Service] = true
	}
	for svc, found := range wantServices {
		if !found {
			t.Errorf("missing service %q", svc)
		}
	}
}

func TestTemplateByService(t *testing.T) {
	cases := []struct {
		in       string
		wantSvc  string
		wantFile string
	}{
		{"wau-store", "wau-store", "store.yaml"},
		{"store", "wau-store", "store.yaml"},
		{"wau-llm-router", "wau-llm-router", "router.yaml"},
		{"router", "wau-llm-router", "router.yaml"},
		{"WAU-STORE", "wau-store", "store.yaml"}, // 大小写不敏感
		{"wau-edge", "wau-edge", "edge.yaml"},
		{"wau-channel", "wau-channel", "channel.yaml"},
	}
	for _, tc := range cases {
		tpl, err := TemplateByService(tc.in)
		if err != nil {
			t.Errorf("TemplateByService(%q) err: %v", tc.in, err)
			continue
		}
		if tpl.Service != tc.wantSvc {
			t.Errorf("TemplateByService(%q).Service = %q, want %q", tc.in, tpl.Service, tc.wantSvc)
		}
		if tpl.Filename != tc.wantFile {
			t.Errorf("TemplateByService(%q).Filename = %q, want %q", tc.in, tpl.Filename, tc.wantFile)
		}
		if len(tpl.Contents) == 0 {
			t.Errorf("TemplateByService(%q) contents empty", tc.in)
		}
	}
}

func TestTemplateByService_NotFound(t *testing.T) {
	_, err := TemplateByService("wau-ghost")
	if err == nil {
		t.Fatal("want error for unknown service")
	}
	if !strings.Contains(err.Error(), "no template") {
		t.Errorf("err = %v, want 'no template'", err)
	}
}

func TestRemapServiceName(t *testing.T) {
	cases := map[string]string{
		"wau-router":    "wau-llm-router",
		"wau-store":     "wau-store",
		"wau-ghost":     "wau-ghost", // 没映射,原样
		"wau-llm-router": "wau-llm-router",
	}
	for in, want := range cases {
		got := remapServiceName(in)
		if got != want {
			t.Errorf("remapServiceName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		"":      "",
		"/abs":  "/abs",
		"rel":   "rel",
		"~":     home,
		"~/x":   filepath.Join(home, "x"),
		"~~/x":  "~~/x", // 不展开:不是合法的 ~ path(必须 ~/ 开头或单独的 ~)
	}
	for in, want := range cases {
		got := ExpandHome(in)
		if got != want {
			t.Errorf("ExpandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

// ─── Writer tests ──────────────────────────────────────────────────────────

func TestWriter_Write_New(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{OutputDir: dir, Force: false, DryRun: false}
	tpl, err := TemplateByService("wau-store")
	if err != nil {
		t.Fatal(err)
	}
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Errorf("status = %q, want wrote (err=%v)", res.Status, res.Err)
	}
	if res.Size == 0 {
		t.Error("size = 0, want > 0")
	}
	// 文件真的写出来了
	got, err := os.ReadFile(filepath.Join(dir, "store.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(tpl.Contents) {
		t.Error("written file content mismatch")
	}
}

func TestWriter_Write_ExistsSkip(t *testing.T) {
	dir := t.TempDir()
	// 预先写一个不同的内容
	existing := filepath.Join(dir, "store.yaml")
	if err := os.WriteFile(existing, []byte("# pre-existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &Writer{OutputDir: dir, Force: false, DryRun: false}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "skipped" {
		t.Errorf("status = %q, want skipped", res.Status)
	}
	// 内容保持原样
	got, _ := os.ReadFile(existing)
	if string(got) != "# pre-existing\n" {
		t.Errorf("file overwritten despite skip; got %q", got)
	}
}

func TestWriter_Write_ExistsForce(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "store.yaml")
	if err := os.WriteFile(existing, []byte("# pre-existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &Writer{OutputDir: dir, Force: true, DryRun: false}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Errorf("status = %q, want wrote (err=%v)", res.Status, res.Err)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != string(tpl.Contents) {
		t.Error("file not overwritten despite Force")
	}
}

func TestWriter_DryRun(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{OutputDir: dir, DryRun: true}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "would-write" {
		t.Errorf("status = %q, want would-write", res.Status)
	}
	if res.Size != len(tpl.Contents) {
		t.Errorf("size = %d, want %d", res.Size, len(tpl.Contents))
	}
	// DryRun 不应写文件
	if _, err := os.Stat(filepath.Join(dir, "store.yaml")); !os.IsNotExist(err) {
		t.Error("DryRun wrote a file")
	}
}

func TestWriter_WriteAll(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{OutputDir: dir}
	ts, _ := ListTemplates()
	results := w.WriteAll(ts)
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	wrote := 0
	for _, r := range results {
		if r.Status == "wrote" {
			wrote++
		}
	}
	if wrote != 4 {
		t.Errorf("wrote = %d, want 4", wrote)
	}
	// 4 个文件都应该存在
	for _, tpl := range ts {
		p := filepath.Join(dir, tpl.Filename)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestWriter_WriteAll_PartialSkip(t *testing.T) {
	dir := t.TempDir()
	// 预先写 1 个
	if err := os.WriteFile(filepath.Join(dir, "store.yaml"), []byte("# pre"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &Writer{OutputDir: dir}
	ts, _ := ListTemplates()
	results := w.WriteAll(ts)
	wrote, skipped := 0, 0
	for _, r := range results {
		switch r.Status {
		case "wrote":
			wrote++
		case "skipped":
			skipped++
		}
	}
	if wrote != 3 || skipped != 1 {
		t.Errorf("wrote=%d skipped=%d, want 3/1", wrote, skipped)
	}
}

func TestWriter_NestedOutputDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "nested", "configs")
	w := &Writer{OutputDir: nested}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Errorf("status = %q, want wrote (err=%v)", res.Status, res.Err)
	}
	if _, err := os.Stat(filepath.Join(nested, "store.yaml")); err != nil {
		t.Errorf("nested file not written: %v", err)
	}
}

func TestWriter_AtomicWrite_NoPartialFile(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{OutputDir: dir}
	tpl, _ := TemplateByService("wau-store")
	_ = w.Write(tpl)
	// 确认 .tmp 没遗留
	if _, err := os.Stat(filepath.Join(dir, "store.yaml.tmp")); !os.IsNotExist(err) {
		t.Error(".tmp file leaked after successful write")
	}
}
// ─── P4.5 EnvSubst tests ───────────────────────────────────────────────────

func TestWriter_EnvSubst_Template(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WAU_STORE_PG_DSN", "postgres://demo:demo@localhost:5432/wau_store")
	t.Setenv("WAU_STORE_REDIS_PASSWORD", "redispass")
	t.Setenv("WAU_STORE_ADMIN_TOKEN", "admintoken")

	w := &Writer{OutputDir: dir, EnvSubst: true}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Fatalf("status = %q, want wrote (err=%v)", res.Status, res.Err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "store.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// 验证替换
	for _, want := range []string{
		"postgres://demo:demo@localhost:5432/wau_store",
		"redispass",
		"admintoken",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("expanded file missing %q", want)
		}
	}
	// 验证原占位符消失
	if strings.Contains(string(got), "$WAU_STORE_PG_DSN") {
		t.Error("$WAU_STORE_PG_DSN not substituted")
	}
}

func TestWriter_EnvSubst_MissingEnvVar(t *testing.T) {
	dir := t.TempDir()
	// 不 set 任何东西 → $VAR 替换成空字符串(os.ExpandEnv 默认行为)
	t.Setenv("WAU_STORE_PG_DSN", "")
	t.Setenv("WAU_STORE_REDIS_PASSWORD", "")
	t.Setenv("WAU_STORE_ADMIN_TOKEN", "")

	w := &Writer{OutputDir: dir, EnvSubst: true}
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Fatalf("status = %q, want wrote (err=%v)", res.Status, res.Err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "store.yaml"))
	// $VAR 替换成空字符串后,原来的 `dsn: "$WAU_STORE_PG_DSN"` → `dsn: ""`
	if strings.Contains(string(got), "$WAU_STORE_PG_DSN") {
		t.Error("$VAR not expanded when env empty (got literal back)")
	}
	// 确认 dsn 行真的变了
	if !strings.Contains(string(got), `dsn: ""`) {
		t.Errorf("dsn line not empty-quoted after substitution:\n%s", string(got))
	}
}

func TestWriter_NoEnvSubst_PreservesLiteral(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WAU_STORE_PG_DSN", "should-not-be-used")

	w := &Writer{OutputDir: dir, EnvSubst: false} // 不展开
	tpl, _ := TemplateByService("wau-store")
	res := w.Write(tpl)
	if res.Status != "wrote" {
		t.Fatalf("status = %q", res.Status)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "store.yaml"))
	if !strings.Contains(string(got), "$WAU_STORE_PG_DSN") {
		t.Error("EnvSubst=false should preserve $VAR literal")
	}
	if strings.Contains(string(got), "should-not-be-used") {
		t.Error("env var leaked into output despite EnvSubst=false")
	}
}

func TestExtractEnvVars_Template(t *testing.T) {
	tpl, _ := TemplateByService("wau-store")
	vars := ExtractEnvVars(tpl.Contents)
	want := map[string]bool{
		"WAU_STORE_PG_DSN":          false,
		"WAU_STORE_REDIS_PASSWORD":  false,
		"WAU_STORE_ADMIN_TOKEN":     false,
	}
	for _, v := range vars {
		if _, ok := want[v]; ok {
			want[v] = true
		}
	}
	for v, found := range want {
		if !found {
			t.Errorf("ExtractEnvVars missing %q (got: %v)", v, vars)
		}
	}
}

func TestExtractEnvVars_BothSyntaxes(t *testing.T) {
	// $BAR 出现两次应只返回 1 个;${BAZ} 跟 $BAR 两种语法都要识别
	content := []byte("foo: $BAR\nbaz: ${BAZ}\nqux: $BAR again\n")
	vars := ExtractEnvVars(content)
	want := map[string]bool{"BAR": false, "BAZ": false}
	for _, v := range vars {
		if _, ok := want[v]; ok {
			want[v] = true
		}
	}
	if len(vars) != 2 {
		t.Errorf("len = %d, want 2 (got: %v)", len(vars), vars)
	}
	for v, found := range want {
		if !found {
			t.Errorf("missing %q", v)
		}
	}
}

func TestExtractEnvVars_Empty(t *testing.T) {
	cases := [][]byte{
		[]byte(""),
		[]byte("# no env vars here\nfoo: bar\n"),
		[]byte("$ alone\n$$ literal\n"),
	}
	for _, c := range cases {
		vars := ExtractEnvVars(c)
		// "$" 单独不匹配(没变量名)
		if len(vars) != 0 {
			t.Errorf("ExtractEnvVars(%q) = %v, want empty", c, vars)
		}
	}
}

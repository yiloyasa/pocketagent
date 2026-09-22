package hostenv

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestNodeCandidatesDeriveFromEnv 钉住「候选链随用户环境变化」这一性质。
//
// 【为什么这是关键不变量】消除硬编码的本质不是"换一条路径"，而是"路径不再是
// 常量"。所以判据不能是"候选里有没有某个字符串"（写死一条也能满足），而必须是
// 「改环境变量 → 候选随之改变」。将来若有人把绝对路径写回来，这条会红。
func TestNodeCandidatesDeriveFromEnv(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `X:\fake\local`)
	t.Setenv("ProgramFiles", `X:\fake\pf`)
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("PATH", "")

	got := NodeCandidates()
	want := filepath.Join(`X:\fake\local`, "Programs", "nodejs", "node.exe")
	if !slices.Contains(got, want) {
		t.Fatalf("候选链未从 LOCALAPPDATA 派生（说明仍是硬编码路径）:\n  期望含 %s\n  实得 %v", want, got)
	}
	wantPF := filepath.Join(`X:\fake\pf`, "nodejs", "node.exe")
	if !slices.Contains(got, wantPF) {
		t.Fatalf("候选链未覆盖 ProgramFiles:\n  期望含 %s\n  实得 %v", wantPF, got)
	}
}

// TestNpmGlobalRootsDeriveFromEnv 钉住 npm 全局根同样按环境派生。
func TestNpmGlobalRootsDeriveFromEnv(t *testing.T) {
	t.Setenv("APPDATA", `X:\fake\roaming`)
	t.Setenv("LOCALAPPDATA", `X:\fake\local`)
	t.Setenv("ProgramFiles", "")

	got := NpmGlobalRoots()
	want := filepath.Join(`X:\fake\roaming`, "npm", "node_modules")
	if !slices.Contains(got, want) {
		t.Fatalf("npm 全局根未从 APPDATA 派生:\n  期望含 %s\n  实得 %v", want, got)
	}
}

// TestPackageCandidatesExpandRelToAllRoots 钉住包内相对路径会展开到每个根下。
func TestPackageCandidatesExpandRelToAllRoots(t *testing.T) {
	t.Setenv("APPDATA", `X:\fake\roaming`)
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("ProgramFiles", "")

	got := PackageCandidates("@scope/name/dist/cli.js")
	want := filepath.Join(`X:\fake\roaming`, "npm", "node_modules", "@scope", "name", "dist", "cli.js")
	if !slices.Contains(got, want) {
		t.Fatalf("包内相对路径未展开到 npm 全局根:\n  期望含 %s\n  实得 %v", want, got)
	}
	// 展开结果必须是绝对路径 —— 相对路径交给 CreateProcess 只会得到费解的错误。
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Fatalf("候选应为绝对路径: %q", p)
		}
	}
}

// TestFindNodeReturnsEmptyWhenNothingFound 钉住"找不到就返回空串"，
// 而不是回落到任何写死的路径（回落写死路径正是本模块要消灭的形态）。
func TestFindNodeReturnsEmptyWhenNothingFound(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `X:\definitely\missing`)
	t.Setenv("ProgramFiles", "")
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("PATH", `X:\definitely\missing\bin`)

	if got := FindNode(); got != "" {
		t.Fatalf("无可达 node 时应返回空串（交由调用方报可操作的错误），实得 %q", got)
	}
}

// TestDeveloperTracesInDetectsKnownNeedles 正反例：命中表内关键字返回命中，
// 干净内容返回空。
func TestDeveloperTracesInDetectsKnownNeedles(t *testing.T) {
	dirty := []byte("const Node = `C:\\Users\\yanzi\\node.exe`\n")
	if hits := DeveloperTracesIn(dirty); len(hits) == 0 {
		t.Fatal("含开发机用户名的内容必须被检出")
	}
	clean := []byte(`const Node = "node.exe"` + "\n")
	if hits := DeveloperTracesIn(clean); len(hits) != 0 {
		t.Fatalf("干净内容不该有命中: %v", hits)
	}
	// 大小写不敏感（表内 "yanzi" 为小写，源码里可能是首字母大写）
	if hits := DeveloperTracesIn([]byte("C:\\Users\\Yanzi\\node.exe")); len(hits) == 0 {
		t.Fatal("大小写不同也应命中")
	}
}

// TestNeedlesAreAllNonEmpty 防御：关键字表里若混入空串，
// bytes.Contains(x, "") 恒为真 —— 整条闸门会变成"什么都报"或"什么都放过"。
func TestNeedlesAreAllNonEmpty(t *testing.T) {
	for _, n := range developerTraceNeedles() {
		if strings.TrimSpace(n) == "" {
			t.Fatal("关键字表含空串 —— 会让 bytes.Contains 恒真/恒假，闸门失效")
		}
	}
}

// pathguard_test.go —— 共享路径护栏的清单与锚点回归(2026-10-04 F3 收口)。
//
// 【钉什么】GuardDeny 七项判定(①~⑦)+ 四个审查缺口的修复锚点:
//	G1 junction→禁区根:物理锚点比对(server 包 TestGuardDangerousPathJunctionEscape
//	    端到端复验,这里不重复建 junction,只钉物理解析函数本身);
//	G2 裸盘符 `C:`(Clean/Abs 之前识别);
//	G3 设备形态 `\\?\C:\`(剥前缀后命中①);
//	G4 管理共享回指 `\\localhost\C$\Windows`(X$ 别名归一后命中②)。
// 【设计纪律】期望拒绝的路径只调 GuardDeny 本身,绝不真正执行删除/移动;
// 根目录类用例(盘根/系统目录/主目录/Temp 根)读环境得出,不创建、不修改。

package pathguard

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestResolvePhysicalExistingDir 物理解析的地基行为:真实目录解析成功且与
// Clean 形态一致(大小写不敏感比较,盘上大小写漂移被归一);不存在的路径
// 如实报 ok=false(调用方据此退回原始形态比对,不猜)。
func TestResolvePhysicalExistingDir(t *testing.T) {
	dir := t.TempDir()
	got, ok := ResolvePhysical(dir)
	if !ok {
		t.Fatalf("真实目录 %s 应能解析出物理路径", dir)
	}
	if !equalFold(got, filepath.Clean(dir)) {
		t.Fatalf("物理路径应与 Clean 形态一致(仅大小写可漂移): got=%s want=%s", got, filepath.Clean(dir))
	}
	missing := filepath.Join(t.TempDir(), "not-yet-created")
	if p, ok := ResolvePhysical(missing); ok {
		t.Fatalf("不存在的路径应返回 ok=false,实际: %s", p)
	}
}

// TestGuardDenyDenyList 命中清单表驱动:每个用例同时钉"拒不拒"与"命中哪条"
// (Kind 断言防止清单条目之间互相吞掉 —— 比如数据子树误报成系统目录)。
func TestGuardDenyDenyList(t *testing.T) {
	dataDir := t.TempDir()
	denied := []struct {
		name string
		path string
		kind DenyKind
	}{
		// ⑤ 系统临时目录根(各平台稳定命中)
		{"temp根", filepath.Clean(os.TempDir()), DenyHomeLikeRoot},
		// ⑥ daemon 数据目录:唯一整棵子树都拒 —— 根、深层、文件三层都要命中
		{"数据目录根本身", dataDir, DenyDataDir},
		{"数据目录深层", filepath.Join(dataDir, "collab", "artifacts", "sess", "m1"), DenyDataDir},
		{"数据目录内文件", filepath.Join(dataDir, "threads.json"), DenyDataDir},
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		denied = append(denied, struct {
			name, path string
			kind       DenyKind
		}{"用户主目录根", home, DenyHomeLikeRoot})
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		denied = append(denied, struct {
			name, path string
			kind       DenyKind
		}{"进程工作目录", wd, DenyProcessCwd})
	}
	if exe, err := os.Executable(); err == nil {
		denied = append(denied, struct {
			name, path string
			kind       DenyKind
		}{"可执行文件所在目录", filepath.Dir(exe), DenyExeDir})
	}
	if runtime.GOOS == "windows" {
		denied = append(denied,
			struct {
				name, path string
				kind       DenyKind
			}{"盘根大写", `C:\`, DenyDriveRoot},
			struct {
				name, path string
				kind       DenyKind
			}{"盘根小写双斜杠", `c:\`, DenyDriveRoot},
			struct {
				name, path string
				kind       DenyKind
			}{"盘根正斜杠", `c:/`, DenyDriveRoot},
			// G2 裸盘符:Abs 会把它归一成 <cwd>\C: 而漏检,必须在归一化前识别
			struct {
				name, path string
				kind       DenyKind
			}{"裸盘符", `C:`, DenyDriveRoot},
			struct {
				name, path string
				kind       DenyKind
			}{"裸盘符小写", `d:`, DenyDriveRoot},
			// G3 设备形态盘根:物理解析剥 \\?\ 前缀后命中①
			struct {
				name, path string
				kind       DenyKind
			}{"设备形态盘根", `\\?\C:\`, DenyDriveRoot},
			// G4 管理共享回指本机:X$ 别名归一后命中②
			struct {
				name, path string
				kind       DenyKind
			}{"管理共享回指系统目录", `\\localhost\C$\Windows`, DenySystemRoot},
			struct {
				name, path string
				kind       DenyKind
			}{"管理共享回指(环回地址)", `\\127.0.0.1\C$\Windows`, DenySystemRoot},
		)
		if sysRoot := filepath.Clean(os.Getenv("SystemRoot")); sysRoot != "" && sysRoot != "." {
			denied = append(denied,
				struct {
					name, path string
					kind       DenyKind
				}{"系统目录原样", sysRoot, DenySystemRoot},
				struct {
					name, path string
					kind       DenyKind
				}{"系统目录小写", lower(sysRoot), DenySystemRoot},
				struct {
					name, path string
					kind       DenyKind
				}{"系统目录顶层子项", filepath.Join(sysRoot, "System32"), DenySystemRootChild},
				struct {
					name, path string
					kind       DenyKind
				}{"系统目录经.归一", sysRoot + `\.`, DenySystemRoot},
				// `..` 归一回的是盘根(Clean("X:\WINDOWS\..")="X:\"),不是系统目录:
				// 单锚点按清单①→⑦序首个命中,① 先于 ② → DenyDriveRoot(与表内
				// "盘根经..归一" 同一落点同一 Kind;收口前旧 fs.go 的 len<=3 规则
				// 对该形态同样按驱动器根拒绝,Kind 口径与历史行为一致)。
				struct {
					name, path string
					kind       DenyKind
				}{"系统目录经..归一回盘根(① 先于 ②)", sysRoot + `\..`, DenyDriveRoot},
			)
		}
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if root := filepath.Clean(os.Getenv(env)); root != "" && root != "." {
				denied = append(denied, struct {
					name, path string
					kind       DenyKind
				}{env + "根", root, DenyHomeLikeRoot})
			}
		}
		if up := filepath.Clean(os.Getenv("USERPROFILE")); up != "" && up != "." {
			denied = append(denied, struct {
				name, path string
				kind       DenyKind
			}{"USERPROFILE根", up, DenyHomeLikeRoot})
		}
		denied = append(denied, struct {
			name, path string
			kind       DenyKind
		}{"盘根经..归一", `C:\Windows\..`, DenyDriveRoot})
	}

	for _, c := range denied {
		r := GuardDeny(c.path, []string{dataDir})
		if !r.Denied {
			t.Errorf("GuardDeny 应拒绝 %s(%s),实际放行", c.path, c.name)
			continue
		}
		if r.Kind != c.kind {
			t.Errorf("GuardDeny(%s) 命中条目应=%v,实际=%v(条目互吞或顺序漂移)", c.path, c.kind, r.Kind)
		}
	}
}

// TestGuardDenyAllowList 放行面:清单只挡"根/顶层本身/数据子树",不误伤普通
// 工作路径与远端命名共享;护栏不做存在性裁决(不存在 ≠ 拒绝)。
func TestGuardDenyAllowList(t *testing.T) {
	dataDir := t.TempDir()
	allowed := []string{
		filepath.Join(t.TempDir(), "workspace", "project", "deep"),
		filepath.Join(t.TempDir(), "not-yet-created"),
		filepath.Join(filepath.Dir(dataDir), "sibling-ok", "deep"),
		`\\server\share\docs\file.txt`,
		`\\?\UNC\server\share\docs`,
		`rel-sub\dir\file.txt`,
	}
	if runtime.GOOS != "windows" {
		// Windows 专属形态在别的平台是普通相对名,不进放行断言
		allowed = allowed[:3]
	}
	for _, p := range allowed {
		if r := GuardDeny(p, []string{dataDir}); r.Denied {
			t.Errorf("GuardDeny 不应拒绝 %s,实际命中条目=%v", p, r.Kind)
		}
	}
}

// TestGuardDenyDataDirsEmptyExtra 空 dataDirs:调用方自身数据目录缺席时只剩
// hostenv 标准/旧版位置在守;传入的空串被过滤,不产生相对路径根的怪异比对。
func TestGuardDenyDataDirsEmptyExtra(t *testing.T) {
	r := GuardDeny(filepath.Join(t.TempDir(), "deep", "ok"), nil)
	if r.Denied {
		t.Errorf("无 dataDirs 时普通路径不应被拒: %v", r.Kind)
	}
	r = GuardDeny(filepath.Join(t.TempDir(), "deep", "ok"), []string{"", "   "})
	if r.Denied {
		t.Errorf("dataDirs 全为空串时普通路径不应被拒: %v", r.Kind)
	}
}

// TestAdminShareAlias X$ 别名归一的形态学边界:恰好的"单盘符+$"才映射,
// 同名普通共享 / 残缺 / 设备形态不误映射。
func TestAdminShareAlias(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("管理共享形态是 Windows 专属")
	}
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{`\\localhost\C$`, `C:\`, true},
		{`\\localhost\C$\Windows`, `C:\Windows`, true},
		{`\\localhost\C$\Windows\System32`, `C:\Windows\System32`, true},
		{`\\LOCALHOST\c$\w`, `c:\w`, true},
		{`\\127.0.0.1\D$\data`, `D:\data`, true},
		{`\\srv\share\x`, "", false},  // 普通命名共享:绝不映射
		{`\\h\c$xyz`, "", false},      // 以 X$ 开头的共享名:不映射
		{`\\h\share`, "", false},      // 非 X$ 形态
		{`\\?\UNC\h\C$\w`, "", false}, // 设备形态:走剥前缀锚点,别名不重复处理
		{`C:\Windows`, "", false},     // 本地路径
		{`\\`, "", false},             // 裸双斜杠残缺形态
	}
	for _, c := range cases {
		got, ok := adminShareAlias(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("adminShareAlias(%q) = (%q, %v),期望 (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestIsBareDriveRoot 裸盘符识别边界:恰好两字符、盘位必须是字母。
func TestIsBareDriveRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("盘符相对形态是 Windows 专属")
	}
	for _, in := range []string{`C:`, `z:`} {
		if !isBareDriveRoot(in) {
			t.Errorf("isBareDriveRoot(%q) 应为 true", in)
		}
	}
	for _, in := range []string{`C:\`, `C:\.`, `cc:`, `1:`, `:`, `C`, `C:\Windows`} {
		if isBareDriveRoot(in) {
			t.Errorf("isBareDriveRoot(%q) 应为 false", in)
		}
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func lower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}

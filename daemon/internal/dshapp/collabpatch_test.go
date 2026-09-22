package dshapp

// collabpatch_test.go —— 把 collabpatch.go 的分支**真的钉死**。
//
// 【为什么全是临时目录】WriteCollabMcpPatch 的落盘目标是隔离 HOME 的
// profile 补丁层,真实路径在 ~/.pocketagentd/dsh-home 下、归 daemon 所有。
// 测试一律用 t.TempDir(),绝不碰用户的 ~/.dsh / ~/.pocketagentd —— 读写用户
// 目录是红线,验证也只在内存/临时文件里做。
//
// 【YAML 怎么校验】本机没装 pyyaml,又不许引新依赖。这里**独立**手写一份
// 双引号标量反解(unquoteYAMLScalar),与生产代码的 yamlScalar 不共享实现:
// 若 yamlScalar 哪天漏转义了某个字符(最典型是 Windows 路径的 `\`),反解
// 会撞到"非法转义"或值对不上,测试即红。这比"用生产函数验自己"更能钉住
// 转义正确性。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unquoteYAMLScalar 是 yamlScalar 的**独立**反实现,只认双引号标量的合法转义。
// 见到无法识别的转义(\ 后不是 \ " n r t x)即报错 —— 这恰好能让"生产漏转义"
// 暴露出来。
func unquoteYAMLScalar(in string) (string, error) {
	if len(in) < 2 || in[0] != '"' || in[len(in)-1] != '"' {
		return "", errNotScalar(in)
	}
	inner := in[1 : len(in)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(inner) {
			return "", errDanglingBS(in)
		}
		n := inner[i+1]
		switch n {
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		default:
			return "", errBadEscape(in, n)
		}
		i++
	}
	return b.String(), nil
}

type yamlScalarErr string

func (e yamlScalarErr) Error() string { return "非双引号 YAML 标量: " + string(e) }
func errNotScalar(s string) error     { return yamlScalarErr(s) }
func errDanglingBS(s string) error    { return yamlScalarErr("悬空反斜杠: " + s) }
func errBadEscape(s string, n byte) error {
	return yamlScalarErr("非法转义 \\" + string(n) + ": " + s)
}

// parseRenderedScalars 把 renderCollabPatch 产出的受限 YAML(顶层数组 + 固定
// 嵌套)解析成 {键:值} 映射与顺序 args 列表。只覆盖我们这种形状,够用即可,
// 不引 YAML 库。
func parseRenderedScalars(t *testing.T, content string) (map[string]string, []string) {
	t.Helper()
	kv := map[string]string{}
	var args []string
	for _, line := range strings.Split(content, "\n") {
		s := strings.TrimLeft(line, " ")
		if strings.HasPrefix(s, "#") || s == "" {
			continue
		}
		if strings.HasPrefix(s, "- ") { // 列表项:args 的标量,或顶层 "- id: ..." 这种映射
			inner := strings.TrimSpace(s[2:])
			if strings.HasPrefix(inner, `"`) {
				u, err := unquoteYAMLScalar(inner)
				if err != nil {
					t.Fatalf("args 标量反解失败: %v", err)
				}
				args = append(args, u)
				continue
			}
			// 列表项本身是一个映射(如顶层 "- id: \"...\"")——按映射键收进 kv。
			if idx := strings.Index(inner, ":"); idx >= 0 {
				key := strings.TrimSpace(inner[:idx])
				rest := strings.TrimSpace(inner[idx+1:])
				if strings.HasPrefix(rest, `"`) {
					u, err := unquoteYAMLScalar(rest)
					if err != nil {
						t.Fatalf("列表项映射值反解失败: %v", err)
					}
					kv[key] = u
				}
			}
			continue
		}
		idx := strings.Index(s, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(s[:idx])
		rest := strings.TrimSpace(s[idx+1:])
		if rest == "" || !strings.HasPrefix(rest, `"`) {
			continue // 块键(config:/args:/env:),无标量
		}
		u, err := unquoteYAMLScalar(rest)
		if err != nil {
			t.Fatalf("映射标量 %q 反解失败: %v", key, err)
		}
		kv[key] = u
	}
	return kv, args
}

// 用例 1:文件不存在 → 写入成功,返回 true。
func TestWriteCollabMcpPatchFileNotExists(t *testing.T) {
	home := filepath.Join(t.TempDir(), "no", "such", "dir") // 目录也不存在,测 MkdirAll
	ok, err := WriteCollabMcpPatch(home, `C:\pa\pa.exe`, []string{"-x"}, map[string]string{"K": "V"})
	if err != nil {
		t.Fatalf("不应出错: %v", err)
	}
	if !ok {
		t.Fatal("文件不存在时应写入并返回 true")
	}
	data, err := os.ReadFile(CollabPatchPath(home))
	if err != nil {
		t.Fatalf("应已落盘: %v", err)
	}
	kv, _ := parseRenderedScalars(t, string(data))
	if kv["id"] != collabPatchID {
		t.Fatalf("id 应为 %s, got %q", collabPatchID, kv["id"])
	}
}

// 用例 2:空文件 / `[]` / 只有注释 → 写入成功。
func TestWriteCollabMcpPatchEmptyLayers(t *testing.T) {
	for _, seed := range []string{"", "[]", "# 仅注释\n# 没条目\n"} {
		t.Run(fmtSeedName(seed), func(t *testing.T) {
			home := t.TempDir()
			path := CollabPatchPath(home)
			if seed != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ok, err := WriteCollabMcpPatch(home, `C:\pa\pa.exe`, nil, nil)
			if err != nil || !ok {
				t.Fatalf("空层应写入: ok=%v err=%v", ok, err)
			}
			got, _ := os.ReadFile(path)
			if !strings.Contains(string(got), collabPatchID) {
				t.Fatalf("应写入我们的条目, got:\n%s", got)
			}
		})
	}
}

// 用例 3:已含我们的 id → 返回 true 且**文件一字不动**(尊重用户手改的路径)。
func TestWriteCollabMcpPatchAlreadyOursUntouched(t *testing.T) {
	home := t.TempDir()
	path := CollabPatchPath(home)
	// 用户手改过的版本:command 是我们的 id 但路径不同。
	existing := "- id: " + collabPatchID + "\n" +
		`  name: "@deepseek-ai/dsh-mcp-client"` + "\n" +
		`  config:` + "\n" +
		`    serverName: "collab"` + "\n" +
		`    command: "C:\custom\path\pa.exe"` + "\n" // 用户的自定义路径
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := WriteCollabMcpPatch(home, `C:\other\pa.exe`, []string{"-x"}, nil)
	if err != nil {
		t.Fatalf("不应出错: %v", err)
	}
	if !ok {
		t.Fatal("已含 id 应返回 true(幂等)")
	}
	got, _ := os.ReadFile(path)
	if string(got) != existing {
		t.Fatalf("已含 id 时文件必须原样, 期望:\n%s\n实际:\n%s", existing, got)
	}
}

// 用例 4(最关键):非空且是别人的 patch → 返回 (false, nil) 且**原文一字不动**。
// 宁可这次没接上,也绝不抹掉用户内容。
func TestWriteCollabMcpPatchForeignUntouched(t *testing.T) {
	home := t.TempDir()
	path := CollabPatchPath(home)
	foreign := "- id: \"someone-elses-patch\"\n" +
		`  name: "other-plugin"` + "\n" +
		`  config:` + "\n" +
		`    command: "C:\other\thing.exe"` + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := WriteCollabMcpPatch(home, `C:\pa\pa.exe`, []string{"-x"}, nil)
	if ok || err != nil {
		t.Fatalf("别人的 patch 应返回 (false, nil), got ok=%v err=%v", ok, err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != foreign {
		t.Fatalf("别人的 patch 必须原封不动, 期望:\n%s\n实际:\n%s", foreign, got)
	}
}

// 用例 5:command 为空 → 返回 (false, nil) 且不写文件。
func TestWriteCollabMcpPatchEmptyCommandNoWrite(t *testing.T) {
	home := t.TempDir()
	ok, err := WriteCollabMcpPatch(home, "", []string{"-x"}, nil)
	if ok || err != nil {
		t.Fatalf("空 command 应返回 (false, nil), got ok=%v err=%v", ok, err)
	}
	if _, statErr := os.Stat(CollabPatchPath(home)); !os.IsNotExist(statErr) {
		t.Fatal("空 command 不应创建任何文件")
	}
}

// 用例 6:YAML 转义 —— 反斜杠路径 / 含空格 / 含冒号 URL / 含双引号,往返不变。
func TestWriteCollabMcpPatchEscapeRoundTrip(t *testing.T) {
	home := t.TempDir()
	cmd := `G:\项目1\pocketagent\bin\pa.exe` // 反斜杠 + 中文
	args := []string{
		`C:\Program Files\app\run.exe`, // 含空格
		`http://example.com:8080/v`,    // 含冒号 URL
		`say "hi" now`,                 // 含双引号
	}
	env := map[string]string{
		"POCKETAGENT_TOKEN": `tok"with\quote`,
		"URL":               `https://a.b:1/c`,
	}
	ok, err := WriteCollabMcpPatch(home, cmd, args, env)
	if err != nil || !ok {
		t.Fatalf("转义用例应写入: ok=%v err=%v", ok, err)
	}
	data, _ := os.ReadFile(CollabPatchPath(home))
	kv, gotArgs := parseRenderedScalars(t, string(data))
	if kv["command"] != cmd {
		t.Fatalf("command 转义往返失败:\n 期望 %q\n 实际 %q", cmd, kv["command"])
	}
	if len(gotArgs) != len(args) {
		t.Fatalf("args 数量不符: 期望 %d 实际 %d (%v)", len(args), len(gotArgs), gotArgs)
	}
	for i, a := range args {
		if gotArgs[i] != a {
			t.Fatalf("args[%d] 转义往返失败:\n 期望 %q\n 实际 %q", i, a, gotArgs[i])
		}
	}
	for k, v := range env {
		if kv[k] != v {
			t.Fatalf("env[%q] 转义往返失败:\n 期望 %q\n 实际 %q", k, v, kv[k])
		}
	}
	// 反向再确认:产出的文件里 command 行必须是双反斜杠转义过的字面量。
	if !strings.Contains(string(data), `command: "G:\\项目1\\pocketagent\\bin\\pa.exe"`) {
		t.Fatalf("Windows 反斜杠必须被转义为 \\\\, 实际:\n%s", data)
	}
}

// 用例 7:幂等 —— 连续两次不产生重复条目,内容一致。
func TestWriteCollabMcpPatchIdempotent(t *testing.T) {
	home := t.TempDir()
	first, err := WriteCollabMcpPatch(home, `C:\pa\pa.exe`, []string{"-x"}, map[string]string{"K": "V"})
	if err != nil || !first {
		t.Fatalf("首次应写入: ok=%v err=%v", first, err)
	}
	after1, _ := os.ReadFile(CollabPatchPath(home))
	second, err := WriteCollabMcpPatch(home, `C:\pa\pa.exe`, []string{"-x"}, map[string]string{"K": "V"})
	if err != nil || !second {
		t.Fatalf("二次应幂等返回 true: ok=%v err=%v", second, err)
	}
	after2, _ := os.ReadFile(CollabPatchPath(home))
	if string(after1) != string(after2) {
		t.Fatal("两次写入内容应完全一致")
	}
	idCount := strings.Count(string(after2), collabPatchID)
	if idCount != 1 {
		t.Fatalf("应只有 1 条我们的条目, 实际 %d", idCount)
	}
}

func fmtSeedName(seed string) string {
	switch seed {
	case "":
		return "empty-file"
	case "[]":
		return "brackets"
	default:
		return "comment-only"
	}
}

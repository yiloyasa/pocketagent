package hermesapp

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setHermesHomeForTest 把 hermes 家指到临时目录,返回还原函数。
func setHermesHomeForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	return dir
}

func TestSetReasoningEffortCreatesAgentSection(t *testing.T) {
	home := setHermesHomeForTest(t)
	if err := SetReasoningEffort("high"); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(home, "config.yaml"))
	got := string(raw)
	if !strings.Contains(got, "agent:") || !strings.Contains(got, "reasoning_effort: high") {
		t.Fatalf("应写入 agent.reasoning_effort= high,得到:\n%s", got)
	}
}

func TestSetReasoningEffortReplacesExistingKey(t *testing.T) {
	home := setHermesHomeForTest(t)
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("model:\n  default: deepseek-flash\nagent:\n  reasoning_effort: low\n"), 0o644)
	if err := SetReasoningEffort("xhigh"); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(cfg)
	got := string(raw)
	if strings.Contains(got, "reasoning_effort: low") {
		t.Fatalf("旧值 low 应被替换:\n%s", got)
	}
	if !strings.Contains(got, "reasoning_effort: xhigh") || !strings.Contains(got, "default: deepseek-flash") {
		t.Fatalf("新值 xhigh 与 model 段应共存:\n%s", got)
	}
}

func TestSetReasoningEffortEmptyRemovesKey(t *testing.T) {
	home := setHermesHomeForTest(t)
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("agent:\n  reasoning_effort: high\n"), 0o644)
	if err := SetReasoningEffort(""); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(cfg)
	if strings.Contains(string(raw), "reasoning_effort") {
		t.Fatalf("空 effort 应删键回默认,得到:\n%s", string(raw))
	}
}

func TestIsValidReasoningEffort(t *testing.T) {
	for _, ok := range []string{"minimal", "low", "medium", "high", "xhigh", "max", "ultra", "HIGH"} {
		if !IsValidReasoningEffort(ok) {
			t.Errorf("IsValidReasoningEffort(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "hgih", "extreme"} {
		if IsValidReasoningEffort(bad) {
			t.Errorf("IsValidReasoningEffort(%q) = true, want false", bad)
		}
	}
}

// ===== 红线 2 判决区(2026-10-17:P0-1 根治)=====
//
// 判决目标:把「写用户家目录」这条路**彻底焊死**。用户显式把家目录指到临时
// 目录后调用 SetReasoningEffort,必须报错且该目录树**一个字节都不许变**。
// 这组断言是本波最重要的判决:犯事的那条分支原先零覆盖(每个旧用例都
// t.Setenv("HERMES_HOME")),所以回落家目录的写法能一直活着。

// sandboxUserHome 把「用户家目录」指到临时目录并清空 HERMES_HOME,返回临时
// 家目录。Windows 的 os.UserHomeDir 读 %USERPROFILE%,类 Unix 读 $HOME ——
// 两个都设,免得测试只在一种平台上有判决力(本仓主战场是 Windows)。
func sandboxUserHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HERMES_HOME", "") // 未设/空白 = 不允许有写入落点
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	return home
}

// treeFiles 列目录树下全部文件的相对路径(排序,便于比对前后)。
func treeFiles(t *testing.T, root string) []string {
	t.Helper()
	var got []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("扫目录树 %s: %v", root, err)
	}
	sort.Strings(got)
	return got
}

// 【红线判决】HERMES_HOME 未设 + 用户家目录指向临时目录:必须返回错误,
// 且家目录树前后**逐字节不变**(既不许写 ~/.hermes/config.yaml,也不许留
// .tmp,也不许建 .hermes 目录本身)。
func TestSetReasoningEffortRefusesToWriteUserHome(t *testing.T) {
	home := sandboxUserHome(t)

	// 前置:用户自己手写的 hermes 配置必须**原样保留**(读面可见、写面禁碰)。
	userCfg := filepath.Join(home, ".hermes", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(userCfg), 0o755); err != nil {
		t.Fatalf("预置用户配置: %v", err)
	}
	const userOriginal = "model:\n  default: deepseek-chat\nagent:\n  reasoning_effort: low\n"
	if err := os.WriteFile(userCfg, []byte(userOriginal), 0o644); err != nil {
		t.Fatalf("预置用户配置: %v", err)
	}
	before := treeFiles(t, home)

	// 自证留痕(-v 可见):落点决策 + 前后家目录树 + 原样上抛的错误。
	t.Logf("【自证】HERMES_HOME=%q → 写面 WritableConfigPath()=%q(空=无合法落点),读面 ConfigPath()=%q",
		os.Getenv("HERMES_HOME"), WritableConfigPath(), ConfigPath())
	t.Logf("【自证】调用前用户家树: %v", before)

	err := SetReasoningEffort("high")
	t.Logf("【自证】调用后 err = %v", err)
	t.Logf("【自证】调用后用户家树: %v", treeFiles(t, home))
	if err == nil {
		t.Fatalf("HERMES_HOME 未设时必须拒写(红线 2),却返回成功;家目录树:\n%s",
			strings.Join(treeFiles(t, home), "\n"))
	}
	if !errors.Is(err, errNoHermesHome) {
		t.Errorf("应返回 errNoHermesHome,实际 %T: %v", err, err)
	}
	// 错误文案要说清"为什么不写"和"怎么恢复",否则透给用户等于没解释。
	msg := err.Error()
	for _, want := range []string{"HERMES_HOME", "~/.hermes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误文案应含 %q(让手机端能如实解释),实际 %q", want, msg)
		}
	}

	after := treeFiles(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("用户家目录树被改动(红线 2 违反)\n调用前: %v\n调用后: %v", before, after)
	}
	raw, readErr := os.ReadFile(userCfg)
	if readErr != nil {
		t.Fatalf("用户配置读不到了: %v", readErr)
	}
	if string(raw) != userOriginal {
		t.Errorf("用户配置被改写\n期望:\n%s\n实际:\n%s", userOriginal, raw)
	}
}

// 【红线判决·无残留】HERMES_HOME 未设时连一个字节都不许落盘:既没有
// ~/.hermes/config.yaml,也没有 config.yaml.tmp(失败路径不许留残骸)。
// 两个子场景都要盖:空家目录(不许凭空建目录/文件)与已有用户配置
// (旧回落写法在这里会真的写成功,断言才非空洞 —— 变异验证过)。
func TestSetReasoningEffortLeavesNoTmpWhenRefused(t *testing.T) {
	t.Run("空家目录不留痕", func(t *testing.T) {
		home := sandboxUserHome(t)
		if err := SetReasoningEffort("high"); err == nil {
			t.Fatal("HERMES_HOME 未设时应拒写")
		}
		for _, p := range []string{
			filepath.Join(home, ".hermes", "config.yaml"),
			filepath.Join(home, ".hermes", "config.yaml.tmp"),
		} {
			if _, err := os.Stat(p); err == nil {
				t.Errorf("失败路径留下了文件(红线 2 违反): %s", p)
			} else if !os.IsNotExist(err) {
				t.Errorf("stat %s: %v", p, err)
			}
		}
		// 整棵树(目录也算)必须仍是空 —— 哪怕是 mkdir 出来的 .hermes
		// 目录也算"daemon 动了用户家"。
		if got := treeFiles(t, home); len(got) != 0 {
			t.Errorf("家目录树出现了文件(红线 2 违反): %v", got)
		}
		if ents, err := os.ReadDir(home); err != nil {
			t.Fatalf("读家目录: %v", err)
		} else if len(ents) != 0 {
			t.Errorf("家目录下出现了条目(红线 2 违反): %v", ents)
		}
	})

	t.Run("已有用户配置无残骸", func(t *testing.T) {
		home := sandboxUserHome(t)
		cfg := filepath.Join(home, ".hermes", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
			t.Fatalf("预置用户配置: %v", err)
		}
		if err := os.WriteFile(cfg, []byte("agent:\n  reasoning_effort: low\n"), 0o644); err != nil {
			t.Fatalf("预置用户配置: %v", err)
		}
		if err := SetReasoningEffort("high"); err == nil {
			t.Fatal("HERMES_HOME 未设时应拒写")
		}
		if _, err := os.Stat(cfg + ".tmp"); err == nil {
			t.Errorf("失败路径残留 config.yaml.tmp: %s.tmp", cfg)
		}
		if got := treeFiles(t, home); strings.Join(got, "\n") != ".hermes/config.yaml" {
			t.Errorf("家目录树除用户自己的 config.yaml 外应无增删,实际 %v", got)
		}
		raw, err := os.ReadFile(cfg)
		if err != nil || string(raw) != "agent:\n  reasoning_effort: low\n" {
			t.Errorf("用户配置被改写: %v\n%s", err, raw)
		}
	})
}

// 【读放宽不回归】读面仍回落用户家 —— status/查询面要看到用户真实配置。
// 这一条钉住"读放宽、写收紧"的分工:防后人顺手把读也砍了(那是另一个
// 行为变更,得显式决策,不能夹带)。
func TestConfigPathStillFallsBackToUserHomeForRead(t *testing.T) {
	home := sandboxUserHome(t)

	got := ConfigPath()
	want := filepath.Join(home, ".hermes", "config.yaml")
	if got != want {
		t.Errorf("读面 ConfigPath() = %q,期望回落用户家 %q(读放宽)", got, want)
	}
	// 同一时刻,写面必须拒绝给落点 —— 一读一写,规则不同。
	if w := WritableConfigPath(); w != "" {
		t.Errorf("写面 WritableConfigPath() = %q,HERMES_HOME 未设时应为空(绝不回落用户家)", w)
	}
}

// HERMES_HOME 已设时,读写两面同址(hermes 自己的家目录,非用户桌面配置面)。
func TestWritableConfigPathUsesHermesHome(t *testing.T) {
	home := setHermesHomeForTest(t)
	want := filepath.Join(home, "config.yaml")
	if got := ConfigPath(); got != want {
		t.Errorf("ConfigPath() = %q,期望 %q", got, want)
	}
	if got := WritableConfigPath(); got != want {
		t.Errorf("WritableConfigPath() = %q,期望 %q", got, want)
	}
}

// 空白串 = 未设:写面不得把 "   " 当成合法家目录(否则会写到相对当前目录的
// 垃圾路径上)。
func TestWritableConfigPathRejectsBlankHermesHome(t *testing.T) {
	t.Setenv("HERMES_HOME", "   ")
	t.Setenv("USERPROFILE", t.TempDir())
	if got := WritableConfigPath(); got != "" {
		t.Errorf("空白 HERMES_HOME 时 WritableConfigPath() = %q,应为空", got)
	}
	if err := SetReasoningEffort("high"); !errors.Is(err, errNoHermesHome) {
		t.Errorf("空白 HERMES_HOME 时应拒写并返回 errNoHermesHome,实际 %v", err)
	}
}

// 写入成功路径不许留 config.yaml.tmp(rename 消费掉临时文件)。
func TestSetReasoningEffortLeavesNoTmpOnSuccess(t *testing.T) {
	home := setHermesHomeForTest(t)
	if err := SetReasoningEffort("high"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "config.yaml.tmp")); err == nil {
		t.Errorf("写入后残留 config.yaml.tmp")
	}
	if got := treeFiles(t, home); strings.Join(got, "\n") != "config.yaml" {
		t.Errorf("HERMES_HOME 沙箱里应只有 config.yaml,实际 %v", got)
	}
}

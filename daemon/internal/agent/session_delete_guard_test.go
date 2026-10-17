package agent

// session_delete_guard_test.go —— 共享删除闸门本身的钉子。
//
// 【钉的是什么】pi 与 codex 的会话文件删除**共用**一条闸门序列(解析 → 包含性 →
// 扩展名 → 文件头 id)。共用的价值全在"只有一份顺序":一旦某家绕开闸门直接
// os.Remove,不对称就会重新长出来。故这里钉三件事:
//  1. 四道闸的**顺序**(前后闸同时不通过时,报的是靠前那一道);
//  2. pi 侧既有错误文案逐字未变(闸门抽出来了,对外可观察的措辞不能漂);
//  3. 目标已不在 ⇒ 幂等成功(不是故障)。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gateFixture 落一份"哪一闸都不该拦它"的文件,再由用例按需把某一道闸的前提破坏掉。
func gateFixture(t *testing.T, dir, name, headID string) string {
	t.Helper()
	body := `{"type":"session","id":"` + headID + `","cwd":"G:\\项目1"}` + "\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写 fixture 失败: %v", err)
	}
	return path
}

// TestNativeSessionDeleteGateOrder 顺序钉子:同时破坏两道闸时,必须由**靠前**的那道
// 拦下(否则"闸门被绕过"这类改动可能悄悄换一种顺序继续存在)。
func TestNativeSessionDeleteGateOrder(t *testing.T) {
	head := func(path string) string {
		var info PiSessionInfo
		scanPiSessionHead(path, &info)
		return info.SessionID
	}

	t.Run("包含性先于扩展名", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		// 目录外 + 扩展名也不对:包含性在前,报包含性。
		p := gateFixture(t, outside, "note.txt", "sid")
		err := deleteNativeSessionFile(root, p, "sid", "Pi", head)
		if err == nil || !strings.Contains(err.Error(), "不在 sessions 目录中") {
			t.Fatalf("应报包含性, got %v", err)
		}
	})

	t.Run("扩展名先于身份", func(t *testing.T) {
		root := t.TempDir()
		// 目录内 + 扩展名不对 + 头 id 也不对:扩展名在前,报扩展名。
		p := gateFixture(t, root, "note.txt", "other")
		err := deleteNativeSessionFile(root, p, "sid", "Pi", head)
		if err == nil || !strings.Contains(err.Error(), "不是 Pi 会话文件") {
			t.Fatalf("应报扩展名, got %v", err)
		}
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("被拦下的文件不该消失: %v", statErr)
		}
	})

	t.Run("目录本身不是会话文件", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "sid.jsonl")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := deleteNativeSessionFile(root, sub, "sid", "Pi", head); err == nil {
			t.Fatal("rel == \".\"(=sessions 目录本身)必须被拒")
		}
	})
}

// TestDeletePiSessionKeepsLegacyOutcome 闸门抽成共享函数后,pi 侧的既有可观察结果
// 一条都不许变:三条拒绝文案逐字保留,目标已不在仍是幂等成功。
func TestDeletePiSessionKeepsLegacyOutcome(t *testing.T) {
	head := func(path string) string {
		var info PiSessionInfo
		scanPiSessionHead(path, &info)
		return info.SessionID
	}

	if err := deletePiSessionFrom(t.TempDir(), "unused", "sid"); err != nil {
		t.Fatalf("空路径(线程还没建过原生会话)应幂等成功, got %v", err)
	}

	root := t.TempDir()
	other := t.TempDir()
	inside := gateFixture(t, root, "session.jsonl", "sid")

	cases := []struct {
		name string
		call func() error
		want string
	}{
		{"目录外", func() error { return deletePiSessionFrom(other, inside, "sid") }, "Pi 会话路径不在 sessions 目录中"},
		{"扩展名不对", func() error {
			return deleteNativeSessionFile(root, gateFixture(t, root, "session.txt", "sid"), "sid", "Pi", head)
		}, "不是 Pi 会话文件"},
		{"头 id 不一致", func() error { return deletePiSessionFrom(root, inside, "other") }, "Pi 会话 ID 与文件头不一致，未删除"},
		{"头 id 读不到", func() error {
			return deleteNativeSessionFile(root, gateFixture(t, root, "no-head.jsonl", ""), "sid", "Pi", head)
		}, "Pi 会话 ID 与文件头不一致，未删除"},
		{"请求 id 为空", func() error { return deletePiSessionFrom(root, inside, "") }, "Pi 会话 ID 与文件头不一致，未删除"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.call()
			if err == nil || err.Error() != c.want {
				t.Fatalf("文案漂了: got %v, want %q", err, c.want)
			}
		})
	}
	if _, err := os.Stat(inside); err != nil {
		t.Fatalf("被拒的文件不该消失: %v", err)
	}

	// 目标已不存在 ⇒ 幂等成功(用户在桌面端先清掉了)。
	if err := deletePiSessionFrom(root, filepath.Join(root, "gone.jsonl"), "sid"); err != nil {
		t.Fatalf("目标已不在应幂等成功, got %v", err)
	}
}

// TestDeleteNativeSessionFileReaderAdapterStaysStrict 第 ④ 道改成可注入"判定"之后,
// 读取器版入口(deleteNativeSessionFile,pi 走它)必须仍是**严格相等**,不许顺手把
// 约定分支漏进 pi 这边。同时钉住:请求 id 为空时闸门**根本不调用**注入的判定 ——
// 否则一个"什么都返回 true"的判定就能把空 id 放过去,fail closed 就漏了。
func TestDeleteNativeSessionFileReaderAdapterStaysStrict(t *testing.T) {
	head := func(path string) string {
		var info PiSessionInfo
		scanPiSessionHead(path, &info)
		return info.SessionID
	}

	root := t.TempDir()
	// 文件头写 sid_<子>,文件名的 id 段也正是 sid_<子>(codex 分叉那种形态)。
	forkish := gateFixture(t, root, "rollout-2026-10-11T12-45-22-sid_child.jsonl", "sid")
	if err := deleteNativeSessionFile(root, forkish, "sid_child", "Pi", head); err == nil {
		t.Fatal("pi 走读取器版入口时必须是严格相等,不许因 codex 分叉约定而放行")
	}
	if _, err := os.Stat(forkish); err != nil {
		t.Fatalf("被拒的文件不该消失: %v", err)
	}

	// 请求 id 为空 ⇒ 判定一次都不许被调用。
	called := false
	permissive := func(string, string) bool { called = true; return true }
	p := gateFixture(t, root, "session.jsonl", "sid")
	if err := deleteNativeSessionFileWithIdentity(root, p, "", "Pi", permissive); err == nil {
		t.Fatal("请求 id 为空时必须拒删,哪怕判定放行")
	}
	if called {
		t.Fatal("请求 id 为空时不该调用身份判定(fail closed 不许有洞)")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("空请求 id 时文件仍被删了: %v", err)
	}
	// 判定返回 false ⇒ 拒删(且文件还在)。
	never := func(string, string) bool { return false }
	if err := deleteNativeSessionFileWithIdentity(root, p, "sid", "Pi", never); err == nil {
		t.Fatal("身份判定不认时必须拒删")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("判定拒删后文件却消失了: %v", err)
	}
}

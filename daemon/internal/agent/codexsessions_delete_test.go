package agent

// codexsessions_delete_test.go —— codex 会话文件删除的四道闸 + 唯一命中。
//
// 【钉的是什么】pi 侧删会话文件有四道闸(解析/包含性/扩展名/文件头 id),codex 侧
// 曾经只剩"文件名后缀像"一道,首个命中就删 —— 守卫强度不对称是本用例群的由来。
// 现在 codex 走共享闸门(session_delete_guard.go),本文件把每一道闸的判决点各自
// 钉死,**重点是最上面那条:多命中必须报错,而不是挑一个删**。
//
// 【最重要的判决】命中多于一个时 DeleteCodexSessionFrom 必须报错,且**两个文件都
// 还在**。挑一个删 = 猜,猜错就是另一个会话的完整历史不可恢复地没了。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const codexDeleteTestID = "01a12946-6c10-72d3-a7df-6207720e57cb"

// writeCodexRollout 落一份 rollout fixture:headID 非空时首行是真实形状的
// session_meta(2026-10-11 实测结构:payload 里 id 与 session_id 同值);headID
// 为空时首行换成 event_msg —— 模拟"文件头读不到身份"的老/损坏 rollout。
func writeCodexRollout(t *testing.T, dir, name, headID string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	head := `{"timestamp":"2026-10-11T04:44:05.178Z","ordinal":0,"type":"event_msg","payload":{"type":"task_started"}}`
	if headID != "" {
		head = `{"timestamp":"2026-10-11T04:44:05.178Z","ordinal":0,"type":"session_meta","payload":{"session_id":"` +
			headID + `","id":"` + headID + `","cwd":"G:\\项目1","cli_version":"0.162.0"}}`
	}
	body := head + "\n" + `{"timestamp":"2026-10-11T04:44:06.000Z","ordinal":1,"type":"response_item","payload":{"type":"message","id":"msg_x","role":"user","content":[{"type":"input_text","text":"你好"}]}}` + "\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("写 rollout fixture 失败: %v", err)
	}
	return path
}

func rolloutName(id string) string {
	return "rollout-2026-10-11T04-44-05-" + id + ".jsonl"
}

// TestDeleteCodexSessionRejectsAmbiguousHits 是本文件最重要的判决点:两个同名尾巴
// 的 rollout 都在时,删除必须失败,且两个文件一个都不许少。
func TestDeleteCodexSessionRejectsAmbiguousHits(t *testing.T) {
	root := t.TempDir()
	a := writeCodexRollout(t, filepath.Join(root, "2026", "10", "11"), rolloutName(codexDeleteTestID), codexDeleteTestID)
	b := writeCodexRollout(t, filepath.Join(root, "2026", "10", "12"), rolloutName(codexDeleteTestID), codexDeleteTestID)

	err := deleteCodexSessionFrom(root, codexDeleteTestID)
	if err == nil {
		t.Fatal("多个 rollout 命中同一 id 时必须报错,而不是挑一个删")
	}
	// "有多个候选"与"没有这条会话"是相反的事实:不能伪装成 ErrCodexSessionNotFound
	// (调用方据此区分"提示用户会话已删"与"按还在处理")。
	if errors.Is(err, ErrCodexSessionNotFound) {
		t.Fatalf("多命中错误不该包 ErrCodexSessionNotFound: %v", err)
	}
	for _, p := range []string{a, b} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("多命中时不得删任何文件,却少了 %s: %v", p, statErr)
		}
	}

	// 定位侧同样按"不猜"处理:命中多于一个直接报错,不返回任一路径。
	if _, err := findCodexSessionFrom(root, codexDeleteTestID); err == nil {
		t.Fatal("findCodexSessionFrom 命中多个时必须报错")
	}
}

// TestDeleteCodexSessionDeletesOnlyTheExactRollout 正常路径不回归:唯一命中且头
// 里 id 一致 ⇒ 真删;删完再删是幂等成功(找不到按哨兵包装,上层据此当"已经没有了")。
func TestDeleteCodexSessionDeletesOnlyTheExactRollout(t *testing.T) {
	root := t.TempDir()
	target := writeCodexRollout(t, filepath.Join(root, "2026", "10", "11"), rolloutName(codexDeleteTestID), codexDeleteTestID)
	other := writeCodexRollout(t, filepath.Join(root, "2026", "10", "11"), rolloutName("01a12fde-1c84-7953-8eff-9c70e525a4ad"), "01a12fde-1c84-7953-8eff-9c70e525a4ad")

	got, err := findCodexSessionFrom(root, codexDeleteTestID)
	if err != nil {
		t.Fatalf("唯一命中应定位成功: %v", err)
	}
	if got != target {
		t.Fatalf("定位到 %q,期望 %q", got, target)
	}
	if err := deleteCodexSessionFrom(root, codexDeleteTestID); err != nil {
		t.Fatalf("唯一命中 + 头 id 一致应删成功: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("目标 rollout 未被删除: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("无关会话的 rollout 被动到了: %v", err)
	}
	if err := deleteCodexSessionFrom(root, codexDeleteTestID); !errors.Is(err, ErrCodexSessionNotFound) {
		t.Fatalf("删完再删应是幂等成功(哨兵包装), got %v", err)
	}
	if err := deleteCodexSessionFrom(root, "   "); err == nil {
		t.Fatal("空 id 必须被守卫拒绝")
	}
}

// TestDeleteCodexSessionRejectsOutsideSessionsDir 包含性闸(第 ② 道):候选文件落在
// sessions 目录之外 ⇒ 拒删,目录外那个文件必须还在。fixture 刻意做成"其它三道闸
// 全过"的形状(正确扩展名 + 头 id 与请求一致),这样它只有在第 ② 道闸生效时才
// 活下来 —— 闸门写反或漏掉都会立刻暴露。
func TestDeleteCodexSessionRejectsOutsideSessionsDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := writeCodexRollout(t, outside, rolloutName(codexDeleteTestID), codexDeleteTestID)

	if err := deleteNativeSessionFile(root, outsideFile, codexDeleteTestID, "codex", codexRolloutSessionID); err == nil {
		t.Fatal("sessions 目录外的文件必须被包含性闸拒删")
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("目录外的文件被删了: %v", err)
	}
}

// TestDeleteCodexSessionRejectsSymlinkedRolloutOutsideDir 包含性闸的**穿越**那一路:
// sessions 目录内放一个指向目录外的符号链接,链接名照样以 -<id>.jsonl 结尾(能通过
// 按名定位那一关),只有第 ② 道闸能挡住。链接本身能不能建取决于系统权限(Windows
// 需开发者模式/管理员),建不了就如实跳过,不做假装跑过的假绿。
func TestDeleteCodexSessionRejectsSymlinkedRolloutOutsideDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := writeCodexRollout(t, outside, rolloutName(codexDeleteTestID), codexDeleteTestID)

	link := filepath.Join(root, rolloutName(codexDeleteTestID))
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skipf("本机不能建符号链接,跳过链接穿越这一路: %v", err)
	}
	if err := deleteCodexSessionFrom(root, codexDeleteTestID); err == nil {
		t.Fatal("经符号链接落到 sessions 目录外的 rollout 必须被拒删")
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("经链接指向的目录外文件被删了: %v", err)
	}
	if _, err := os.Stat(link); err != nil {
		t.Fatalf("链接本身被删了: %v", err)
	}
}

// TestDeleteCodexSessionRejectsNonJSONL 扩展名闸(第 ③ 道)。按名定位那一关本来就
// 只认 -<id>.jsonl,所以这一道是"定位漏了也不至于删错东西"的第二层防线,直接对
// 闸门发起调用来钉。
func TestDeleteCodexSessionRejectsNonJSONL(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")
	notJSONL := writeCodexRollout(t, dir, "rollout-2026-10-11T04-44-05-"+codexDeleteTestID+".txt", codexDeleteTestID)
	log := writeCodexRollout(t, dir, "codex-10-11.log", codexDeleteTestID)

	for _, p := range []string{notJSONL, log} {
		if err := deleteNativeSessionFile(root, p, codexDeleteTestID, "codex", codexRolloutSessionID); err == nil {
			t.Fatalf("非 .jsonl 文件必须被扩展名闸拒删: %s", p)
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("非会话文件被删了 %s: %v", p, err)
		}
	}
	// 文件名像会话但实际是目录的情况:闸门只对文件 os.Remove,目录不该被牵连。
	sub := filepath.Join(dir, rolloutName(codexDeleteTestID)+".d")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := deleteNativeSessionFile(root, sub, codexDeleteTestID, "codex", codexRolloutSessionID); err == nil {
		t.Fatal("目录不该通过删除闸(会话文件一定是文件)")
	}
	if _, err := os.Stat(sub); err != nil {
		t.Fatalf("目录被删了: %v", err)
	}
}

// TestDeleteCodexSessionRejectsHeadIDMismatch 身份闸(第 ④ 道):头里的 id 与请求
// id 不一致,或头里压根读不到 id(老版本/损坏 rollout),一律拒删 —— 无法证明 = 不删。
func TestDeleteCodexSessionRejectsHeadIDMismatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")
	mismatch := writeCodexRollout(t, dir, rolloutName(codexDeleteTestID), "01a12fde-1c84-7953-8eff-9c70e525a4ad")
	noHead := writeCodexRollout(t, dir, rolloutName("01a13061-9dc4-7ac2-a5b9-d9ffa9bccbb2"), "")

	if err := deleteNativeSessionFile(root, mismatch, codexDeleteTestID, "codex", codexRolloutSessionID); err == nil {
		t.Fatal("文件头 id 与请求 id 不一致时必须拒删")
	}
	if _, err := os.Stat(mismatch); err != nil {
		t.Fatalf("头 id 不一致的文件被删了: %v", err)
	}
	if err := deleteNativeSessionFile(root, noHead, "01a13061-9dc4-7ac2-a5b9-d9ffa9bccbb2", "codex", codexRolloutSessionID); err == nil {
		t.Fatal("文件头读不到身份时必须拒删(fail closed)")
	}
	if _, err := os.Stat(noHead); err != nil {
		t.Fatalf("读不到文件头 id 的文件被删了: %v", err)
	}
	// 请求 id 为空 = 没有可校验的身份,同样不删。
	good := writeCodexRollout(t, dir, rolloutName("01a12a7f-175f-7a20-a27c-94beae4b517c"), "01a12a7f-175f-7a20-a27c-94beae4b517c")
	if err := deleteNativeSessionFile(root, good, "", "codex", codexRolloutSessionID); err == nil {
		t.Fatal("请求 id 为空时不得删任何文件")
	}
	if _, err := os.Stat(good); err != nil {
		t.Fatalf("请求 id 为空时文件仍被删了: %v", err)
	}
}

// TestCodexRolloutSessionIDOnlyTrustsSessionMeta 头解析本身:只有 session_meta 行
// 的 payload.id 算数;response_item 里的 msg_xxx 不是会话 id(否则第 ④ 道闸形同
// 虚设),坏行跳过不吞整份文件。
func TestCodexRolloutSessionIDOnlyTrustsSessionMeta(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")

	noMeta := writeCodexRollout(t, dir, "rollout-a.jsonl", "")
	if got := codexRolloutSessionID(noMeta); got != "" {
		t.Fatalf("没有 session_meta 行时应读不到 id, got %q", got)
	}
	withMeta := writeCodexRollout(t, dir, "rollout-b.jsonl", codexDeleteTestID)
	if got := codexRolloutSessionID(withMeta); got != codexDeleteTestID {
		t.Fatalf("session_meta 行的 payload.id 应读出 %q, got %q", codexDeleteTestID, got)
	}
	// 头被挪到若干行之后也要读得到(留了 codexRolloutHeadLineCap 的余量)。
	lines := []string{`{"type":"event_msg","payload":{"type":"task_started"}}`}
	for i := 0; i < 10; i++ {
		lines = append(lines, `{"type":"event_msg","payload":{"type":"token_count"}}`)
	}
	lines = append(lines, `{"type":"session_meta","payload":{"id":"`+codexDeleteTestID+`"}}`)
	late := filepath.Join(dir, "rollout-c.jsonl")
	if err := os.WriteFile(late, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := codexRolloutSessionID(late); got != codexDeleteTestID {
		t.Fatalf("靠后的 session_meta 行也应读出 id, got %q", got)
	}
}

// ---- 分叉/续写 rollout 的身份判定(2026-10-17 fork 删除行为差处置)----

// 真实分叉文件形态(2026-10-17 本机逐份实测):文件名 rollout-<时间>-<父>_<子>.jsonl,
// 文件头 session_meta.payload.id 存的是**父** id;PocketAgent 侧 CodexThreadID 存的是
// 名字那一整段 `<父>_<子>`。常量取自本机真实 rollout。
const (
	codexForkTestParent = "01a12946-6c10-72d3-a7df-6207720e57cb"
	codexForkTestChild  = "01a12947-a240-71f1-80b5-248916ae60ff"
	codexForkTestID     = codexForkTestParent + "_" + codexForkTestChild
)

func forkRolloutName(id string) string {
	return "rollout-2026-10-11T12-45-22-" + id + ".jsonl"
}

// TestDeleteCodexSessionDeletesForkShapedRollout 主路径不许静默失效:导入一条电脑上
// 的分叉会话(CodexThreadID = `<父>_<子>`)再删掉它,必须真把那份 rollout 删掉。
// 同时钉住"删分叉绝不能连带删掉父会话自己那份"—— 两份文件的文件头 id 是同一个,
// 唯一把它们区分开的只有文件名,这条判决正是本改动的全部风险面。
func TestDeleteCodexSessionDeletesForkShapedRollout(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")
	fork := writeCodexRollout(t, dir, forkRolloutName(codexForkTestID), codexForkTestParent)
	parent := writeCodexRollout(t, dir, rolloutName(codexForkTestParent), codexForkTestParent)

	got, err := findCodexSessionFrom(root, codexForkTestID)
	if err != nil {
		t.Fatalf("分叉形态应能按名定位(后缀 -<父>_<子>.jsonl 命中): %v", err)
	}
	if got != fork {
		t.Fatalf("定位到 %q,期望 %q", got, fork)
	}
	if err := deleteCodexSessionFrom(root, codexForkTestID); err != nil {
		t.Fatalf("分叉形态删除必须成功(否则导入的会话永远删不掉): %v", err)
	}
	if _, err := os.Stat(fork); !os.IsNotExist(err) {
		t.Fatalf("目标分叉 rollout 未被删除: %v", err)
	}
	if _, err := os.Stat(parent); err != nil {
		t.Fatalf("删分叉时把父会话自己的 rollout 也删了(串删): %v", err)
	}
	// 反向:请求父 id 时只删父会话那份;分叉那份不是候选(它的名字以 _<子> 结尾)。
	if err := deleteCodexSessionFrom(root, codexForkTestParent); err != nil {
		t.Fatalf("父会话删除必须成功: %v", err)
	}
}

// TestDeleteCodexSessionRejectsForkShapeWithoutMatchingHead 放行的是 codex 的命名
// 约定,不是"凡是带下划线的名字都信":分叉形态的文件头必须是**父段**那个 id。
// 头里是第三个 id(改名/复制/同步盘残留)、头读不到身份、或头里正好是"子"段,
// 一律拒删,且文件必须留在盘上。
func TestDeleteCodexSessionRejectsForkShapeWithoutMatchingHead(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")
	wrongHead := writeCodexRollout(t, dir, forkRolloutName(codexForkTestID), "01a12fde-1c84-7953-8eff-9c70e525a4ad")
	noHead := writeCodexRollout(t, dir, forkRolloutName(codexForkTestParent+"_"+codexForkTestChild+"_x"), "")
	childHead := writeCodexRollout(t, dir, forkRolloutName(codexForkTestParent+"_"+codexForkTestChild+"_y"), codexForkTestChild)

	for _, p := range []string{wrongHead, noHead, childHead} {
		nameID := codexRolloutNameID(filepath.Base(p))
		if err := deleteNativeSessionFileWithIdentity(root, p, nameID, "codex", codexRolloutIdentityMatches); err == nil {
			t.Fatalf("分叉形态但文件头对不上时必须拒删: %s", filepath.Base(p))
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("被拒的分叉形态文件被删了 %s: %v", filepath.Base(p), err)
		}
	}
}

// TestDeleteCodexSessionIdentityIsStricterThanNameLookup 名字定位是后缀匹配(够"像"
// 即可),身份判定是**全等** —— 三段畸形名只让后两段命中定位时,身份判定必须拒删。
// 钉住"第 ④ 道不比定位松":放宽只发生在已证实的两段分叉形态上。
func TestDeleteCodexSessionIdentityIsStricterThanNameLookup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "11")
	// 文件名 id 段是 `aaa-b_y_<子>`(三段),请求 `b_y_<子>` —— 后缀 -b_y_<子>.jsonl
	// 恰好命中,但请求的不是文件名那一整段。
	name := "rollout-2026-10-11T12-45-22-aaa-b_y_" + codexForkTestChild + ".jsonl"
	p := writeCodexRollout(t, dir, name, codexForkTestChild)

	if _, err := findCodexSessionFrom(root, "b_y_"+codexForkTestChild); err != nil {
		t.Fatalf("后缀定位应命中: %v", err)
	}
	if err := deleteCodexSessionFrom(root, "b_y_"+codexForkTestChild); err == nil {
		t.Fatal("请求 id 与文件名整段不等时必须拒删")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("被拒的三段畸形名文件被删了: %v", err)
	}
}

// TestCodexRolloutNameIDIsTheSingleSourceOfTruth 钉住"列表页展示 / 导入落库 / 删除
// 校验"三处共用同一个 id 解析:用的都是本机真实文件名形态(含分叉复合形态),
// 判不出来的一律空串(调用方据此不入列表、不删)。
func TestCodexRolloutNameIDIsTheSingleSourceOfTruth(t *testing.T) {
	cases := []struct{ name, want string }{
		{"rollout-2026-10-11T12-44-03-" + codexDeleteTestID + ".jsonl", codexDeleteTestID},
		{"rollout-2026-10-11T12-45-22-" + codexForkTestID + ".jsonl", codexForkTestID},
		{"codex-10-11.log", ""},
		{"rollout-2026-10-11T12-44-03-.jsonl", ""},
		{"rollout-short.jsonl", ""},
	}
	for _, c := range cases {
		if got := codexRolloutNameID(c.name); got != c.want {
			t.Fatalf("codexRolloutNameID(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

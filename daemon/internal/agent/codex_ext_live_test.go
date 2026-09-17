// codex_ext_live_test.go —— 对**真实 codex** 的实测(默认跳过)。
//
// 【为什么单测之外还需要它】codex_guardian_test.go 钉的是"我们的转写表",
// 而那张表本身是对着 codex 试出来的:协议里 event 是无类型透传,字段名一个字
// 都没写。所以单测全绿只能证明"我们内部自洽",证明不了"codex 认"。
// 这个文件补上后半段:把转写结果真的发给 codex,看它接不接。
//
// 运行方式(需要本机装有 codex,并且已经登录):
//
//	PA_LIVE_CODEX=1 go test ./internal/agent/ -run Live -v
//
// 默认跳过,因为:①它依赖外部程序与登录态,CI/别的机器上会红;
// ②它会**真的在用户的 codex 里创建并归档一个线程**。
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
)

func liveCodex(t *testing.T) *CodexProvider {
	t.Helper()
	if os.Getenv("PA_LIVE_CODEX") == "" {
		t.Skip("跳过实测(设 PA_LIVE_CODEX=1 开启;需要本机 codex 已登录)")
	}
	p := NewCodexProvider()
	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		t.Fatalf("拉不起 codex app-server: %v", err)
	}
	return p
}

// liveCwd 给新线程一个工作目录。
//
// 【不能用 t.TempDir()】codex 会把会话的工作目录一直握着(rollout 里记着 cwd,
// 子进程也还在跑),测试结束时 Go 的 TempDir 清理会报
// "The process cannot access the file because it is being used by another process",
// 而 **TempDir 清理失败会让整个测试判 FAIL** —— 于是接入明明是好的,报告却是红的。
// 这里用一个固定目录,故意不清理。
func liveCwd(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "pa-live-probe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("准备探针工作目录失败: %v", err)
	}
	return dir
}

// newLiveThread 起一个空线程(thread/start 不消耗模型额度,只是落一条 rollout)。
func newLiveThread(t *testing.T, p *CodexProvider) string {
	t.Helper()
	tid := ""
	err := p.withClient(func(c *codexapp.Client) error {
		raw, err := c.Call("thread/start", map[string]interface{}{"cwd": liveCwd(t)})
		if err != nil {
			return err
		}
		var res struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		tid = res.Thread.ID
		return nil
	})
	if err != nil || tid == "" {
		t.Fatalf("建线程失败: %v (tid=%q)", err, tid)
	}
	return tid
}

// ---- 只读面 ----

func TestLiveCodexReadOnlySurface(t *testing.T) {
	p := liveCodex(t)

	// 用量:拉不到不是失败(未登录/地区不支持都算正常),但**必须能区分**
	// "读不到"与"额度为 0" —— 这是这个接口存在的全部意义。
	usage := p.Usage()
	if usage.FetchedAtUnixMs == 0 {
		t.Error("Usage() 没有填拉取时间")
	}
	if usage.Error != "" {
		t.Logf("用量读不到(可接受): %s", usage.Error)
	} else {
		t.Logf("用量: plan=%q primary=%d%% secondary=%d%% credits=%v",
			usage.PlanType, usage.Primary.GetUsedPercent(), usage.Secondary.GetUsedPercent(), usage.HasCredits)
	}

	req := p.Requirements()
	// 没配托管策略时必须是"无限制"(三个白名单都空),否则手机端会错把选项藏起来。
	if !req.Managed && (len(req.AllowedApprovalPolicies) > 0 || len(req.AllowedSandboxModes) > 0) {
		t.Error("未托管却给出了非空白名单:会把手机端的可选项错误地限制住")
	}
	t.Logf("托管策略: managed=%v note=%q", req.Managed, req.Note)

	caps := p.Capabilities(".")
	kinds := map[string]int{}
	for _, c := range caps {
		kinds[c.Kind]++
	}
	t.Logf("能力: %d 项 %v", len(caps), kinds)
	// 每一项都必须有 kind 与 name,否则手机端渲染出来是一堆空行
	for i, c := range caps {
		if c.Kind == "" || c.Name == "" {
			t.Fatalf("第 %d 项能力缺少 kind/name: %+v", i, c)
		}
	}

	if servers, err := p.McpStatusDetailed(); err != nil {
		t.Logf("MCP 状态读不到(可接受): %v", err)
	} else {
		t.Logf("MCP: %d 个", len(servers))
		for _, s := range servers {
			// 这三项是"以前丢掉、现在必须带上"的:判断"某个 MCP 到底能不能用"
			// 全靠它们,缺了就等于没接。
			t.Logf("  %s tools=%d auth=%q status=%q title=%q ver=%q err=%q",
				s.Name, s.Tools, s.AuthStatus, s.Status, s.ServerTitle, s.ServerVersion, s.Error)
		}
	}
}

// ---- 分组:create → list → rename → move → delete ----

func TestLiveCodexSectionLifecycle(t *testing.T) {
	p := liveCodex(t)

	name := "pa-probe-" + time.Now().Format("150405")
	sec, err := p.CreateThreadSection(name)
	if err != nil {
		t.Fatalf("建分组失败: %v", err)
	}
	if sec.GetId() == "" || sec.GetName() != name {
		t.Fatalf("建分组返回不完整: %+v", sec)
	}
	defer func() { _ = p.DeleteThreadSection(sec.GetId()) }()

	sections, err := p.ThreadSections()
	if err != nil {
		t.Fatalf("列分组失败: %v", err)
	}
	found := false
	for _, s := range sections {
		if s.GetId() == sec.GetId() {
			found = true
		}
	}
	if !found {
		t.Errorf("刚建的分组没出现在列表里(共 %d 个)", len(sections))
	}

	renamed, err := p.RenameThreadSection(sec.GetId(), name+"-renamed")
	if err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	if renamed.GetName() != name+"-renamed" {
		t.Errorf("改名没生效: %q", renamed.GetName())
	}

	tid := newLiveThread(t, p)
	if err := p.MoveThreadToSection(tid, sec.GetId()); err != nil {
		t.Errorf("移入分组失败: %v", err)
	} else {
		// 移入后必须能在 thread/list 里读到 section.id —— 手机端的分组视图
		// 完全依赖这个字段;读不到就会出现"分组是空的,会话都在未分组"。
		entries, err := p.ListThreadEntries("", "", "", false, false, 0)
		if err != nil {
			t.Errorf("列会话失败: %v", err)
		}
		got, listed := "", false
		for _, e := range entries {
			if e.ID == tid {
				got, listed = e.SectionID, true
			}
		}
		if !listed {
			// 【已知的 codex 侧限制】没跑过任何回合的线程不会出现在 thread/list 里
			// (它连 rollout 文件都还没落,见下面 fork 的同类现象)。这时"分组读不回"
			// 不是我们的解析问题,是这条线程根本没被列举。如实说明而不是判失败。
			t.Logf("线程还没进 thread/list(尚无 rollout),分组读回不可验证")
		} else if got != sec.GetId() {
			t.Errorf("会话的分组没读回来: want=%s got=%q", sec.GetId(), got)
		}
	}
	// 移出:sectionId 必须显式 null(传空字符串会被当成"空名字的分组")
	if err := p.MoveThreadToSection(tid, ""); err != nil {
		t.Errorf("移出分组失败: %v", err)
	}
}

// ---- fork / rollback / 归档 / 改名 ----

func TestLiveCodexThreadOrganization(t *testing.T) {
	p := liveCodex(t)
	tid := newLiveThread(t, p)

	newID, err := p.ForkThread(tid, "")
	if err != nil {
		// 【已知的 codex 侧限制,不是我们的缺陷】thread/start 刚建的空线程
		// 还没落 rollout 文件,fork 会回 "no rollout found for thread id ..."。
		// 要让它能 fork 就得先跑一个回合 —— 那会真的消耗模型额度,不适合放在
		// 自动化实测里。这里如实记录并跳过后续断言,而不是把环境限制伪装成失败。
		if strings.Contains(err.Error(), "no rollout found") {
			t.Skipf("空线程还没有 rollout,fork 不可测(需要先跑一个回合): %v", err)
		}
		t.Fatalf("fork 失败: %v", err)
	}
	if newID == "" || newID == tid {
		t.Fatalf("fork 没给出新线程 id: %q", newID)
	}
	t.Logf("fork: %s → %s", tid, newID)

	// 刚 fork 出来的线程还没有回合,回滚必然失败 —— 这不是缺陷,是我们想确认的边界:
	// 错误要能被上层原样转成"退不回去",而不是静默成功。
	if err := p.RollbackThread(newID, 1); err != nil {
		t.Logf("回滚空线程按预期失败: %v", err)
	} else {
		t.Log("回滚空线程居然成功了(codex 允许,不是我们的问题)")
	}
	if err := p.RollbackThread(tid, 0); err == nil {
		t.Error("numTurns=0 应当在本地就被拒绝")
	}

	if err := p.SetCodexThreadName(tid, "pa-probe-name"); err != nil {
		t.Errorf("改名失败: %v", err)
	} else {
		entries, _ := p.ListThreadEntries("", "", "", false, false, 0)
		name := ""
		for _, e := range entries {
			if e.ID == tid {
				name = e.Title
			}
		}
		if name != "pa-probe-name" {
			t.Errorf("改名没写回 codex: %q", name)
		}
	}

	if err := p.SetCodexArchived(tid, true); err != nil {
		t.Errorf("归档失败: %v", err)
	} else {
		// 归档后必须只能在"含归档"查询里看到
		archived, err := p.ListThreadEntries("", "", "", false, true, 0)
		if err != nil {
			t.Errorf("含归档查询失败: %v", err)
		}
		seen, marked := false, false
		for _, e := range archived {
			if e.ID == tid {
				seen = true
				marked = e.Archived
			}
		}
		if !seen {
			t.Error("归档后的线程在含归档查询里也看不到")
		}
		if !marked {
			t.Error("归档标记没打上:syncThreadsFromCodex 会因此把归档的会话当活跃的")
		}
		if err := p.SetCodexArchived(tid, false); err != nil {
			t.Errorf("取消归档失败: %v", err)
		}
	}
}

// ---- 审查推翻:验证我们转写的 event 真的过得了 codex 的校验 ----

func TestLiveCodexGuardianApproveAccepted(t *testing.T) {
	p := liveCodex(t)
	tid := newLiveThread(t, p)

	// 造一条"被驳回"的记录(形状与真实通知完全一致:camelCase 的 status/action)
	p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
		"threadId":"`+tid+`","turnId":"probe-turn","reviewId":"probe-review-1",
		"startedAtMs":1000,"completedAtMs":1500,"targetItemId":"probe-item-1",
		"review":{"status":"denied","riskLevel":"high","rationale":"探针:验证载荷能被 codex 接受"},
		"action":{"type":"command","command":"echo probe","cwd":"C:\\Windows","source":"shell"}}`))
	if p.GuardianLatest(tid) == nil {
		t.Fatal("前置条件不成立:记录没进去")
	}

	// 真正下发一次。codex 接受 ⇒ 我们的转写(denied 保持不变、camelCase 迁移)是对的。
	// 这条线程本来就没有待处理的驳回,所以 codex 会宽容地返回成功 ——
	// 我们验证的正是"载荷过得了 serde 校验":写法错了会得到
	// "invalid Guardian denial event: ...",而不是成功。
	err := p.ApproveGuardianDenied(tid, "probe-review-1", "probe-item-1")
	if err != nil {
		// 载荷写法错(serde 拒绝)是最严重的情况:说明转写表错了。
		if strings.Contains(err.Error(), "invalid Guardian denial event") {
			t.Fatalf("codex 拒绝了我们的载荷,转写表有问题: %v", err)
		}
		t.Fatalf("下发失败: %v", err)
	}
	t.Log("codex 接受了我们转写的 GuardianAssessmentEvent")

	// 推过一次之后入口必须撤掉,否则用户会对着同一个结论反复点。
	if g := p.GuardianLatest(tid); g != nil {
		t.Error("推翻之后仍然提供推翻入口")
	}
}

// 每种动作类型都下发一次,确认转写在 codex 侧全部被接受。
//
// 这是本文件最有价值的一条:它把"我们猜的转写表"逐个拿真 codex 验了一遍。
func TestLiveCodexGuardianAllActionVariantsAccepted(t *testing.T) {
	p := liveCodex(t)
	tid := newLiveThread(t, p)

	actions := []string{
		`{"type":"command","command":"ls","cwd":"C:\\Windows","source":"unifiedExec"}`,
		`{"type":"execve","program":"node","argv":["node","-v"],"cwd":"C:\\Windows","source":"shell"}`,
		`{"type":"applyPatch","cwd":"C:\\Windows","files":["C:\\Windows\\a.txt"]}`,
		`{"type":"networkAccess","host":"example.com","port":443,"protocol":"socks5Tcp","target":"example.com"}`,
		`{"type":"mcpToolCall","server":"probe","toolName":"probe_tool"}`,
		`{"type":"requestPermissions","permissions":{},"reason":"探针"}`,
	}
	for i, act := range actions {
		reviewID := "probe-variant-" + itoa(i)
		p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
			"threadId":"`+tid+`","reviewId":"`+reviewID+`","targetItemId":"item-`+itoa(i)+`",
			"review":{"status":"timedOut"},
			"action":`+act+`}`))
		if err := p.ApproveGuardianDenied(tid, reviewID, ""); err != nil {
			t.Errorf("变体 %s 未被接受: %v", act, err)
		} else {
			t.Logf("变体通过: %.60s", act)
		}
	}
}

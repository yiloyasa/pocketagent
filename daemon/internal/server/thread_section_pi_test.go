package server

// thread_section_pi_test.go —— T-05 回归钉子:会话管理(分组/重命名)对
// pi/zcode 会话的覆盖。
//
// 【修复前的事实】
//  1. MoveThreadToSection 一刀切要求 codexProvider 在场且线程已落地:
//     pi/zcode 会话(永远没有 codex_thread_id)被硬拒,报的还是"先发一条消息"
//     这种对 pi 会话牛头不对马嘴的话 —— 与 ThreadsScreen 覆盖说明
//     (SECTION_COVERAGE_NOTE:"三家都可以移入")自相矛盾。
//  2. RenameThread 只落库;zcode 没有标题下发通道,下一轮 session/titleUpdated
//     自动命名经 TitleHandler 无条件落库,把用户刚改的名字顶回去
//     ("改名成功,聊了一句又变回去了")。
//
// 【修法】分组对非 codex 归属纯本地落库;重命名对没有下发能力(titlePusher)
// 的 agent 置 TitlePinned,applyZcodeTitle 跳过钉住的线程。

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// zcodeStub provider 接口的最小实现(与 PiProviderStub 同款,避免测试真拉子进程)。
type zcodeStub struct{}

func (zcodeStub) ID() string             { return "zcode" }
func (zcodeStub) DisplayName() string    { return "ZCode(测试)" }
func (zcodeStub) Models() []pb.ModelInfo { return nil }

func (zcodeStub) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

// piRenameStub 在最小桩上补 Rename(set_session_name 下发能力),用于验证
// "有下发通道的 agent 不钉 TitlePinned"。
type piRenameStub struct {
	PiProviderStub
}

func (piRenameStub) Rename(_ *store.Thread, _ string) error { return nil }

// TestMovePiThreadToSectionWithoutCodex pi 会话移动分组:不依赖 codex 在场,
// SectionID 落本地库;移出分组(空 section_id)同一条路径。
func TestMovePiThreadToSectionWithoutCodex(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"pi": &PiProviderStub{}, // 注意:注册表里没有 codex
	}})
	th := &store.Thread{ID: "th-pi-sec", Title: "pi 会话", AgentID: "pi", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.MoveThreadToSection(ctxBG(), &pb.MoveThreadToSectionRequest{ThreadId: th.ID, SectionId: "sec-1"}); err != nil {
		t.Fatalf("pi 会话移动分组不得要求 codex: %v", err)
	}
	got, ok := srv.cfg.Store.Get(th.ID)
	if !ok || got.SectionID != "sec-1" {
		t.Fatalf("分组应落本地库,得到 section=%q ok=%v", got.SectionID, ok)
	}
	// 移出分组:同一条本地路径,清空 SectionID。
	if _, err := srv.MoveThreadToSection(ctxBG(), &pb.MoveThreadToSectionRequest{ThreadId: th.ID}); err != nil {
		t.Fatalf("pi 会话移出分组失败: %v", err)
	}
	if got, _ = srv.cfg.Store.Get(th.ID); got.SectionID != "" {
		t.Fatalf("移出分组后 SectionID 应为空,得到 %q", got.SectionID)
	}
}

// TestMoveZcodeThreadToSectionWithoutCodex zcode 会话同款:纯本地分组。
func TestMoveZcodeThreadToSectionWithoutCodex(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"zcode": zcodeStub{},
	}})
	th := &store.Thread{ID: "th-zc-sec", Title: "zcode 会话", AgentID: "zcode", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.MoveThreadToSection(ctxBG(), &pb.MoveThreadToSectionRequest{ThreadId: th.ID, SectionId: "sec-2"}); err != nil {
		t.Fatalf("zcode 会话移动分组不得要求 codex: %v", err)
	}
	if got, _ := srv.cfg.Store.Get(th.ID); got.SectionID != "sec-2" {
		t.Fatalf("分组应落本地库,得到 %q", got.SectionID)
	}
}

// TestMoveUnlandedCodexThreadStillRejected codex 会话保持原语义:未落地/无
// codex 时显式报错,不得静默改成"只动本地"(codex 桌面端也有分组,只动本地
// 会造成两端两套事实)。
func TestMoveUnlandedCodexThreadStillRejected(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"codex": &seqProvider{id: "codex"}, // 测试桩:codexProvider() 断言不到,等价"codex 不可用"
	}})
	th := &store.Thread{ID: "th-cx-sec", Title: "codex 会话", AgentID: "codex", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.MoveThreadToSection(ctxBG(), &pb.MoveThreadToSectionRequest{ThreadId: th.ID, SectionId: "sec-3"}); err == nil {
		t.Fatal("codex 会话分组不可用/未落地时必须显式报错,不得静默只动本地")
	}
}

// TestRenameZcodeThreadPinsAgainstAutoTitle zcode 会话手动改名后:
// (a) TitlePinned 落库;(b) 之后的自动命名回调(applyZcodeTitle)不再覆盖;
// (c) 没被手动改过的线程照常被自动命名同步(守卫不能误伤同步功能)。
func TestRenameZcodeThreadPinsAgainstAutoTitle(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"zcode": zcodeStub{},
	}})
	pinned := &store.Thread{ID: "th-zc-pin", Title: "自动命名A", AgentID: "zcode", Workspace: "C:\\ws"}
	auto := &store.Thread{ID: "th-zc-auto", Title: "自动命名A", AgentID: "zcode", Workspace: "C:\\ws"}
	for _, th := range []*store.Thread{pinned, auto} {
		if err := srv.cfg.Store.Create(th); err != nil {
			t.Fatalf("建测试线程: %v", err)
		}
	}
	if _, err := srv.RenameThread(ctxBG(), &pb.RenameThreadRequest{ThreadId: pinned.ID, Title: "我的名字"}); err != nil {
		t.Fatalf("RenameThread: %v", err)
	}
	got, _ := srv.cfg.Store.Get(pinned.ID)
	if got.Title != "我的名字" {
		t.Fatalf("手动改名应落库,得到 %q", got.Title)
	}
	if !got.TitlePinned {
		t.Fatal("zcode(无标题下发通道)手动改名后必须置 TitlePinned")
	}
	// 自动命名来了:钉住的跳过,没钉的照常同步。
	applyZcodeTitle(srv.cfg.Store, pinned.ID, "自动命名B")
	applyZcodeTitle(srv.cfg.Store, auto.ID, "自动命名B")
	if got, _ = srv.cfg.Store.Get(pinned.ID); got.Title != "我的名字" {
		t.Fatalf("钉住的线程不得被自动命名顶掉,得到 %q", got.Title)
	}
	if got2, _ := srv.cfg.Store.Get(auto.ID); got2.Title != "自动命名B" {
		t.Fatalf("未改名线程的自动命名同步不得受影响,得到 %q", got2.Title)
	}
}

// TestRenameWithPushChannelNotPinned 有标题下发能力的 agent(pi,Rename 探测
// 命中)手动改名不置 TitlePinned —— 双端同源靠下发保证,电脑端发起的改名要能
// 照常同步回手机,不能被钉子挡住。
func TestRenameWithPushChannelNotPinned(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{
		"pi": piRenameStub{},
	}})
	th := &store.Thread{ID: "th-pi-pin", Title: "旧名", AgentID: "pi", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.RenameThread(ctxBG(), &pb.RenameThreadRequest{ThreadId: th.ID, Title: "新名"}); err != nil {
		t.Fatalf("RenameThread: %v", err)
	}
	got, _ := srv.cfg.Store.Get(th.ID)
	if got.TitlePinned {
		t.Fatal("有下发通道的 agent(pi)不得置 TitlePinned")
	}
}

// TestApplyZcodeTitleIgnoresOtherThreads applyZcodeTitle 只动自己名下的线程,
// 不碰别的线程(回调带的是我们自己的线程 ID,这里钉死不串号)。
func TestApplyZcodeTitleIgnoresOtherThreads(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{}})
	th := &store.Thread{ID: "th-a", Title: "旧名", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	applyZcodeTitle(srv.cfg.Store, "th-不存在", "新名")
	if got, _ := srv.cfg.Store.Get(th.ID); got.Title != "旧名" {
		t.Fatalf("别的线程不得被误改,得到 %q", got.Title)
	}
}

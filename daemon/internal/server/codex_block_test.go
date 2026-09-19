package server

// codex_block_test.go —— "daemon 无界阻塞"的回归测试。
//
// 【要钉死的两件事】
//  1. 探测错误的**分级映射**:超时/正忙 → unreachable(可重试),真失败 → failed,
//     不支持项永远 unsupported。这是手机端文案措辞的依据,不能靠肉眼。
//  2. **保存的语义底线**:探测不可用时,SetCodexSettings 仍必须返回成功,
//     且返回值等于**落盘值** —— 绝不能把"回读失败"变成"保存失败"。
//     这正是用户抱怨"点了保存永远转圈 / 几分钟后自己回滚"的根治点。

import (
	"context"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 假探测:覆盖"永远阻塞"与"立即报错"两种行为 ----

// blockingProbe 永远不主动返回,直到 ctx 到期(模拟 codex 起来了但握手不完成)。
type blockingProbe struct{}

func (blockingProbe) ReadyCtx(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
func (blockingProbe) AccountCtx(context.Context) string  { return "" }
func (blockingProbe) RequirementsCtx(context.Context) (agent.CodexRequirements, error) {
	return agent.CodexRequirements{}, nil
}

// errorProbe 立即返回给定错误(模拟"锁被占"或"codex 挂了")。
type errorProbe struct{ err error }

func (e errorProbe) ReadyCtx(context.Context) error { return e.err }
func (e errorProbe) AccountCtx(context.Context) string {
	return ""
}
func (e errorProbe) RequirementsCtx(context.Context) (agent.CodexRequirements, error) {
	return agent.CodexRequirements{}, nil
}

// newProbeTestServer 造一个带真实(临时)FileStore 与真实 provider 的 Server,
// 只把"探测"替换成注入的假实现。
// (注意:不要叫 newTestServer —— 该名字已被 thread_ext_test.go 占用。)
func newProbeTestServer(t *testing.T, probe codexProbe) (*Server, *agent.CodexProvider) {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	p := &agent.CodexProvider{}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{"codex": p}})
	srv.probeOverride = probe
	return srv, p
}

// TestCodexSaveReturnsStoreValueWhenProbeUnavailable 是本轮验收点:
// 探测不可用(超时 / 正忙)时,保存仍然成功,返回值等于落盘值,状态降级为 unreachable。
func TestCodexSaveReturnsStoreValueWhenProbeUnavailable(t *testing.T) {
	cases := []struct {
		name  string
		probe codexProbe
	}{
		{"探测永远阻塞(超时)", blockingProbe{}},
		{"探测立即报忙(ErrBusy)", errorProbe{agent.ErrBusy}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newProbeTestServer(t, tc.probe)

			// 给一个很短的父 ctx:snapshot 会取其与自身上界的更早者,
			// 于是 blockingProbe 在 ~200ms 内到期,测试不会真的等 8~10s。
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			ap := "on-request"
			md := "codex/gpt-5"
			pbOn := true
			start := time.Now()
			got, err := srv.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
				ApprovalPolicy:    &ap,
				Model:             &md,
				DisablePasteBurst: &pbOn,
			})
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("探测不可用时保存不应失败(落盘已成功): %v", err)
			}
			if got == nil {
				t.Fatalf("返回值为 nil")
			}
			// 铁律:返回值必须等于落盘值。
			if got.ApprovalPolicy != "on-request" {
				t.Fatalf("返回值 approval 应等于落盘值 on-request, got=%q", got.ApprovalPolicy)
			}
			if got.Model != "codex/gpt-5" {
				t.Fatalf("返回值 model 应等于落盘值 codex/gpt-5, got=%q", got.Model)
			}
			if !got.DisablePasteBurst {
				t.Fatalf("disable_paste_burst 应照常落盘并回显")
			}
			// store 真的写进去了。
			if srv.cfg.Store.CodexPrefs().ApprovalPolicy != "on-request" {
				t.Fatalf("store 未落盘 approval")
			}
			if srv.cfg.Store.DefaultModelID() != "codex/gpt-5" {
				t.Fatalf("store 未落盘 model")
			}
			if !srv.cfg.Store.CodexPrefs().DisablePasteBurst {
				t.Fatalf("store 未落盘 disable_paste_burst")
			}
			// 逐项状态:可达性降级为 unreachable(而不是 failed)。
			states := map[string]string{}
			for _, s := range got.SettingStatus {
				states[s.Key] = s.State
			}
			if states["approval_policy"] != "unreachable" {
				t.Fatalf("探测不可用时 approval_policy 应为 unreachable, got=%q", states["approval_policy"])
			}
			if states["disable_paste_burst"] != "unsupported" {
				t.Fatalf("unsupported 项必须保持 unsupported, got=%q", states["disable_paste_burst"])
			}
			// 上界要有界:不能出现"永远不返回"。
			if elapsed > 5*time.Second {
				t.Fatalf("保存耗时 %v 过长(上界失效)", elapsed)
			}
		})
	}
}

// TestClassifyReach 钉住"正忙/超时 → unreachable,真失败 → failed,nil → ok"。
func TestClassifyReach(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codexReach
	}{
		{"nil → ok", nil, reachOK},
		{"ErrBusy → unreachable", agent.ErrBusy, reachUnreachable},
		{"DeadlineExceeded → unreachable", context.DeadlineExceeded, reachUnreachable},
		{"Canceled → unreachable", context.Canceled, reachUnreachable},
		{"普通错误 → failed", errPlain{}, reachFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyReach(tc.err); got != tc.want {
				t.Fatalf("classifyReach(%v)=%d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

type errPlain struct{}

func (errPlain) Error() string { return "boom" }

// TestBuildSettingStatusWithReach 钉住三态翻译,以及"不支持项优先于可达性"。
func TestBuildSettingStatusWithReach(t *testing.T) {
	c := &pb.CodexSettings{ApprovalPolicy: "on-request", Model: "codex/gpt-5"}

	t.Run("unreachable:支持项降级为 unreachable,不支持项仍 unsupported", func(t *testing.T) {
		got := buildSettingStatusWithReach(c, reachUnreachable, "codex 未响应(超过 8s),无法确认是否生效,稍后可重试")
		for _, s := range got {
			if s.Key == "disable_paste_burst" {
				if s.State != "unsupported" {
					t.Fatalf("不支持项应为 unsupported, got=%q", s.State)
				}
				continue
			}
			if s.State != "unreachable" {
				t.Fatalf("%s 应为 unreachable, got=%q", s.Key, s.State)
			}
			if s.Detail == "" {
				t.Fatalf("%s unreachable 但没给 detail", s.Key)
			}
		}
	})

	t.Run("failed:支持项为 failed 且 detail 带真实原因", func(t *testing.T) {
		got := buildSettingStatusWithReach(c, reachFailed, "无法连接 codex:没有安装")
		for _, s := range got {
			if s.Key == "disable_paste_burst" {
				continue
			}
			if s.State != "failed" {
				t.Fatalf("%s 应为 failed, got=%q", s.Key, s.State)
			}
			if s.Detail == "" {
				t.Fatalf("%s failed 但没给 detail", s.Key)
			}
		}
	})

	t.Run("ok:有值=applied、空值=follow_desktop", func(t *testing.T) {
		got := buildSettingStatusWithReach(&pb.CodexSettings{ApprovalPolicy: "never"}, reachOK, "")
		byKey := map[string]*pb.CodexSettingStatus{}
		for _, s := range got {
			byKey[s.Key] = s
		}
		if byKey["approval_policy"].State != "applied" {
			t.Fatalf("approval_policy 应为 applied, got=%q", byKey["approval_policy"].State)
		}
		if byKey["model"].State != "follow_desktop" {
			t.Fatalf("model 空值应为 follow_desktop, got=%q", byKey["model"].State)
		}
	})
}

// TestReachDetailOfMentionsRetryOrReason 钉住 unreachable 的措辞是可重试的(不是"坏了")。
func TestReachDetailOfMentionsRetryOrReason(t *testing.T) {
	busy := reachDetailOf(reachUnreachable, agent.ErrBusy, 8*time.Second)
	if busy == "" {
		t.Fatalf("busy detail 为空")
	}
	timeout := reachDetailOf(reachUnreachable, context.DeadlineExceeded, 8*time.Second)
	if timeout == "" {
		t.Fatalf("timeout detail 为空")
	}
	failed := reachDetailOf(reachFailed, errPlain{}, 8*time.Second)
	if failed == "" {
		t.Fatalf("failed detail 为空")
	}
}

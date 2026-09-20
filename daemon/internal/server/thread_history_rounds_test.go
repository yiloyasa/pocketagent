// thread_history_rounds_test.go —— GetThreadHistoryRequest.rounds 按轮尾部截断
// 的回归:正常 N 轮切分 / 失败轮只有 user / 导入种子首条非 user / rounds 超总
// 轮数=全量 / rounds=0=全量(兼容铁律:存量 APK 不传 rounds 必须仍全量)/
// 与 limit 的并存叠加语义。另含 tailByRounds 纯函数的边界表测。
package server

import (
	"context"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// newRoundsThread 建一条指定 role 序列的历史线程(u=用户,a=助手),CreatedAt 递增。
func newRoundsThread(t *testing.T, st *store.FileStore, id, roles string) *store.Thread {
	t.Helper()
	th := &store.Thread{ID: id, Title: id, Workspace: "w", ModelID: "codex/gpt"}
	at := int64(1000)
	for _, r := range roles {
		role := "user"
		if r == 'a' {
			role = "assistant"
		}
		th.Messages = append(th.Messages, store.StoredMessage{Role: role, Blocks: textBlocks(role), CreatedAt: at})
		at += 1000
	}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return th
}

// rolesOf 取回包里的 role 序列,便于逐条断言。
func rolesOf(out *pb.GetThreadHistoryResponse) []string {
	rs := make([]string, 0, len(out.Messages))
	for _, m := range out.Messages {
		rs = append(rs, m.Role)
	}
	return rs
}

func expectRoles(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("条数不符: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 条 role 不符: got=%v want=%v", i, got, want)
		}
	}
}

// ① 正常 N 轮切分:user/assistant 严格交替,rounds=N 取最近 N 轮。
func TestGetThreadHistoryRoundsNormalCut(t *testing.T) {
	st := mustStore(t)
	th := newRoundsThread(t, st, "th-r-normal", "uauauaua") // 4 轮
	srv := New(Config{Store: st})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 2})
	if err != nil {
		t.Fatalf("rounds=2 不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out), "user", "assistant", "user", "assistant")

	out1, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 1})
	if err != nil {
		t.Fatalf("rounds=1 不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out1), "user", "assistant")

	// 恰好等于总轮数:等于全量(顺序原样,不重排)。
	out4, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 4})
	if err != nil {
		t.Fatalf("rounds=4 不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out4), "user", "assistant", "user", "assistant", "user", "assistant", "user", "assistant")
}

// ② 失败轮只有 user 无 assistant:该 user 自己算一轮,必须随 rounds 返回。
func TestGetThreadHistoryRoundsFailedTurnUserOnly(t *testing.T) {
	st := mustStore(t)
	th := newRoundsThread(t, st, "th-r-fail", "uauau") // 第 3 轮失败:只有 user
	srv := New(Config{Store: st})

	// rounds=2:最后一轮(失败的 user)+ 前一个完整轮。
	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 2})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out), "user", "assistant", "user")

	// rounds=1:只取失败的 user(尾部 assistant 悬挂容忍:本来就没有)。
	out1, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 1})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out1), "user")
}

// ③ 导入/fork 种子首条非 user:种子不属于任何轮,不参与轮次计数;
// rounds 数满时从最早的 user 起切(种子被裁),rounds 超总轮数才带出种子。
func TestGetThreadHistoryRoundsSeedFirstNotUser(t *testing.T) {
	st := mustStore(t)
	th := newRoundsThread(t, st, "th-r-seed", "auaua") // 首条为 assistant 种子 + 2 轮
	srv := New(Config{Store: st})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 1})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out), "user", "assistant")

	out2, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 2})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out2), "user", "assistant", "user", "assistant")

	// rounds=3 超过可数轮数(2)→ 全量,种子原样带回。
	out3, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: 3})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out3), "assistant", "user", "assistant", "user", "assistant")
}

// ④⑤ rounds 超总轮数 = 全量;rounds=0 / 负数 = 全量(兼容铁律:存量 APK
// 不传 rounds,导出全量与断流恢复回填都依赖这个语义)。
func TestGetThreadHistoryRoundsZeroAndOverflowFullReturn(t *testing.T) {
	st := mustStore(t)
	th := newRoundsThread(t, st, "th-r-full", "uaua") // 2 轮
	srv := New(Config{Store: st})
	full := []string{"user", "assistant", "user", "assistant"}

	for _, tc := range []struct {
		name   string
		rounds int32
	}{
		{"rounds=0(旧客户端缺省)", 0},
		{"rounds=-5(负数防御)", -5},
		{"rounds=99(超总轮数)", 99},
	} {
		out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Rounds: tc.rounds})
		if err != nil {
			t.Fatalf("%s 不应报错: %v", tc.name, err)
		}
		expectRoles(t, rolesOf(out), full...)
	}
}

// ⑥ 与 limit 并存:limit 先按条数尾截(既有语义、优先级不变),rounds 在
// limit 的结果上叠加收窄;limit 切进轮中间导致轮起点不足时,rounds 不放大。
func TestGetThreadHistoryRoundsCoexistsWithLimit(t *testing.T) {
	st := mustStore(t)
	th := newRoundsThread(t, st, "th-r-limit", "uauaua") // 3 轮 6 条
	srv := New(Config{Store: st})

	// limit=4 尾截到 [u a u a],rounds=1 再收窄到最后一轮。
	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Limit: 4, Rounds: 1})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out), "user", "assistant")

	// limit 切进轮中间:[a u a](只剩 1 个 user 起点),rounds=2 轮数不足
	// → 不放大,原样返回 limit 后的切片。
	out2, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Limit: 3, Rounds: 2})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(out2), "assistant", "user", "assistant")

	// 只有 limit(不传 rounds):既有行为零变化。
	outL, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Limit: 2})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	expectRoles(t, rolesOf(outL), "user", "assistant")
}

// tailByRounds 纯函数边界表测(空表 / 非法 rounds / 连续 user / 悬挂 assistant)。
func TestTailByRoundsEdgeCases(t *testing.T) {
	m := func(roles ...string) []store.StoredMessage {
		out := make([]store.StoredMessage, len(roles))
		for i, r := range roles {
			out[i] = store.StoredMessage{Role: r}
		}
		return out
	}
	roles := func(msgs []store.StoredMessage) []string {
		out := make([]string, len(msgs))
		for i, mm := range msgs {
			out[i] = mm.Role
		}
		return out
	}

	t.Run("nil 与空表全量返回", func(t *testing.T) {
		if got := tailByRounds(nil, 3); got != nil {
			t.Fatalf("nil 应原样返回, got=%v", got)
		}
		if got := tailByRounds(m(), 3); len(got) != 0 {
			t.Fatalf("空表应原样返回, got=%v", got)
		}
	})
	t.Run("rounds<=0 原样返回(兼容铁律)", func(t *testing.T) {
		msgs := m("user", "assistant")
		if got := tailByRounds(msgs, 0); len(got) != 2 {
			t.Fatalf("rounds=0 应全量, got=%v", roles(got))
		}
		if got := tailByRounds(msgs, -1); len(got) != 2 {
			t.Fatalf("rounds<0 应全量, got=%v", roles(got))
		}
	})
	t.Run("连续 user 各算一轮", func(t *testing.T) {
		// 两个连续 user 是两个轮起点;rounds=2 切到第 1 个 user,其后全归第 2 轮。
		got := tailByRounds(m("user", "user", "assistant"), 2)
		expectRoles(t, roles(got), "user", "user", "assistant")
	})
	t.Run("尾部 assistant 悬挂全归最后一轮", func(t *testing.T) {
		got := tailByRounds(m("user", "assistant", "user", "assistant", "assistant"), 1)
		expectRoles(t, roles(got), "user", "assistant", "assistant")
	})
}

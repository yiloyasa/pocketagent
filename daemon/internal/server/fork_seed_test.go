package server

// 分叉时"带哪些历史"的规则。
//
// 【为什么值得单测】这条规则错了的表现是"分叉出来是一条空白会话" —— 真机验过,
// 用户看到的就是"分叉坏了"。而它真正的分支很多(读到了 / 读不到 / 读到空 /
// 源会话本身没消息),每种都要有确定行为,不能靠"大概会走到主路径"。

import (
	"errors"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// msg 造一条纯文本消息(StoredMessage 没有 Text 字段,文本在 Blocks 里)。
func msg(role, text string) store.StoredMessage {
	return store.StoredMessage{
		Role: role,
		Blocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}},
		},
	}
}

// textOf 取回消息文本,便于断言"带的是哪一份历史"。
func textOf(m store.StoredMessage) string {
	// 按下标取址遍历,避免 range 复制 protobuf 消息(内嵌 MessageState 含锁)。
	for i := range m.Blocks {
		if tb, ok := m.Blocks[i].Block.(*pb.ContentBlock_Text); ok {
			return tb.Text.Text
		}
	}
	return ""
}

func TestForkSeedPrefersTheNewThreadsRollout(t *testing.T) {
	src := &store.Thread{Messages: []store.StoredMessage{msg("user", "源会话的消息")}}
	rollout := []store.StoredMessage{msg("user", "被复制过来的历史"), msg("assistant", "好的")}

	got := forkSeedMessages("new-codex-id", src, func(id string) ([]store.StoredMessage, error) {
		if id != "new-codex-id" {
			t.Fatalf("应当用**新**线程 id 去读 rollout,实际传了 %q", id)
		}
		return rollout, nil
	})

	if len(got) != 2 || textOf(got[0]) != "被复制过来的历史" {
		t.Fatalf("应当采用 rollout 里的历史,实际 %d 条", len(got))
	}
}

func TestForkSeedFallsBackToSourceWhenRolloutIsUnavailable(t *testing.T) {
	src := &store.Thread{Messages: []store.StoredMessage{msg("user", "hi"), msg("assistant", "hello")}}

	got := forkSeedMessages("new-codex-id", src, func(string) ([]store.StoredMessage, error) {
		return nil, errors.New("找不到 rollout")
	})

	if len(got) != 2 || textOf(got[0]) != "hi" {
		t.Fatalf("读不到 rollout 时应退回源会话消息,实际 %d 条", len(got))
	}
}

func TestForkSeedFallsBackWhenRolloutIsEmpty(t *testing.T) {
	src := &store.Thread{Messages: []store.StoredMessage{msg("user", "hi")}}

	// "读到了但一条都没有"和"读不到"要一样处理:空历史等于空白会话,
	// 退回源消息至少能让用户看见自己知道的那部分。
	got := forkSeedMessages("new-codex-id", src, func(string) ([]store.StoredMessage, error) {
		return nil, nil
	})

	if len(got) != 1 || textOf(got[0]) != "hi" {
		t.Fatalf("rollout 为空时应退回源会话消息,实际 %d 条", len(got))
	}
}

func TestForkSeedDoesNotShareTheSourceSlice(t *testing.T) {
	src := &store.Thread{Messages: []store.StoredMessage{msg("user", "hi"), msg("assistant", "hello")}}

	got := forkSeedMessages("", src, nil)
	if len(got) != 2 {
		t.Fatalf("应当复制源会话的两条消息,实际 %d 条", len(got))
	}
	// 之后回滚会裁剪 Messages,共用底层数组会让源会话跟着被裁 —— 必须是真的复制。
	got[0] = msg("user", "改过的")
	if textOf(src.Messages[0]) != "hi" {
		t.Fatalf("分叉出来的消息与源会话共用底层数组,改一个动了另一个")
	}
}

func TestForkSeedWithoutRolloutAndWithoutSourceMessages(t *testing.T) {
	got := forkSeedMessages("new-codex-id", &store.Thread{}, func(string) ([]store.StoredMessage, error) {
		return nil, errors.New("nope")
	})
	if len(got) != 0 {
		t.Fatalf("两边都没有消息时应当返回空,实际 %d 条", len(got))
	}
}

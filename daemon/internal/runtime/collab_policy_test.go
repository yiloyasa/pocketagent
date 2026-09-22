// collab_policy_test.go —— 协同策略的默认值语义与落盘(规格 §11)。
package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollabPolicyDefaultsAreAllOn(t *testing.T) {
	// 【为什么默认全开且必须钉住】这些开关是用来收窄能力的;默认值与既有行为不一致,
	// 就意味着升级一次就悄悄少了一半功能,而用户无从知道是自己哪次操作导致的。
	p := CollabPolicy{}
	if !p.CollabEnabled() || !p.DelegateEnabled() || !p.Fanout() {
		t.Fatalf("零值策略应等同内置默认(全开): %+v", p)
	}
	if p.MaxMembers() != 0 {
		t.Fatalf("默认扇出成员数应为不限(0),得到 %d", p.MaxMembers())
	}
}

func TestCollabPolicyExplicitFalseWins(t *testing.T) {
	off := false
	p := CollabPolicy{FanoutEnabled: &off}
	if p.Fanout() {
		t.Fatal("显式 false 必须生效 —— 用零值当哨兵会让'用户明确关掉'被读成'没设置'")
	}
	if !p.CollabEnabled() {
		t.Fatal("未设置的项应沿用默认(开)")
	}
}

func TestCollabPolicyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	off := false
	two := 2
	p := CollabPolicy{Revision: 4, FanoutEnabled: &off, MaxParallelMembers: &two}
	if err := SaveCollabPolicy(dir, p); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	got, err := LoadCollabPolicy(dir)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if got.Revision != 4 || got.Fanout() || got.MaxMembers() != 2 {
		t.Fatalf("读回不一致: %+v", got)
	}
	// 原子写:不该残留临时文件。
	if _, err := os.Stat(filepath.Join(dir, collabPolicyFileName+".tmp")); err == nil {
		t.Fatal("落盘后不该残留 .tmp")
	}
}

func TestCollabPolicyValidateRejectsNegativeMembers(t *testing.T) {
	neg := -1
	if err := (CollabPolicy{MaxParallelMembers: &neg}).Validate(); err == nil {
		t.Fatal("负数上限应被拒(不限请显式填 0)")
	}
	zero := 0
	if err := (CollabPolicy{MaxParallelMembers: &zero}).Validate(); err != nil {
		t.Fatalf("0 是合法取值(不限),不该被拒: %v", err)
	}
}

func TestCollabPolicyMissingFileUsesDefault(t *testing.T) {
	p, err := LoadCollabPolicy(t.TempDir())
	if err != nil {
		t.Fatalf("缺文件不该报错: %v", err)
	}
	if !p.Fanout() {
		t.Fatal("缺文件应回内置默认")
	}
}

func TestCollabPolicyCorruptIsFailSoftButLoud(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, collabPolicyFileName)
	if err := os.WriteFile(path, []byte("{ broken"), 0o600); err != nil {
		t.Fatalf("造文件失败: %v", err)
	}
	p, err := LoadCollabPolicy(dir)
	if err == nil {
		t.Fatal("损坏应回报错误(供调用方出声)")
	}
	if !p.Fanout() {
		t.Fatal("损坏时应回默认(全开)—— 静默全关等于悄悄砍掉一半功能")
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Fatalf("损坏文件应保留: %v", serr)
	}
}

func TestCollabPolicyEffectiveFillsAllFields(t *testing.T) {
	off := false
	eff := (CollabPolicy{FanoutEnabled: &off}).Effective()
	if eff.FanoutEnabled == nil || eff.CollaborationEnabled == nil ||
		eff.NativeDelegateEnabled == nil || eff.MaxParallelMembers == nil {
		t.Fatalf("生效值应全字段齐备(客户端据此显示,不该靠缺席推断): %+v", eff)
	}
	if *eff.FanoutEnabled {
		t.Fatal("生效值应保留用户的显式关闭")
	}
	if !*eff.CollaborationEnabled {
		t.Fatal("生效值应把未设置项填成默认")
	}
}

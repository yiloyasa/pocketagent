// zcode_noticekind_preadopt_20260930_test.go —— 2026-09-30 zcode 预收编批的
// 分类钉子。
//
// 【钉什么】process/toolExecResource、plugins/operationProgress、
// interaction/providerRuntimeHeadersCancelled 三名在 bundle 顶层通知注册表静态
// 在案但无实测帧,主模型裁决按 process/* 遥测家族口径**预收编** PASSTHROUGH
// (未来出现即已知,不灌 UNKNOWN)。本文件钉:①三名 = PASSTHROUGH(只留痕不
// 投影);②全局钩子接线(zcodeLedgerMethod)以**平键**登记三名(它们是顶层
// 通知,不走 session/event 拆壳),分类随行;③台账落库 kind 与分类一致;
// ④红线反向钉:反向请求 interaction/requestOfficialMcpAuthHeaders 仍 UNKNOWN
// (本表只管通知 —— 与预收编的 interaction/* 通知不是一路)。
//
// 【证据口径】bundle 顶层通知注册表静态在案,无实测帧(证据分级:静态)。
package agent

import (
	"encoding/json"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

// preAdopt20260930Names 是本批预收编的三名(顶层通知注册表静态在案)。
var preAdopt20260930Names = []string{
	"process/toolExecResource",
	"plugins/operationProgress",
	"interaction/providerRuntimeHeadersCancelled",
}

func TestZcodeNoticeKindPreAdopt20260930Batch(t *testing.T) {
	for _, m := range preAdopt20260930Names {
		if got := zcodeNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("zcodeNoticeKind(%q) = %q, want PASSTHROUGH(预收编:注册表在案无实测帧,出现即已知不灌 UNKNOWN)", m, got)
		}
	}
}

// 全局钩子接线钉:三名是顶层通知,zcodeLedgerMethod 必须原样平键登记、
// 分类随行 —— 不许被误当 session/event 拆壳成复合键。
func TestZcodeLedgerMethodPreAdopt20260930PlainKeys(t *testing.T) {
	for _, m := range preAdopt20260930Names {
		gotM, gotK := zcodeLedgerMethod(m, json.RawMessage(`{}`))
		if gotM != m || gotK != runtime.KindKnownPassthrough {
			t.Errorf("zcodeLedgerMethod(%q) = (%q,%q), want 平键 + PASSTHROUGH", m, gotM, gotK)
		}
	}
}

// 台账落库钉:走与生产全局钩子同一对(zcodeLedgerMethod + recordProviderEvent),
// 断言三名在台账里的 normalized_kind 确为 PASSTHROUGH。
func TestZcodePreAdopt20260930LandsInLedger(t *testing.T) {
	const th = "th-zcode-preadopt-20260930"
	for _, m := range preAdopt20260930Names {
		ledgerM, kind := zcodeLedgerMethod(m, json.RawMessage(`{}`))
		recordProviderEvent("zcode", ledgerM, th, kind, json.RawMessage(`{}`))
	}
	seen := 0
	for _, e := range sharedRuntimeBackend().UnknownEvents().List(runtime.ListFilter{ThreadID: th}, 0) {
		isPreAdopt := false
		for _, m := range preAdopt20260930Names {
			if e.SourceMethod == m {
				isPreAdopt = true
				break
			}
		}
		if !isPreAdopt {
			continue
		}
		seen++
		if e.NormalizedKind != runtime.KindKnownPassthrough {
			t.Errorf("%s 台账 normalized_kind = %q, want PASSTHROUGH", e.SourceMethod, e.NormalizedKind)
		}
	}
	if seen != len(preAdopt20260930Names) {
		t.Fatalf("有预收编名没落台账: seen=%d want=%d", seen, len(preAdopt20260930Names))
	}
}

// 红线反向钉:预收编不改变既有边界 —— 反向请求
// interaction/requestOfficialMcpAuthHeaders 仍 UNKNOWN(本表只管通知);未知名
// 兜底仍 UNKNOWN(预收编不等于"凡是新名字都算已知")。
func TestZcodeNoticeKindPreAdoptRedLinesHold(t *testing.T) {
	for _, m := range []string{
		"interaction/requestOfficialMcpAuthHeaders",
		"process/brandNewSample", "plugins/brandNewProgress", "",
	} {
		if got := zcodeNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("zcodeNoticeKind(%q) = %q, want UNKNOWN(预收编批不改既有红线/兜底)", m, got)
		}
	}
}

// prefs_access.go —— 桌面偏好(DesktopPrefs)的共享读写 + 由它驱动的思考流过滤出口。
//
// 【它解决什么】三段逐字重复体:
//  1. ApplyPrefs 整快照覆盖(dsh/hermes/pi/zcode 四份 —— codex 不同,见下);
//  2. hideReasoning 读"隐藏推理"开关(同样四份);
//  3. emitThinking/emitThinkingFor 的 ThinkingDelta 过滤与帧形状(pi/zcode 四个方法)。
//
// 收敛后语义零变,但"加锁→读/写快照"与"推一帧"各自只有一处实现:
//   - 锁语义一字不改 —— 各家仍锁各家的锁(hermes 的 p.mu 按其结构注释就是
//     "prefs 锁",dsh/pi/zcode 用 prefsMu),锁的选型归各家,本文件不代选;
//   - 方法本身全部保留(单测直接调 p.ApplyPrefs / p.hideReasoning /
//     p.emitThinking / p.hideReasoningFor),签名与调用点不动。
//
// codex 不并入:它的 ApplyPrefs 写的是 setMu 保护的**拆散字段**
// (hideAgentReasoning 等),不是整份 DesktopPrefs 快照,结构本就不同。
package agent

import (
	"sync"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// applyPrefsSnapshot 在调用方给定的锁内做整快照覆盖(照单全收,不逐字段合并)。
// 这是 PrefsAware 的既定纪律:设置页给的是七项完整快照,旧值必须被覆盖掉,
// 否则"关掉"会读回上一次的"开"。
func applyPrefsSnapshot(mu sync.Locker, dst *DesktopPrefs, src DesktopPrefs) {
	mu.Lock()
	defer mu.Unlock()
	*dst = src
}

// hideReasoningFrom 在调用方给定的锁内读 DesktopPrefs.HideReasoning。
// 传锁进来只是为了不复制那四行样板 —— 读的是同一字段、走同一把锁,与各家
// 原实现等价。
func hideReasoningFrom(mu sync.Locker, prefs *DesktopPrefs) bool {
	mu.Lock()
	defer mu.Unlock()
	return prefs.HideReasoning
}

// emitThinkingDelta 在"隐藏推理"关闭时把思考增量推给 sink(daemon 侧强制过滤,
// 与 codex 的 HideReasoning 分支同款;pi/zcode 协议都没有原生开关)。
// hide=true 时一帧不发 —— 单测钉死"HideReasoning=true 时 ThinkingDelta 一帧不出"
// (pi_prefs_effect_test / zcode_prefs_effect_test)。
func emitThinkingDelta(hide bool, delta string, sink Sink) {
	if hide {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: delta}}})
}

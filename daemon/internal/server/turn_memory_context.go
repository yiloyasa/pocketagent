package server

import (
	"fmt"
	"strings"
)

// turn_memory_context.go —— P2「四层上下文组装」：ReadWindow/记忆槽的第一个真实读者
// （memory 分层设计稿 §4 P2/D4、spec §7:598-610「daemon 必须在每次请求前按预算生成上下文」）。
//
// 【四层是什么、从哪来】对照 ChatGPT 逆向实测的四层（设计稿 §1.3），每一层都有
// 明确的数据源，全部来自本 daemon 已有的记忆面，零新存储：
//
//	① 会话元数据   installation 标识 + 会话归属（daemonTurnContextMeta）
//	② 已保存的记忆 summary 槽的摘要行（P1 轮末收敛登记的 compact_digest 行）
//	③ 早期对话摘要 conversation 槽的 Digest（Compact 的覆盖段，含 summary-v1 审计首行）
//	④ 近期消息     conversation 槽未压缩项的尾部（原文，旧→新）
//
// 【为什么是纯函数】组装规则（顺序/裁剪/降级）是策略，数据是输入——分开才能用
// 三根钉（顺序/预算/降级）把策略钉死，也让未来的真实消费方（见 thread.go 的接线
// TODO）只负责取数。预算裁照 ReadWindow 的既有语义（从新到旧保留、单条超预算仍保底），
// 不新发明裁剪机制。
//
// 【未做的一层】spec §7 五段式里的 "relevant artifacts" 没进本轮四层：hybrid 家的
// artifact 槽装的是**索引**（形状指纹/字节，不含正文，D3 红线），拼进提示词只会
// 给模型看一堆 JSON 行；daemon 家本轮根本没有 artifact 产出。等真正出现"可入提示词
// 的产物正文"再扩第五层，不预支抽象。

// 四层的层头。顺序即层序（改动即失效：顺序钉测试按这些字面量断言）。
const (
	ctxLayerMeta   = "[会话元数据]"
	ctxLayerSaved  = "[已保存的记忆]"
	ctxLayerDigest = "[早期对话摘要]"
	ctxLayerMsgs   = "[近期消息]"
)

// turnContextMessage 是第四层（近期消息）的单条消息。
type turnContextMessage struct {
	Role string
	Text string
}

// daemonTurnContextMeta 造第一层（会话元数据）正文：installation 标识 + 会话归属一行。
// hostID 取 Server 的 installation 身份（hostIDFromInstallation），让"这是哪台电脑上的
// 哪个 agent 的哪个会话"在上下文里自证——多设备场景下模型不再靠猜。
func daemonTurnContextMeta(hostID, agentID, threadID, workspace string) string {
	return fmt.Sprintf("host=%s agent=%s thread=%s workspace=%s", hostID, agentID, threadID, workspace)
}

// assembleTurnContext 把四层原料按固定顺序组装成上下文前缀（纯函数）。
//
// 【组装规则】
//   - 层序固定：元数据 → 已保存记忆 → 早期摘要 → 近期消息；层内正文旧→新呈现。
//   - 预算：maxBytes 为总额度（0 = 不限）。额度内**从新到旧保留、超限截断最旧**：
//     saved 行从最旧丢起、消息从最旧丢起；摘要层要么整段在、要么整段让位——
//     半段摘要是谎报（比缺更糟）。元数据是身份行，最先占额度、放不下时截断而不丢弃。
//   - 保底：最新一条消息永远出现，哪怕自身超预算（ReadWindow"单条自身超预算仍返回"
//     的同款纪律——上下文可以穷，不能没有"用户刚说了什么"）。
//   - 降级：任何一层无内容（或一分预算都摊不到），整层连同层头消失；全空输入
//     返回空串。空层头既费预算，又让读者误以为"这里本来该有内容"。
//
// 返回按 "\n" 连接的多行文本；调用方作为整体前缀注入（消费点见 thread.go 接线 TODO）。
func assembleTurnContext(meta string, saved []string, digest string, msgs []turnContextMessage, maxBytes int64) string {
	sink := &ctxLineSink{maxBytes: maxBytes}

	// 层①：会话元数据。身份行最先占额度；放不下时截断（身份残缺好过身份缺席）。
	if meta != "" {
		if room := sink.remaining() - int64(len(ctxLayerMeta)+1); room > 0 {
			body := meta
			if int64(len(body)) > room {
				body = truncateRunesEllipsis(body, room)
			}
			if body != "" {
				sink.tryAdd(ctxLayerMeta)
				sink.forceAdd(body) // 已按 room 裁过，必能放下
			}
		}
	}

	// 层②：已保存的记忆。从最新往回收，遇到第一条放不下的就停（截断最旧），
	// 收到的行再按旧→新呈现——选择和呈现方向解耦，与 ReadWindow 同形。
	if len(saved) > 0 {
		room := sink.remaining() - int64(len(ctxLayerSaved)+1)
		if room > 0 {
			var kept []string
			for i := len(saved) - 1; i >= 0; i-- {
				if int64(len(saved[i]))+1 > room {
					break
				}
				kept = append(kept, saved[i])
				room -= int64(len(saved[i]) + 1)
			}
			if len(kept) > 0 {
				sink.tryAdd(ctxLayerSaved)
				for i := len(kept) - 1; i >= 0; i-- {
					sink.forceAdd(kept[i])
				}
			}
		}
	}

	// 层③：早期对话摘要。整段进出：Compact 的 Digest 是带覆盖记账的连续段落，
	// 截半段等于伪造"覆盖到了 X"的谎报——放不下就让位给更新的层。
	if digest != "" {
		cost := int64(len(ctxLayerDigest) + 1 + len(digest) + 1)
		if sink.maxBytes <= 0 || sink.used+cost <= sink.maxBytes {
			sink.tryAdd(ctxLayerDigest)
			sink.forceAdd(digest)
		}
	}

	// 层④：近期消息。从最新往回收（截断最旧），保底最新一条；旧→新呈现。
	if len(msgs) > 0 {
		render := func(m turnContextMessage) string { return m.Role + ": " + m.Text }
		room := sink.remaining() - int64(len(ctxLayerMsgs)+1)
		var kept []string
		for i := len(msgs) - 1; i >= 0; i-- {
			line := render(msgs[i])
			if int64(len(line))+1 > room {
				break
			}
			kept = append(kept, line)
			room -= int64(len(line) + 1)
		}
		if len(kept) == 0 {
			// 保底：额度再紧,"用户刚说了什么"必须在场（超预算也进，ReadWindow 同纪律）。
			// 此时层头一并省去——裸消息好过"有头无正文"的空层。
			kept = append(kept, render(msgs[len(msgs)-1]))
		} else {
			sink.tryAdd(ctxLayerMsgs) // room 已预扣层头，必能放下
		}
		for i := len(kept) - 1; i >= 0; i-- {
			sink.forceAdd(kept[i])
		}
	}

	return strings.Join(sink.lines, "\n")
}

// ctxLineSink 组装输出行收纳器：逐行记账（每行按 len+1 计入，保守估换行），
// tryAdd 尊重额度、forceAdd 用于已按额度裁过或保底的行。
type ctxLineSink struct {
	lines    []string
	used     int64
	maxBytes int64 // <=0 = 不限
}

func (s *ctxLineSink) remaining() int64 {
	if s.maxBytes <= 0 {
		return 1 << 62
	}
	return s.maxBytes - s.used
}

func (s *ctxLineSink) tryAdd(line string) bool {
	cost := int64(len(line)) + 1
	if s.maxBytes > 0 && s.used+cost > s.maxBytes {
		return false
	}
	s.used += cost
	s.lines = append(s.lines, line)
	return true
}

func (s *ctxLineSink) forceAdd(line string) {
	s.used += int64(len(line)) + 1
	s.lines = append(s.lines, line)
}

// truncateRunesEllipsis 按 rune 截断到 maxBytes 内，截断发生时以 "…"（3 字节）结尾。
// 不按字节硬切——中文正文切在半个 rune 上会产生非法 UTF-8，混进提示词就是脏输入。
func truncateRunesEllipsis(s string, maxBytes int64) string {
	if int64(len(s)) <= maxBytes {
		return s
	}
	const mark = "…"
	reserve := int64(len(mark))
	if maxBytes < reserve {
		reserve = 0 // 连省略号都放不下：硬裁，不再装饰
	}
	limit := maxBytes - reserve
	var b strings.Builder
	for _, r := range s {
		if int64(len(b.String()))+int64(len(string(r))) > limit {
			break
		}
		b.WriteRune(r)
	}
	if reserve > 0 {
		b.WriteString(mark)
	}
	return b.String()
}

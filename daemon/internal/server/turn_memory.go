package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// turn_memory.go —— 让统一 Runtime 的记忆/checkpoint 在**真实轮次**里被使用
// （方案 §8-D7「所有 Agent 接入统一 Runtime」）。
//
// 【为什么接线在 server，而不是各 provider 自己的 Run 里】轮次主体 runTurnOnce 是
// 所有 provider 与跨 agent 委派引擎共用的心脏（thread.go 的注释明说"daemon 绝不重写
// 轮次循环"）。写入放在这里之后，"现在有哪几家在用统一 Runtime"就不再是一份需要维护
// 的名单：新加一家 provider 自动入列，与 CollabAware / PrefsAware / NativeTurnReader
// 的接口探测纪律同源（README §7「静态名单是债」）。逐家改 provider 会产出五份重复
// 实现，而且"谁接了"这件事只在那五个文件里才看得见。
//
// 【写什么由 provider 申报的 MemoryModel 决定，不由 agent 名字决定】MemoryModel 是
// 各家在 runtime_descriptors.go 里自报的形态，其枚举注释即是判据（见
// runtime/descriptor.go）：
//
//   - memory.daemon：daemon 侧 conversation memory 是主要记忆 → 写会话**原文**
//     （用户 / assistant 正文）。没有原生会话的家走这条（echo、OpenAI 兼容）。
//   - memory.hybrid：**原生存正文，daemon 存摘要与索引** → 绝不重复写正文，只写一条
//     **轻量索引**（turn_id / role / 块类型与字节数 / 形状指纹），外加每轮 checkpoint
//     （"恢复状态"）。codex / pi / zcode / dsh 走这条。
//   - memory.native：正文全在原生侧 → 不写（写了也没有读者，只是磁盘开销）。
//   - 未申报（provider 没实现 runtime.Participant）→ 不写。沉默缺失不等于它有记忆；
//     猜着写会把别家的数据记到它名下，而这种错在下游看起来完全正常。
//
// 【索引为什么借用 MemoryArtifact 槽】规格 §6.3 只有 working / conversation /
// summary / artifact 四层，"索引"没有独立槽位。artifact 是唯一"面向机器、既非对话
// 正文也非摘要"的槽，而规格给 ArtifactMemory 定义的字段（turn_id / agent_id / kind /
// size / hash）与索引条目要记的东西本来就重叠。条目自带 Meta["entry"]=turn_index
// 自我描述，将来若规格补出独立槽位，按这个标记迁移即可（不必事后猜哪些条目是索引）。
//
// 【失败不阻断轮次】与 echo 原先的 recordTurn 同一条纪律：记忆是加法，写不进去不该
// 让用户看不到回复。但绝不静默 —— 日志带 agent / thread / role 与原因。

// 索引条目的自我描述与字段键（Meta 是 string→string，客户端与诊断页按这些键读）。
const (
	metaEntryKind  = "entry"      // 条目的种类标记
	metaTurnID     = "turn_id"    // 哪一轮
	metaBlocks     = "blocks"     // 块类型计数，如 "text=1,thinking=1"
	metaBytes      = "bytes"      // 该条消息的块字节合计
	metaSHA256     = "sha256"     // 内容形状指纹（不含正文）
	entryTurnIndex = "turn_index" // metaEntryKind 的取值：轮次索引
)

// recordTurnMemory 把一条**已落库**的轮次消息登记进统一 Runtime 的语义记忆。
//
// 调用点与"消息落库"同点（StartTurn 的 user 消息、runTurnOnce 的两处 assistant
// 消息）：记忆是消息的下游，两者必须看到同一份内容 —— 分开取会在"空正文补偿"这类
// 路径上给出两个事实（补偿后的正文落了库，记忆里却还是空的）。
func (s *Server) recordTurnMemory(provider agent.Provider, t *store.Thread, role, turnID string, blocks []pb.ContentBlock) {
	if s == nil || s.runtimeBackend == nil || t == nil || provider == nil {
		return
	}
	ctx := context.Background()
	var key runtime.MemoryKey
	switch memoryModelOf(provider) {
	case runtime.MemoryDaemon:
		key = runtime.MemoryKey{AgentID: provider.ID(), ThreadID: t.ID, Kind: runtime.MemoryConversation}
		text := blocksPlainText(blocks)
		if strings.TrimSpace(text) == "" {
			// 没有正文可记：不写空条目（空条目只会让诊断页的计数虚高，
			// 而"这家记了 N 条"是用户在判断它有没有用上记忆的唯一依据）。
			return
		}
		if _, err := s.runtimeBackend.Memory().Append(ctx, key, runtime.MemoryItem{
			Kind: runtime.MemoryConversation, Role: role, Payload: []byte(text),
		}); err != nil {
			log.Printf("[runtime] %s 会话记忆写入失败: thread=%s role=%s err=%v", provider.ID(), t.ID, role, err)
			return
		}
	case runtime.MemoryHybrid:
		key = runtime.MemoryKey{AgentID: provider.ID(), ThreadID: t.ID, Kind: runtime.MemoryArtifact}
		payload, meta, ok := turnIndexEntry(turnID, role, blocks)
		if !ok {
			return
		}
		if _, err := s.runtimeBackend.Memory().Append(ctx, key, runtime.MemoryItem{
			Kind: runtime.MemoryArtifact, Role: role, Payload: payload, Meta: meta,
		}); err != nil {
			log.Printf("[runtime] %s 轮次索引写入失败: thread=%s role=%s err=%v", provider.ID(), t.ID, role, err)
			return
		}
	default:
		// memory.native（正文在原生侧）与未申报（不知道它有没有记忆）：都不写。
		return
	}
	// 每轮落一次 checkpoint（规格 line 423 的"恢复状态"）：daemon 重启后由
	// "首次触碰即装载"那条路径读回（见 runtime/backend.go 的 hydrate）。
	// 失败只记日志 —— 轮次已经跑完，不该因为落盘失败回滚用户已经看到的结果。
	if err := s.runtimeBackend.Memory().Checkpoint(ctx, key); err != nil {
		log.Printf("[runtime] %s 记忆 checkpoint 失败: thread=%s kind=%s err=%v", provider.ID(), t.ID, key.Kind, err)
	}
}

// memoryModelOf 取 provider 申报的记忆形态；未申报返回零值（""）。
//
// 【为什么用类型断言而不是把 Descriptor 加进 agent.Provider】Provider 是"接哪家
// 大模型"的最小抽象；记忆归属是运行时段契约（runtime.Participant）的一部分。与
// server 探测 Steerer / PrefsAware / NativeTurnReader 同款取舍：想接就实现，不接
// 不必被迫实现一个返回空值的空方法。
func memoryModelOf(p agent.Provider) runtime.MemoryModel {
	part, ok := p.(runtime.Participant)
	if !ok {
		return ""
	}
	return part.Descriptor().MemoryModel
}

// blocksPlainText 把内容块里的文本拼成记忆原文；非文本块以占位符体现 ——
// 否则"整条记忆只剩文字"，事后看不出这一轮还有图或工具产物。
// （语义与 echo 原 recordTurn 里的实现一致，接线收归后由这里统一承担。）
func blocksPlainText(blocks []pb.ContentBlock) string {
	var sb strings.Builder
	for i := range blocks {
		b := &blocks[i]
		if b == nil {
			continue
		}
		switch v := b.Block.(type) {
		case *pb.ContentBlock_Text:
			sb.WriteString(v.Text.GetText())
		default:
			sb.WriteString("[非文本内容]")
		}
	}
	return sb.String()
}

// turnIndexEntry 造一条**不含正文**的轮次索引条目。
//
// 【为什么不写正文】hybrid 家的原生存着正文（descriptor.go 的枚举注释）；daemon 再
// 存一份就是同一事实的两个副本 —— 磁盘与记忆预算翻倍，而且两份会在原生侧压缩/编辑
// 之后分叉，届时"哪份是对的"没有答案。daemon 侧只需要知道"这一轮存在过、内容是什么
// 形状、多大、指纹多少"：够做索引、够做诊断、够做"正文是不是丢了"的对账。
//
// 【为什么指纹取"形状"而不是内容】把块类型与字节数按序喂进哈希：既能在不存正文的
// 前提下区分两个不同的轮次，又不会把正文泄漏进摘要。
//
// 返回 (载荷, Meta, 是否有内容可记)。
func turnIndexEntry(turnID, role string, blocks []pb.ContentBlock) ([]byte, map[string]string, bool) {
	if len(blocks) == 0 {
		return nil, nil, false
	}
	counts := map[string]int{}
	var totalBytes int
	var shape strings.Builder
	for i := range blocks {
		b := &blocks[i]
		if b == nil {
			continue
		}
		kind := blockKindName(b)
		size := proto.Size(b)
		counts[kind]++
		totalBytes += size
		fmt.Fprintf(&shape, "%s:%d\n", kind, size)
	}
	if totalBytes == 0 && len(counts) == 0 {
		return nil, nil, false
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds) // 决定论：map 遍历顺序随机，不排序会让同一轮两次描述不一致
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	blocksDesc := strings.Join(parts, ",")
	sum := sha256.Sum256([]byte(shape.String()))

	entry := struct {
		TurnID string `json:"turn_id"`
		Role   string `json:"role"`
		Blocks string `json:"blocks"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}{
		TurnID: turnID,
		Role:   role,
		Blocks: blocksDesc,
		Bytes:  totalBytes,
		SHA256: hex.EncodeToString(sum[:8]),
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return nil, nil, false
	}
	meta := map[string]string{
		metaEntryKind: entryTurnIndex,
		metaTurnID:    turnID,
		metaBlocks:    blocksDesc,
		metaBytes:     strconv.Itoa(totalBytes),
		metaSHA256:    entry.SHA256,
	}
	return payload, meta, true
}

// blockKindName 给内容块取一个稳定的类型名（索引与指纹都用它，改动即失效）。
func blockKindName(b *pb.ContentBlock) string {
	switch b.Block.(type) {
	case *pb.ContentBlock_Text:
		return "text"
	case *pb.ContentBlock_Thinking:
		return "thinking"
	case *pb.ContentBlock_Image:
		return "image"
	case *pb.ContentBlock_ToolCall:
		return "tool_call"
	case *pb.ContentBlock_ToolResult:
		return "tool_result"
	default:
		return "other"
	}
}

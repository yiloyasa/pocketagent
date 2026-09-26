package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
// 三个调用点全部在轮次心脏 runTurnOnce 内（用户侧在函数开头、assistant 侧在两处
// 落库点）：记忆是消息的下游，两者必须看到同一份内容 —— 分开取会在"空正文补偿"
// 这类路径上给出两个事实（补偿后的正文落了库，记忆里却还是空的）。
//
// 【为什么用户侧也在心脏里，而不是各调用点的落库处】StartTurn、排队续跑、跨 agent
// 委派、协同成员四条路径都经 runTurnOnce，且都把用户输入原样作为 inputBlocks 传入。
// 写在各调用点会漏掉后两条 —— 实测就是这么漏的：委派与协同自己 append 用户消息后
// 直接进来，于是子线程只剩 assistant 侧记忆（条目数少一半、诊断页偏低）。
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
		// 【预算接线】原文是必需资源：门卫不会因额度拒绝它（越限照写、压力如实上报），
		// 统一走门卫是为了让两条分支的"先记账后写入"形状一眼对齐。
		if !s.memoryBudgetAdmit(provider.ID(), t, runtime.MemoryConversation, int64(len(text))) {
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
		// 【预算接线】索引是可选资源：硬限先拒账、写入就不发生。门卫必须在 Append
		// **之前**——放在写入之后，槽里会留下账上不存在的条目，"拒掉"就成了空话。
		// （门卫记账成功而 Append 失败的相反窗口存在，代价是账面多计一条，日志已出声。）
		if !s.memoryBudgetAdmit(provider.ID(), t, runtime.MemoryArtifact, int64(len(payload))) {
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

// memoryLeaseID 记忆租约的确定性 id。租约按 thread 生命周期一条（设计稿 P1）：
// 该会话第一个记忆写入 Acquire，其后每笔 Charge——不靠 server 持有状态，
// 幂等 id 让"建过没有"由记账层自己回答（ErrLeaseExists）。
func memoryLeaseID(agentID string, threadID string, kind runtime.MemoryKind) string {
	return "memory:" + agentID + ":" + threadID + ":" + string(kind)
}

// memoryBudgetAdmit 把一次记忆写入的字节数记进 ResourceLedger（预算接线，设计稿 P1）。
// 返回 false = 硬限拒绝了这条**可选**写入，调用方必须跳过 Append。
//
// 【可选/必需照搬既有语义】backend.go 的 Acquire/Charge 早已定义："硬上限只拒绝可选
// 资源，必需资源允许越限记账，由 Usage 如实反映压力"。本函数只是把 memory 写入接上
// 那套语义，不新发明预算机制：conversation 原文 = 必需（Optional=false，越限照写）；
// artifact 索引 = 可选（Optional=true，硬限拒新不删旧——删除永远不是记账层的职权）。
//
// 【额度映射】策略模型对每类资源只给一个数（规格 §6.5 基线形态，见 openTurnLease 的
// 同款取舍），这里把它当**硬限**（额度 = "拒绝新增可选资源"的那条线），软限取其半。
// 两线必须分离：soft == hard 时 ClassifyPressure 在额度处直接判 hard，Usage 永远不会
// 出现 OverSoft，"软限触发收敛"就永远没有触发点。取半是待校准的初始值，与
// memorySlotThresholds 的阈值表同地位。
//
// 【记账失败不拦写入】与 openTurnLease 同一纪律：记账是诊断增强，除"硬限拒绝可选写"
// 这条契约行为外，任何记账故障都放行并留日志。
func (s *Server) memoryBudgetAdmit(agentID string, t *store.Thread, kind runtime.MemoryKind, bytes int64) bool {
	if s.runtimeBackend == nil || t == nil || bytes <= 0 {
		return true
	}
	var resKind runtime.ResourceKind
	var optional bool
	switch kind {
	case runtime.MemoryConversation:
		resKind, optional = runtime.ResConversation, false
	case runtime.MemoryArtifact:
		resKind, optional = runtime.ResArtifactDisk, true
	default:
		// summary 槽是收敛自己的产物：入账会让"压缩响应"反过来推高压力账，
		// 形成"收敛→记账→压力→再收敛"的自激；它的量由自己的阈值兜底，不入账。
		return true
	}
	limits := s.effectiveLimits(agentID, t.Workspace, t.ID, "")
	quota := derefInt64(limits.DiskConversationBytes)
	if resKind == runtime.ResArtifactDisk {
		quota = derefInt64(limits.DiskArtifactBytes)
	}
	leaseID := memoryLeaseID(agentID, t.ID, kind)
	ctx := context.Background()
	_, err := s.runtimeBackend.Resources().Acquire(ctx, runtime.LeaseRequest{
		LeaseID:       leaseID,
		OwnerScope:    runtime.ScopeThread(t.ID),
		Kind:          resKind,
		ReservedBytes: bytes,
		SoftLimit:     quota / 2,
		HardLimit:     quota,
		// 软限触达的处置就是压缩——即 memory_housekeeping.go 轮末钩子的动作，
		// SpillPolicy 在租约上把这个契约显式记下来（诊断页可见，不只靠约定）。
		SpillPolicy: runtime.SpillCompact,
		Optional:    optional,
	})
	switch {
	case err == nil:
		// 新租约：ReservedBytes 即首笔记账。
	case errors.Is(err, runtime.ErrLeaseExists):
		if _, cerr := s.runtimeBackend.Resources().Charge(ctx, leaseID, bytes); cerr != nil {
			if errors.Is(cerr, runtime.ErrOverHardLimit) {
				log.Printf("[runtime] %s 记忆写入被硬上限拒绝（可选资源，不删旧账）: thread=%s kind=%s hard=%d",
					agentID, t.ID, kind, quota)
				return false
			}
			log.Printf("[runtime] %s 记忆记账失败（写入照常）: thread=%s kind=%s err=%v", agentID, t.ID, kind, cerr)
		}
	case errors.Is(err, runtime.ErrOverHardLimit):
		log.Printf("[runtime] %s 记忆写入被硬上限拒绝（可选资源）: thread=%s kind=%s hard=%d",
			agentID, t.ID, kind, quota)
		return false
	default:
		log.Printf("[runtime] %s 记忆租约申请失败（写入照常）: thread=%s kind=%s err=%v", agentID, t.ID, kind, err)
	}
	return true
}

// releaseThreadMemoryLeases 释放该会话名下的记忆租约（清空会话记忆时调用）。
// 记忆清了账也得清：残留的活跃租约会让诊断页显示"没有记忆却有压力"——预算联动
// 的账必须跟得上事实。没有租约（该会话从未写过）按 ErrLeaseNotFound 忽略，
// "没有可放的账"不是错误。
func (s *Server) releaseThreadMemoryLeases(agentID, threadID string) {
	if s.runtimeBackend == nil {
		return
	}
	ctx := context.Background()
	for _, kind := range []runtime.MemoryKind{runtime.MemoryConversation, runtime.MemoryArtifact} {
		if err := s.runtimeBackend.Resources().Release(ctx, memoryLeaseID(agentID, threadID, kind)); err != nil && !errors.Is(err, runtime.ErrLeaseNotFound) {
			log.Printf("[runtime] %s 记忆租约释放失败: thread=%s kind=%s err=%v", agentID, threadID, kind, err)
		}
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

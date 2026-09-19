// thread_thinking.go —— 思考流的落库(设置页第 4 项:"隐藏推理过程持久化")。
//
// 【根因】provider.Run 的返回值里只有**最终文本**(codex 侧 blocks 只装一个
// TextBlock),思考过程是边跑边推的 ThinkingDelta"过路帧",手机收到后只活在
// 内存里。于是"关掉隐藏推理过程后思考卡能显示"只对**当轮**成立 —— 重启 app
// 或退出会话再进来,历史恢复(GetThreadHistory)读的是 store,里面没有思考
// 文本,思考卡自然消失。用户描述的"关掉后显示,但重启/重进就没了"正是这个。
//
// 【修复】在 StartTurn 的 sink 总线上把 ThinkingDelta 逐字累积,轮次终局时作为
// ThinkingBlock **前置**进 assistant 消息一起落库 —— 历史恢复路径
// (android 端 historyToChatItems 的 hasThinking 分支)本就支持渲染思考块,
// 数据一到卡片即回,客户端零改动。
//
// 【为什么前置】思考总是发生在正文之前(与流式时序一致),恢复后时间线的顺序
// 才和当初直播时看到的一致。TurnCompleted 的 final_blocks 同步带上思考块:
// 手机端终局帧只提取文本(finalText),不会被重复渲染。
//
// 【隐藏开关仍是一等公民】HideReasoningFor(线程快照/全局)为 true 时,daemon
// 在 provider 层就不再发 ThinkingDelta —— 缓冲区收不到东西,落库自然也没有
// 思考块。"隐藏推理"同时作用于"推送"与"落库"两处,语义不变。
package server

import (
	"strings"
	"sync"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"google.golang.org/protobuf/proto"
)

// turnThinkingBuf 累积**一轮**的思考流文本(线程安全)。
//
// 【为什么要有锁】sink 的调用方不止 provider 的主事件流:审批、通知分发等
// 可能来自不同 goroutine(与 sendMu 存在的理由相同),逐字累积不能裸拼。
type turnThinkingBuf struct {
	mu sync.Mutex
	sb strings.Builder
}

func (b *turnThinkingBuf) add(delta string) {
	if delta == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sb.WriteString(delta)
}

// take 取走并清空缓冲:排队续跑(P1-4)的下一轮从空缓冲开始,
// 思考文本不会串轮次。
func (b *turnThinkingBuf) take() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.sb.String()
	b.sb.Reset()
	return s
}

// thinkingRecorder 在既有 sink 外包一层"思考记录器":把过路的思考增量记进
// 缓冲,原帧照常下发(顺序、seq 落盘、发送失败标记全部不受影响)。
// 返回包装后的 sink 与缓冲,缓冲由 StartTurn 在轮次终局时 take 走。
func thinkingRecorder(next agent.Sink) (agent.Sink, *turnThinkingBuf) {
	buf := &turnThinkingBuf{}
	return func(ev *pb.TurnServerMessage) {
		if ev != nil {
			if td := ev.GetThinkingDelta(); td != nil {
				buf.add(td.GetDelta())
			}
		}
		next(ev)
	}, buf
}

// prependThinkingBlock 把思考文本作为 ThinkingBlock 前置进消息块;
// 思考为空(隐藏推理开着 / 该轮没有思考)时原样返回,不制造空块。
//
// 【vet】槽位填充 + proto.Merge 逐字段复制,绕开 pb 消息按值拷贝
// (内嵌 MessageState 含锁,与 derefBlocks 同款纪律)。
func prependThinkingBlock(blocks []pb.ContentBlock, think string) []pb.ContentBlock {
	think = strings.TrimSpace(think)
	if think == "" {
		return blocks
	}
	out := make([]pb.ContentBlock, 0, len(blocks)+1)
	out = append(out, pb.ContentBlock{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: think}}})
	for i := range blocks {
		out = append(out, pb.ContentBlock{})
		proto.Merge(&out[len(out)-1], &blocks[i])
	}
	return out
}

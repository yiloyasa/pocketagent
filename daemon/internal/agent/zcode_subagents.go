package agent

// zcode_subagents.go —— ZcodeProvider 对 0.16.9 session/subagents 的接入:
// 轮次终局拉一次原生子代理清单,交给 server 层落库存根(接线见
// server/zcode_subagent_sync.go)。
//
// 【为什么放在轮次终局而不是会话列表】pi 的子代理事实在磁盘上(ListThreads 时
// 扫文件,零成本);zcode 的对应事实只在 app-server 进程里,列举会话去拉就要
// 为高频页面拉子进程 —— 不做。轮次终局时子进程本来活着,一次 RPC 的成本,
// 且"本轮委派的子代理(含后台继续跑的)"此刻刚好有事实。存根落库后持久可见,
// 列表页无需重复拉取。
//
// 【协议依据】bundle schema(iGt/W5i)+ 空态 live 实测,见 zcodeapp/subagents.go
// 文件头;时序纪律(session/create 后立刻查会 -32004)在轮次终局天然满足。

import (
	"context"
	"log"
	"time"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

const (
	// zcodeSubagentsBudget 单次清单拉取的上界:一次 RPC,不需要会话级宽时限。
	zcodeSubagentsBudget = 20 * time.Second
	// zcodeSubagentsEndedLimit ended 分页取数上限(schema 上限 100;单页 50,
	// 超过的子代理如实少登记 —— 见 fetchSubagentsForTurn 的日志)。
	zcodeSubagentsEndedLimit = 50
)

// SubagentsForSession 拉某 zcode 会话的原生子代理清单。
// 导出目的:server 层测试与后续"列表页碰上活进程就顺手同步"的扩展点;
// 生产主路径是 fetchSubagentsForTurn(轮次终局异步调用)。
// 经 withClientCtx 走生命周期锁:子进程不在场时会拉起(与 HistoryFor 同款语义),
// 失败如实返回,不重试不伪造空清单。
func (p *ZcodeProvider) SubagentsForSession(sessionID string) (zcodeapp.SubagentsResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), zcodeSubagentsBudget)
	defer cancel()
	var out zcodeapp.SubagentsResult
	err := p.withClientCtx(ctx, zcodeStatusBudget, func(ctx context.Context, c *zcodeapp.Client) error {
		var err error
		out, err = c.Subagents(ctx, sessionID, zcodeSubagentsEndedLimit, "")
		return err
	})
	return out, err
}

// fetchSubagentsForTurn 轮次终局后的清单拉取(异步入口):取一页并交给
// SubagentsHandler(由 server 层在装配时接线)。任何失败只留痕 —— 存根登记
// 是"有则更好"的增强,绝不影响轮次本身的结果与返回。
func (p *ZcodeProvider) fetchSubagentsForTurn(threadID, sessionID string) {
	if p.SubagentsHandler == nil || sessionID == "" {
		return
	}
	res, err := p.SubagentsForSession(sessionID)
	if err != nil {
		log.Printf("[zcode] 子代理清单拉取失败(轮次后) thread=%s session=%s: %v", threadID, sessionID, err)
		return
	}
	if total := res.Ended.Total; total > len(res.Ended.Items) {
		// 只取了一页:如实说明少了多少,不静默截断成"全部"。
		log.Printf("[zcode] 子代理清单超过单页(%d > %d),本轮只登记最近 %d 条 thread=%s",
			total, len(res.Ended.Items), len(res.Ended.Items), threadID)
	}
	p.SubagentsHandler(threadID, res)
}

// queue.go —— piapp.Client 的 clear_queue 封装(0.87.1 新增 RPC:原生 Esc 式
// 撤销排队输入)。
//
// 【证据】0.87.1 bundle(chunk-OJP47DM6.js,2026-09-25 核对):
//
//	case"clear_queue":return success(id,"clear_queue",session.clearQueue())
//	session.clearQueue(){…清空 _steeringMessages/_followUpMessages…
//	                   …return {steering, followUp}}
//
// 应答 data = {"steering":[原文,…],"followUp":[原文,…]}(数组元素是排队时的
// 消息原文,字符串)。0.85.1 无此命令(调用会 success:false):旧版本本来就不
// 存在"上游原生队列",调用方按失败处理即可,无需版本探测。
package piapp

import (
	"context"
	"encoding/json"
	"time"
)

// ClearedQueue clear_queue 应答:被撤销的排队输入(原文列表)。
type ClearedQueue struct {
	Steering []string `json:"steering"` // 被撤销的插队(steer)输入
	FollowUp []string `json:"followUp"` // 被撤销的排队(follow_up)输入
}

// ClearQueue 撤销当前会话在上游排队中的全部输入(等价 pi TUI 里按 Esc)。
// 上游是纯内存操作毫秒级应答,10s 上界纯兜底(ctx 先到也算失败);失败原样
// 返回错误(含旧版 pi 的"未知命令"类失败,由调用方决定降级话术)。
func (c *Client) ClearQueue(ctx context.Context) (*ClearedQueue, error) {
	raw, err := c.CallCtx(ctx, "clear_queue", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return ParseClearedQueue(raw), nil
}

// ParseClearedQueue 解析 clear_queue 应答(抽成纯函数单测钉形状,与
// ParseSlashCommands 同纪律):缺字段按空切片、脏报文回空对象,不炸调用方。
func ParseClearedQueue(raw json.RawMessage) *ClearedQueue {
	q := &ClearedQueue{Steering: []string{}, FollowUp: []string{}}
	if json.Unmarshal(raw, q) != nil {
		return &ClearedQueue{Steering: []string{}, FollowUp: []string{}}
	}
	if q.Steering == nil {
		q.Steering = []string{}
	}
	if q.FollowUp == nil {
		q.FollowUp = []string{}
	}
	return q
}

package server

// approval_options.go —— 审批卡的「选项/详情」结构化提升与应答选项信封。
//
// 【背景】codex 0.159.2 审批应答新增 acceptForSession(会话内免再问)与
// cancel(拒绝并中止本轮)枚举,审批卡要有第三按钮("本次及本会话允许")。
// provider(codex)把选项表/人话详情写在审批卡 JSON(call.json_args)的
// "options"/"details" 键里 —— 那是旧手机也能原样收到、且对未知键免疫的通道;
// 这里再把它们提升为 ApprovalRequest.options / ApprovalRequest.details 结构化
// 字段(proto 只增不改),给新端 UI 一个一等契约。旧手机对两者都忽略,仍按
// approved 布尔渲染两按钮 —— 新旧端互不破坏。
//
// 应答方向:新端在 ApprovalDecision.option 回填选项 id;ApprovalHook 的签名
// (bool, string, error) 为全体 provider 共用,不能为 codex 一家改 —— 所以把
// option 连同 note 包装成 {"option":..,"note":..} 信封并入 note 传下去,由
// agent.parseApprovalChoice 解析。旧端 option 为空,note 原样透传,行为不变。

import (
	"encoding/json"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// approvalCardExtras 从审批卡 JSON 里提升 options/details(提供方未写则返回零值)。
// 解析失败按"没有附加信息"处理:卡本体(action/reason)照常送达,不受影响。
func approvalCardExtras(call *pb.ToolCallBlock) ([]*pb.ApprovalOption, string) {
	if call == nil || call.GetJsonArgs() == "" {
		return nil, ""
	}
	var raw struct {
		Options []struct {
			Id          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal([]byte(call.GetJsonArgs()), &raw); err != nil {
		return nil, ""
	}
	if len(raw.Options) == 0 && raw.Details == "" {
		return nil, ""
	}
	opts := make([]*pb.ApprovalOption, 0, len(raw.Options))
	for _, o := range raw.Options {
		if o.Id == "" {
			continue // 没有 id 的选项无法应答,不提升
		}
		opts = append(opts, &pb.ApprovalOption{Id: o.Id, Label: o.Label, Description: o.Description})
	}
	return opts, raw.Details
}

// approvalNoteWithOption 把新端回填的 Decision.option 包装成信封并入 note。
// option 为空(旧端)时 note 原样返回,一个字节都不动。
func approvalNoteWithOption(d *pb.ApprovalDecision) string {
	if d == nil || d.GetOption() == "" {
		return d.GetNote()
	}
	env, err := json.Marshal(map[string]string{"option": d.GetOption(), "note": d.GetNote()})
	if err != nil {
		return d.GetNote()
	}
	return string(env)
}

package server

// collab_roles.go —— 协同角色(分工编排)的 RPC 面 + 委派时的角色解析。
//
// 【这一层解决什么】委派原本只能在**每次调用**时传 agent/model/effort/task:
// 父模型要让某个子代理反复扮演同一岗位(例如"审查者":只找安全问题、不改
// 代码、用便宜模型),就得把整段人设在每个 task 里复述一遍,改口径要改 N 处。
// 角色表把"我是谁、长期怎么干"抽出来变成一条配置,父模型每次只用引用 id。
//
// 【这一层刻意不做什么】**不自动编排分工**。谁在什么时候派给谁,由父模型
// (经 MCP 工具入参)或手机端决定。让 daemon 替用户猜分工既不可预测也不可控,
// 那是假智能 —— 本文件只提供"可被引用 / 可被管理"的角色表。
//
// 【为什么解析放在 server 层而不是 collabmcp 引擎层】角色是**配置便利**,
// 不是引擎能力:解析完出来的东西,和父模型直接把值写全的效果完全一样。
// 放进引擎会让引擎多一条"要不要查配置"的分支,且 RPC 之外的入口
// (未来可能的其它调用方)享受不到。**在入口处一次性展开**是最小的改动面。

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// maxCollabRoleIDLen id 长度上限。它不是精算出的安全阈值,而是**防止把整段
// 散文当 id 存进来**:id 要出现在 MCP 工具 JSON 入参里被模型书写,id 越长
// 模型写错的概率越高。超限如实报错,不静默截断 —— 截断会让"我按 XXX 引用却
// 引用不到"变成无法归因的灵异故障。
const maxCollabRoleIDLen = 64

// collabRoleToProto 落库形态 → 协议形态。
func collabRoleToProto(r store.CollabRole) *pb.CollabRole {
	return &pb.CollabRole{
		Id:           r.ID,
		Name:         r.Name,
		AgentId:      r.AgentID,
		Description:  r.Description,
		Instructions: r.Instructions,
		Model:        r.Model,
		Effort:       r.Effort,
		Workspace:    r.Workspace,
	}
}

// protoToCollabRole 协议形态 → 落库形态(id 已 trim)。
func protoToCollabRole(r *pb.CollabRole) store.CollabRole {
	return store.CollabRole{
		ID:           strings.TrimSpace(r.GetId()),
		Name:         strings.TrimSpace(r.GetName()),
		AgentID:      strings.TrimSpace(r.GetAgentId()),
		Description:  strings.TrimSpace(r.GetDescription()),
		Instructions: r.GetInstructions(),
		Model:        strings.TrimSpace(r.GetModel()),
		Effort:       strings.TrimSpace(r.GetEffort()),
		Workspace:    strings.TrimSpace(r.GetWorkspace()),
	}
	// instructions 刻意**不 trim**:它是自由文本(long-term prompt),行首空格
	// 可能是用户有意的排版(列表缩进、代码块缩进)。其余字段都是"值",空格无语义。
}

// ListCollabRoles 列出全部角色(按 id 升序)。
func (s *Server) ListCollabRoles(ctx context.Context, _ *pb.ListCollabRolesRequest) (*pb.ListCollabRolesResponse, error) {
	roles := s.cfg.Store.ListCollabRoles()
	out := make([]*pb.CollabRole, 0, len(roles))
	for _, r := range roles {
		out = append(out, collabRoleToProto(r))
	}
	// 空表返回空数组而不是 nil 的人话:手机端据此区分"还没配"(展示新建引导)
	// 与"取不到"(那会是 gRPC 错误)。两者都不是"未知/错误"的糊涂账。
	return &pb.ListCollabRolesResponse{Roles: out}, nil
}

// SetCollabRole 按 id 幂等 upsert,回显落库后的角色。
func (s *Server) SetCollabRole(ctx context.Context, req *pb.SetCollabRoleRequest) (*pb.SetCollabRoleResponse, error) {
	if req.GetRole() == nil {
		return nil, errInvalidArg("缺少协同角色内容:role 不能为空")
	}
	r := protoToCollabRole(req.GetRole())
	if err := validateCollabRole(r); err != nil {
		return nil, err
	}
	if err := s.cfg.Store.SetCollabRole(r); err != nil {
		return nil, errInternal(err)
	}
	// 回显用**读回来**的真值,而不是把入参原样返回:iđ trim 过(用户填的
	// " reviewer " 会落成 "reviewer"),手机端拿到什么就显示什么,不会
	// "看起来存进去了但引用时又说没有"。
	return &pb.SetCollabRoleResponse{Role: collabRoleToProto(r)}, nil
}

// DeleteCollabRole 删除角色;id 不存在视为已达成目标(幂等成功)。
func (s *Server) DeleteCollabRole(ctx context.Context, req *pb.DeleteCollabRoleRequest) (*pb.DeleteCollabRoleResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, errInvalidArg("缺少要删除的协同角色 id")
	}
	if err := s.cfg.Store.DeleteCollabRole(id); err != nil {
		return nil, errInternal(err)
	}
	return &pb.DeleteCollabRoleResponse{}, nil
}

// validateCollabRole 校验角色可否落库;返回的错误必须是人话。
//
// 【为什么只校验 id,不校验 agent_id】见 store.CollabRole.AgentID 的注释:
// 存在性裁决统一留在委派的那一刻(CreateThread 动态列举当前可用 agent),
// 这里重复校验会把"今天装了明天卸载"变成存不进去。
func validateCollabRole(r store.CollabRole) error {
	if r.ID == "" {
		return errInvalidArg("协同角色 id 不能为空:id 是引用键,父模型和手机端都靠它引用这条配置,请给一个简短稳定的标识(如 reviewer)")
	}
	if len([]rune(r.ID)) > maxCollabRoleIDLen {
		return errInvalidArg(fmt.Sprintf("协同角色 id 过长(%d 字,上限 %d):id 会被写进委派工具的参数里,请用简短标识",
			len([]rune(r.ID)), maxCollabRoleIDLen))
	}
	// 空格会破坏"引用"这件事:模型大概率写不出带词边界的 id,手机端选择框也
	// 容易把 "my role" 显示成一个词的样子。星号/hash 这类则无实际危害,不禁止。
	if strings.ContainsAny(r.ID, " \t\r\n") {
		return errInvalidArg("协同角色 id 不能含空格:请用连字符或下划线连接(如 code-reviewer)")
	}
	return nil
}

// ============ 委派时的角色解析 ============

// resolvedDelegate 角色展开后的委派参数。
//
// 【优先级】请求里**显式**给的字段 > 角色里的同名配置 > (两者都空时的)默认。
// 为什么请求优先:角色回答"长期我是谁",本次请求回答"这次要破例一次"。
// 若反过来让角色盖住显式入参,父模型就永远没法对单个任务临时换模型/换维度
// —— 那会逼它去改配置,比临时覆盖糟得多。
type resolvedDelegate struct {
	agentID   string
	task      string // 已按需拼接角色人设块
	workspace string
	model     string
	effort    string
}

// resolveDelegateWithRole 把请求 + 可选角色展开成引擎能直接吃的参数。
//
// roleID 为空 → 纯透传(与加本功能之前完全一致的语义,不影响既有调用)。
// roleID 非空但查不到 → **如实报错**并附上当前可用 id,绝不回落到"假装没配":
// 静默回落会让父模型永远发现不了自己把角色名写错了(它会以为人设已生效)。
func (s *Server) resolveDelegateWithRole(req *pb.DelegateSubTaskRequest) (*resolvedDelegate, error) {
	roleID := strings.TrimSpace(req.GetRoleId())
	out := &resolvedDelegate{
		agentID:   strings.TrimSpace(req.GetAgentId()),
		task:      req.GetTask(),
		workspace: strings.TrimSpace(req.GetWorkspace()),
		model:     strings.TrimSpace(req.GetModel()),
		effort:    strings.TrimSpace(req.GetEffort()),
	}
	if roleID == "" {
		return out, nil
	}
	role, ok := s.cfg.Store.GetCollabRole(roleID)
	if !ok {
		return nil, errRoleUndefined(roleID, s.cfg.Store.ListCollabRoles())
	}
	if out.agentID == "" {
		out.agentID = strings.TrimSpace(role.AgentID)
	}
	if out.workspace == "" {
		out.workspace = strings.TrimSpace(role.Workspace)
	}
	if out.model == "" {
		out.model = strings.TrimSpace(role.Model)
	}
	if out.effort == "" {
		out.effort = strings.TrimSpace(role.Effort)
	}
	out.task = collabRoleTask(role, req.GetTask())
	return out, nil
}

// collabRoleTask 把角色的人设前置接到子任务之前(纯函数,便于单测)。
//
// 【为什么是"拼进 prompt"而不是找原生 system prompt 通道】并非所有原生 agent
// 都提供 per-turn 的 system prompt 通道。若起个叫 system_prompt 的字段却只在
// 部分 agent 上生效,那就是**假能力**;拼进 prompt 这条路径对任何 agent 都成立,
// 语义也如实:它确实是给这次任务的一段前置说明。
//
// 【为什么没有** pieces 都为空时不加任何块】空壳注释只在消耗 token,却传达不了
// 任何信息 —— 宁可什么都不加,也不制造噪音。
func collabRoleTask(role store.CollabRole, task string) string {
	var b strings.Builder
	if strings.TrimSpace(role.Description) != "" || strings.TrimSpace(role.Instructions) != "" {
		name := strings.TrimSpace(role.Name)
		if name == "" {
			name = role.ID // 没起名字就回落 id,不伪造"未命名"之类的人造默认值
		}
		b.WriteString("[协同角色] " + name + " (id=" + role.ID + ")\n")
		b.WriteString("以下由 PocketAgent 在委派时注入,用于说明你这次扮演的岗位:\n")
		if d := strings.TrimSpace(role.Description); d != "" {
			b.WriteString("职责:" + d + "\n")
		}
		if i := strings.TrimSpace(role.Instructions); i != "" {
			b.WriteString("长期要求:" + i + "\n")
		}
		b.WriteString("--- 以下是本次具体任务 ---\n")
	}
	b.WriteString(task)
	return b.String()
}

// errRoleUndefined 未定义角色的人话错误,顺带列出当前可用 id。
//
// 【为什么要列出可用 id】父模型拿到它就能在下一次调用里自纠,而不是反复重试
// 同一个错误 id 直到耗尽轮次。这是"错误要能指引下一步"的老规矩。
// 一个都没配时给出的是"还没配过"而不是空列表,避免让人以为查询失败。
//
// 【为什么返回**纯 error**而不是 errInvalidArg(grpc status)】本错误唯一的
// 去处是 DelegateSubTaskResponse.Error —— 它要被原样交给**父模型**读。
// 若用 status.Error,err.Error() 会带上 "rpc error: code = InvalidArgument
// desc = " 这段传输层前缀:对 RPC 客户端有意义,对读报错文案的模型只是噪音,
// 还把"这是业务失败"和"这是协议层失败"混成一件事故。所以这里用纯 error;
// 真正的 API 调用(SetCollabRole / DeleteCollabRole)才走 status。
func errRoleUndefined(id string, available []store.CollabRole) error {
	if len(available) == 0 {
		return fmt.Errorf("未定义的协同角色 %q:当前还没有配置过任何协同角色(可在 App 的协同配置里新建)", id)
	}
	ids := make([]string, 0, len(available))
	for _, r := range available {
		ids = append(ids, r.ID)
	}
	return fmt.Errorf("未定义的协同角色 %q:当前可用的是 %s", id, strings.Join(ids, "、"))
}

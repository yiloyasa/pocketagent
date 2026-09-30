// codex_requests.go —— codex app-server 的「反向请求」(server → client)分派。
//
// 【为什么需要单独一个文件】反向请求是 codex 唯一会**阻塞等待我们答复**的通道。
// 以前这里只有一句 `strings.Contains(method, "approval")`,导致两个真实故障:
//
//	① 所有审批都被包装成同一条 "shell 命令",用户点"允许"时根本不知道自己在允许什么;
//	② 非审批类请求(token 刷新 / agent 反问 / MCP 询问 / 动态工具)被统一塞了一个
//	   `{"decision":"decline"}`。这不是"拒绝",这是**用错误的报文形状污染了另一种语义** ——
//	   token 刷新的期望应答是 tokens,拿到 decision 后 codex 只能按失败处理。
//
// 所以现在的规则是三条:
//
//  1. 按方法名白名单分派,不做子串匹配;
//  2. 审批类 → 构造该类型**专属**的审批卡(名称 + 类型特有字段),应答用该协议版本
//     规定的枚举(v2 是 accept/decline,旧版是 approved/denied —— 混用会被拒);
//  3. 我们确实没有能力的 → 回 JSON-RPC error,绝不伪造一个 decision。
package agent

import (
	"encoding/json"
	"sort"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// codexRequestKind 一条反向请求的处理类别。
type codexRequestKind int

const (
	reqUnknown             codexRequestKind = iota // 未登记:回 error
	reqApprovalCommand                             // 命令执行审批
	reqApprovalFileChange                          // 文件变更审批(v2)
	reqApprovalPatch                               // 补丁审批(旧版,带具体 fileChanges)
	reqApprovalPermissions                         // 权限提升审批
	reqAskUser                                     // agent 反问(item/tool/requestUserInput)
	reqAskMcp                                      // MCP 侧询问(elicitation)
	reqUnsupported                                 // 我们不做:回 error
)

// 审批卡上展示的"动作类型"。Android 端按这个 name 决定标题/副标题/正文渲染方式,
// 所以它必须反映真实动作,**不能**一律用 "shell"。
const (
	toolNameShell      = "shell"
	toolNameFileChange = "file_change"
	toolNamePatch      = "apply_patch"
	toolNamePerms      = "permissions"
	toolNameAskUser    = "request_user_input"
	toolNameAskMcp     = "mcp_elicitation"
	// toolNameWriteStdin 0.159.2 起的第三种命令类审批:item/commandExecution/
	// requestApproval 的 params.kind = "writeStdin"(schema CommandExecutionApprovalKind,
	// 活体实测 kind="command" 见 0.159.2 活体轮探针)。它不是"执行新命令",
	// 而是请求**向已运行的终端写入 stdin**(配套 item/commandExecution/
	// terminalInteraction 通知)。手机端按 name 渲染,未知 name 走通用卡,
	// 卡内 JSON 如实带 kind/reason —— 绝不冒充 shell 命令执行。
	toolNameWriteStdin = "write_stdin"
)

// classifyCodexRequest 把方法名映射成处理类别(精确匹配,不做子串判断)。
func classifyCodexRequest(method string) codexRequestKind {
	switch method {
	case "item/commandExecution/requestApproval", "execCommandApproval":
		return reqApprovalCommand
	case "item/fileChange/requestApproval":
		return reqApprovalFileChange
	case "applyPatchApproval":
		return reqApprovalPatch
	case "item/permissions/requestApproval":
		return reqApprovalPermissions
	case "item/tool/requestUserInput":
		return reqAskUser
	case "mcpServer/elicitation/request":
		return reqAskMcp
	case "item/tool/call", "account/chatgptAuthTokens/refresh", "attestation/generate":
		// 动态工具执行 / ChatGPT 凭证刷新 / attestation 生成:
		// 这三件事要么属于桌面端职责(凭证),要么我们没有对外注册过任何动态工具,
		// 谎报一个决定会污染对方的语义,一律诚实回 error。
		return reqUnsupported
	}
	return reqUnknown
}

// isLegacyApproval 判定一个审批请求该用**旧版应答枚举**(approved/denied)还是
// 新版(accept/decline)。
//
// 【为什么不再枚举方法名】以前这里写死两个旧方法名(`execCommandApproval` /
// `applyPatchApproval`)。codex 未来若再引入第三种旧式审批方法,新方法会掉进
// "按新版应答"的分支、发出对方不认的 accept/decline —— 用户看到的现象是
// **"点了允许却没反应"**,而且很难归因(AUDIT §3.3 点名的"静态名单是债")。
//
// 改成按**协议形态**判定:新版审批一律挂在 `item/` 命名空间下
// (item/commandExecution|fileChange|permissions/requestApproval),
// 旧版是顶层 camelCase 方法名。任一侧将来新增方法都自动落进正确分支,
// 这里不需要再改(也正因如此,它不再是一份"需要维护的名单")。
//
// 注:非审批类方法(如 mcpServer/elicitation/request)不走本函数 ——
// approvalDecision 只对审批类调用。
func isLegacyApproval(method string) bool {
	return !strings.HasPrefix(method, "item/")
}

// approvalDecision 统一的"允许/拒绝" → 该协议版本规定的枚举值。
func approvalDecision(method string, allow bool) string {
	if isLegacyApproval(method) {
		if allow {
			return "approved"
		}
		return "denied"
	}
	if allow {
		return "accept"
	}
	return "decline"
}

// ---- 各类请求的参数结构(只声明我们用得到的字段) ----

// commandApprovalParams 同时覆盖 v2 的 item/commandExecution/requestApproval
// 与旧版 execCommandApproval(v2 用 itemId,旧版用 callId)。
// Kind 是 0.159.2 新增字段:"command"(默认,老服务端不带)= 审批一条新命令;
// "writeStdin" = 请求向运行中的终端写入输入(schema 有 default,解析不到不报错)。
type commandApprovalParams struct {
	ApprovalId string          `json:"approvalId"`
	CallId     string          `json:"callId"`
	ItemId     string          `json:"itemId"`
	Kind       string          `json:"kind"`
	Command    json.RawMessage `json:"command"` // 新版字符串 / 旧版数组,都兼容
	Cwd        string          `json:"cwd"`
	Reason     string          `json:"reason"`
}

// fileChangeApprovalParams v2 的文件变更审批。注意它**不带**文件列表 ——
// 变更内容在同一回合的 item/fileChange/patchUpdated 通知里,由调用方补齐。
type fileChangeApprovalParams struct {
	ItemId    string `json:"itemId"`
	Reason    string `json:"reason"`
	GrantRoot string `json:"grantRoot"`
}

// patchApprovalParams 旧版 applyPatchApproval,自带 fileChanges 映射。
type patchApprovalParams struct {
	CallId      string                     `json:"callId"`
	Reason      string                     `json:"reason"`
	GrantRoot   string                     `json:"grantRoot"`
	FileChanges map[string]json.RawMessage `json:"fileChanges"`
}

// permissionsApprovalParams 权限提升审批:申请额外的文件系统/网络权限。
type permissionsApprovalParams struct {
	ItemId      string          `json:"itemId"`
	Reason      string          `json:"reason"`
	Cwd         string          `json:"cwd"`
	Permissions json.RawMessage `json:"permissions"`
}

// askUserOption 一个可选项(label 是给手机按钮用的短文案)。
type askUserOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// askUserQuestion 一条待用户回答的问题。isSecret/isOther 是 schema
// (ToolRequestUserInputQuestion)的原生字段:isSecret 表示该问题应作密码态
// 输入渲染,isOther 表示允许"其他"自由作答 —— 原样透出到卡 JSON,渲染策略
// 归手机(App 忽略未知字段不受影响)。
type askUserQuestion struct {
	Header   string          `json:"header"`
	Id       string          `json:"id"`
	Question string          `json:"question"`
	IsSecret bool            `json:"isSecret"`
	IsOther  bool            `json:"isOther"`
	Options  []askUserOption `json:"options"`
}

// askUserParams 覆盖 item/tool/requestUserInput 与 mcpServer/elicitation/request。
type askUserParams struct {
	ItemId     string            `json:"itemId"`
	Message    string            `json:"message"`
	ServerName string            `json:"serverName"`
	Mode       string            `json:"mode"`
	Questions  []askUserQuestion `json:"questions"`
}

// buildApprovalCall 按类别构造审批卡。
//
// changedFiles 只在文件变更/补丁两类里用到:它们是"用户点允许时会改哪些文件"的唯一来源,
// 少了它审批卡就回到了"用户不知道自己在允许什么"的老问题。
// 【vet】返回指针:pb 消息含锁,调用方按值接住再传 hook 会被 copylocks 拒绝;
// 直接返回新构造的指针,内容与原值语义完全一致。
func buildApprovalCall(kind codexRequestKind, method string, params json.RawMessage, changedFiles []string) *pb.ToolCallBlock {
	switch kind {
	case reqApprovalCommand:
		var a commandApprovalParams
		_ = json.Unmarshal(params, &a)
		id := a.CallId
		if id == "" {
			id = a.ItemId
		}
		if id == "" {
			id = a.ApprovalId
		}
		// 0.159.2 kind=writeStdin:向已运行终端写入 stdin 的审批,与"执行新命令"
		// 是两种动作,卡片如实分开 —— 用户该知道他批准的是"写输入",不是跑命令。
		// 应答形状两种 kind 相同(accept/decline,Response schema 未变),无需分叉。
		if a.Kind == "writeStdin" {
			return &pb.ToolCallBlock{
				Id:   id,
				Name: toolNameWriteStdin,
				JsonArgs: mustJSON(map[string]interface{}{
					"kind":   a.Kind,
					"reason": a.Reason,
				}),
			}
		}
		return &pb.ToolCallBlock{
			Id:   id,
			Name: toolNameShell,
			JsonArgs: mustJSON(map[string]interface{}{
				"kind":    a.Kind,
				"command": approvalCommand(a.Command),
				"cwd":     a.Cwd,
				"reason":  a.Reason,
			}),
		}

	case reqApprovalFileChange:
		var a fileChangeApprovalParams
		_ = json.Unmarshal(params, &a)
		return &pb.ToolCallBlock{
			Id:   a.ItemId,
			Name: toolNameFileChange,
			JsonArgs: mustJSON(map[string]interface{}{
				"files":     changedFiles,
				"count":     len(changedFiles),
				"reason":    a.Reason,
				"grantRoot": a.GrantRoot,
			}),
		}

	case reqApprovalPatch:
		var a patchApprovalParams
		_ = json.Unmarshal(params, &a)
		paths := make([]string, 0, len(a.FileChanges))
		for p := range a.FileChanges {
			paths = append(paths, p)
		}
		if len(paths) > 0 {
			sort.Strings(paths) // map 遍历无序,排序保证卡片内容稳定可复现
		} else {
			paths = changedFiles
		}
		return &pb.ToolCallBlock{
			Id:   a.CallId,
			Name: toolNamePatch,
			JsonArgs: mustJSON(map[string]interface{}{
				"files":     paths,
				"count":     len(paths),
				"reason":    a.Reason,
				"grantRoot": a.GrantRoot,
			}),
		}

	case reqApprovalPermissions:
		var a permissionsApprovalParams
		_ = json.Unmarshal(params, &a)
		// permissions 是嵌套对象:原样透传,手机端做人类可读的摘要。
		var perms interface{}
		_ = json.Unmarshal(a.Permissions, &perms)
		return &pb.ToolCallBlock{
			Id:   a.ItemId,
			Name: toolNamePerms,
			JsonArgs: mustJSON(map[string]interface{}{
				"permissions": perms,
				"cwd":         a.Cwd,
				"reason":      a.Reason,
			}),
		}

	case reqAskUser, reqAskMcp:
		var a askUserParams
		_ = json.Unmarshal(params, &a)
		name := toolNameAskUser
		if kind == reqAskMcp {
			name = toolNameAskMcp
		}
		questions := make([]map[string]interface{}, 0, len(a.Questions))
		for _, q := range a.Questions {
			opts := make([]map[string]string, 0, len(q.Options))
			for _, o := range q.Options {
				opts = append(opts, map[string]string{"label": o.Label, "description": o.Description})
			}
			questions = append(questions, map[string]interface{}{
				"id": q.Id, "header": q.Header, "question": q.Question,
				// 密码态/自由作答标注:恒透出,键位稳定供手机按布尔渲染
				// (isSecret → 密码态输入框,isOther → 允许"其他"输入)。
				"isSecret": q.IsSecret, "isOther": q.IsOther,
				"options": opts,
			})
		}
		id := a.ItemId
		if id == "" {
			id = a.ServerName
		}
		return &pb.ToolCallBlock{
			Id:   id,
			Name: name,
			JsonArgs: mustJSON(map[string]interface{}{
				"message":   a.Message,
				"server":    a.ServerName,
				"questions": questions,
			}),
		}
	}
	return &pb.ToolCallBlock{}
}

// ApprovalReason 按动作类型给出人话原因。
//
// 以前这里对所有类型都写死"agent 请求执行危险操作" —— 反问类请求(request_user_input)
// 被贴上这句话,既不准确也让人以为出了什么问题。原因描述跟着动作类型走。
// 【vet】参数为指针:pb 消息按值传会被 copylocks 拒绝;只读访问,nil 安全(GetName 零值返回 "")。
func ApprovalReason(call *pb.ToolCallBlock) string {
	switch call.GetName() {
	case toolNameShell:
		return "agent 请求执行命令"
	case toolNameWriteStdin:
		return "agent 请求向运行中的命令写入输入"
	case toolNameFileChange, toolNamePatch:
		return "agent 请求修改文件"
	case toolNamePerms:
		return "agent 请求提升权限"
	case toolNameAskUser:
		return "agent 需要你回答一个问题"
	case toolNameAskMcp:
		return "MCP 服务需要你确认"
	case "write_file":
		return "agent 请求写入文件"
	case unknownRequestCallName:
		// 【辅线 B】未知反向请求的通用确认卡:外层 Reason 与卡内 reason 键同文案,
		// 手机审批卡两处都如实说"未识别的操作",不冒充任何已知动作类型。
		return "agent 请求执行一个未识别的操作"
	}
	return "agent 请求执行操作"
}

// unsupportedRequestMessage 我们不做某类请求时的说明。
// 文案会进 daemon 日志,方便对照 codex 侧行为排查。
func unsupportedRequestMessage(method string) string {
	switch method {
	case "account/chatgptAuthTokens/refresh":
		return "PocketAgent 不托管 ChatGPT 凭证(登录属桌面端职责),请由 codex CLI/桌面端自行刷新"
	case "item/tool/call":
		return "PocketAgent 未向 codex 注册任何动态工具,无法执行该调用"
	case "attestation/generate":
		return "PocketAgent 不生成 upstream attestation"
	}
	return "PocketAgent 不支持该反向请求"
}

// mergePaths 把新出现的路径并入已有列表:去重且保持首次出现的顺序。
// 同一文件在一回合里被多次 patch 是常态,审批卡上不该出现重复行。
func mergePaths(dst []string, add []string) []string {
	seen := make(map[string]bool, len(dst)+len(add))
	for _, p := range dst {
		seen[p] = true
	}
	for _, p := range add {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		dst = append(dst, p)
	}
	return dst
}

// splitAnswerNote 把手机端提交的自由文本拆成 requestUserInput 期望的 answers 映射。
//
// Android 端约定:应答放在 ApprovalDecision.note 里,内容是
// `{"<questionId>": {"answers": ["答案", ...]}}`;兼容多选直接提交的数组形状
// `{"<questionId>": ["A","B"]}` 与裸字符串 `{"<questionId>": "A"}`(统一归一成
// 应答 schema ToolRequestUserInputAnswer 要求的 {"answers":[...]} 形状)。
// 解析失败时退化为"用整段文本回答第一个问题" —— 让 agent 至少拿到用户的意思,
// 而不是收到一个空回答后开始猜。已是对象的值原样透传(不猜未知形状)。
func splitAnswerNote(note string, questions []askUserQuestion) map[string]interface{} {
	answers := map[string]interface{}{}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return answers
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && len(parsed) > 0 {
		for id, v := range parsed {
			answers[id] = normalizeCodexAnswer(v)
		}
		return answers
	}
	if len(questions) > 0 {
		answers[questions[0].Id] = map[string]interface{}{"answers": []string{trimmed}}
	} else {
		answers["answer"] = map[string]interface{}{"answers": []string{trimmed}}
	}
	return answers
}

// normalizeCodexAnswer 把单条答案归一成应答 schema 认可的形状:
// 数组/字符串 → {"answers":[...]}(codex 要求每题的值必须是带 answers 数组的
// 对象);已是对象的(含旧形状 {"answers":[...]})原样透传;其余标量同理包一层。
func normalizeCodexAnswer(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return t
	case []interface{}:
		return map[string]interface{}{"answers": t}
	case string:
		return map[string]interface{}{"answers": []string{t}}
	default:
		return map[string]interface{}{"answers": []interface{}{v}}
	}
}

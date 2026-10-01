// protocol.go —— dsh ACP 方面方法的类型化封装 + 探针实测形状的纯函数。
//
// 本文件的形状以 P0 探针证据(docs/verify/evidence/dsh-p0-probe.md)为准:
//
//	① set_config_option 的 model value 是双层 JSON 字符串(外层 JSON 串,内容又是
//	  JSON 数组 [route, model])—— EncodeModelTuple/DecodeModelTuple/合法校验纯函数,
//	  切换模型只允许原样回传目录里的编码串,禁止拼裸模型名;
//	② session/resume 应答带 configOptions(与 session/new 同形状),重挂载后据此
//	  刷新模型缓存;
//	③ 缺 key 错误 = JSON-RPC error code=-32603、无 data、message 含 "no API key";
//	④ session/list 条目只有 sessionId + cwd(无 title),重挂载校验按二者比对;
//	⑤ 未知 session/update 变体:本层只透传 RawMessage,变体识别在 agent 层,
//	  不猜形状。
package dshapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// dshInitBudget 是 initialize 握手的默认上界。探针实测冷启动(含首次 profile
// 初始化)=1.415s,8s 给足余量又不至于把生命周期锁拖死。
const dshInitBudget = 8 * time.Second

// Initialize 完成 ACP 握手并返回能力快照。
//
// 参数形状按探针实测:protocolVersion=1 + clientCapabilities.fs 全 false
// (daemon 不给 dsh 提供文件系统回调,工具执行全在子进程沙箱内)。
// authMethods 实测为空 → 无需 authenticate 步骤,凭据链在 harness 内部解析。
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), dshInitBudget)
		defer cancel()
	}
	raw, err := c.CallCtx(ctx, "initialize", map[string]interface{}{
		"protocolVersion": 1,
		"clientCapabilities": map[string]interface{}{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	res := &InitializeResult{Raw: raw}
	if err := json.Unmarshal(raw, res); err != nil {
		return nil, fmt.Errorf("initialize 应答无法解析: %w", err)
	}
	return res, nil
}

// InitializeResult 是 initialize 应答的能力快照(探针第 1 项实测形状)。
type InitializeResult struct {
	ProtocolVersion int `json:"protocolVersion"`
	AgentInfo       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"agentInfo"`
	AgentCapabilities struct {
		PromptCapabilities struct {
			Image           bool `json:"image"`
			Audio           bool `json:"audio"`
			EmbeddedContext bool `json:"embeddedContext"`
		} `json:"promptCapabilities"`
		// sessionCapabilities 是"在场即对象"形状:{"close":{},"list":{},"resume":{}}
		// (0.1.5 新增)。用 RawMessage 接 + helper 判在场,对"未来新增键"零假设。
		SessionCapabilities struct {
			Close  json.RawMessage `json:"close"`
			List   json.RawMessage `json:"list"`
			Resume json.RawMessage `json:"resume"`
		} `json:"sessionCapabilities"`
	} `json:"agentCapabilities"`
	AuthMethods []json.RawMessage `json:"authMethods"`
	Raw         json.RawMessage   `json:"-"`
}

// CanList/CanResume/CanClose 按能力在场性放行(§5.2:一切扩展方法的使用以
// initialize 应答为准,不写死协议假设)。
func (r *InitializeResult) CanList() bool {
	return len(r.AgentCapabilities.SessionCapabilities.List) > 0
}
func (r *InitializeResult) CanResume() bool {
	return len(r.AgentCapabilities.SessionCapabilities.Resume) > 0
}
func (r *InitializeResult) CanClose() bool {
	return len(r.AgentCapabilities.SessionCapabilities.Close) > 0
}

// SupportsImageInput 报告 promptCapabilities.image。能力协商驱动,无版本二分
// 判断:0.2.0-rc.2 起该位改为动态判定(bundle:dsh-acp supportsAcpImagePrompts
// —— 按部署附件服务与所选模型 inputModalities 是否含 "image" 现场解析,任何
// 缺席/失败如实为 false;proto020 探针实测本机路由 false,未广播时收图块被
// 准入层以 "inline image prompts were not advertised" 拒绝)。位翻真的部署上
// 图片输入自动放行。
func (r *InitializeResult) SupportsImageInput() bool {
	return r.AgentCapabilities.PromptCapabilities.Image
}

// SessionState 是 session/new 与 session/resume 的公共应答形状(探针第 2/6 项
// 实测:两者顶层同构,顶层键 = [sessionId, configOptions])。
type SessionState struct {
	SessionID     string         `json:"sessionId"`
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// ConfigOption 是一个配置项(实测:{id,name,category,type,currentValue,options})。
type ConfigOption struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Category     string          `json:"category"`
	Type         string          `json:"type"`
	CurrentValue string          `json:"currentValue"`
	Options      json.RawMessage `json:"options,omitempty"` // 分组叶子结构,原样保留
}

// sessionMCPServers 是 session/new 时要挂载的 MCP 服务器定义。
//
// 【与 pi 的分工同款】dsh 的 ACP 面允许**按会话**传 MCP 服务器(session/new 的
// mcpServers 参数),但会话是 provider 在 agent 层建的,而"本 daemon 的可执行文件
// 路径 + 数据目录 + 回环 token"这些接线事实只有 cmd 层知道 —— 所以由 cmd 层
// 经 SetSessionMCPServers 注入最终状态,本包只负责如实带进请求(与
// piapp.SetCollabEnabled 的分工一模一样)。
// 默认 nil = 不挂任何 MCP,与改造前的 "mcpServers": [] 行为零差异。
var (
	sessionMCPServersMu sync.RWMutex
	sessionMCPServers   []map[string]interface{}
)

// SetSessionMCPServers 由 cmd 层接线:设置 session/new 挂载的 MCP 服务器。
// 传 nil 或无参调用即恢复"不挂载"。
func SetSessionMCPServers(servers []map[string]interface{}) {
	sessionMCPServersMu.Lock()
	defer sessionMCPServersMu.Unlock()
	sessionMCPServers = servers
}

// SessionMCPServers 返回当前待挂载的 MCP 服务器**快照**(深拷贝)。
// 【为什么深拷贝】接线值是全局状态、被每个新会话读取;若把内部 map 直接交出去,
// 任一调用方就地改写就会静默污染后续所有会话。拷贝成本 = 几条小 map,每会话
// 一次,可忽略。
func SessionMCPServers() []map[string]interface{} {
	sessionMCPServersMu.RLock()
	defer sessionMCPServersMu.RUnlock()
	if sessionMCPServers == nil {
		return nil
	}
	out := make([]map[string]interface{}, len(sessionMCPServers))
	for i, m := range sessionMCPServers {
		cp := make(map[string]interface{}, len(m))
		for k, v := range m {
			cp[k] = v
		}
		out[i] = cp
	}
	return out
}

// sessionNewParams 构造 session/new 的参数(纯函数,便于单测断言"接线是否
// 真的带进了请求")。mcpServers 恒为数组:无接线时空数组 —— 与改造前一致。
func sessionNewParams(cwd string) map[string]interface{} {
	servers := SessionMCPServers()
	if servers == nil {
		servers = []map[string]interface{}{}
	}
	return map[string]interface{}{"cwd": cwd, "mcpServers": servers}
}

// session/new:新建持久会话。mcpServers 恒为空数组(默认 nil → 空数组,与改造前
// 零差异)。【为什么不再挂 pocketagent-collab】dsh 的 session/new 的 mcpServers
// 在 dsh 侧"空值接受、非空拒绝"(dsh-acp 文档 + 官方 handbook 一致),且 MCP 实例
// 是**部署级 Cordis 插件**、ACP Session 创建于该固定 Host 组合之后 —— 会话级
// 传非空要么被拒(整条 session 建不起)、要么被静默忽略(挂上不生效),两种结果
// 都不是"真接上"。dsh 的 collab MCP 改走部署级 profile 补丁层
// (collabpatch.go 的 WriteCollabMcpPatch),由 dsh 的固定 Host 组合加载,才是
// 可靠路径。SetSessionMCPServers / SessionMCPServers 仍保留(含单测)作能力面与
// 回归钉子,但 cmd 层已不再调用它们挂 collab。
func (c *Client) SessionNew(ctx context.Context, cwd string) (*SessionState, error) {
	raw, err := c.CallCtx(ctx, "session/new", sessionNewParams(cwd))
	if err != nil {
		return nil, fmt.Errorf("session/new: %w", err)
	}
	return parseSessionState(raw, "session/new")
}

// SessionList 列出可恢复的持久会话(最新在前)。条目无 title(探针修正④)。
func (c *Client) SessionList(ctx context.Context) ([]SessionListEntry, error) {
	raw, err := c.CallCtx(ctx, "session/list", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("session/list: %w", err)
	}
	var res struct {
		Sessions []SessionListEntry `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("session/list 应答无法解析: %w", err)
	}
	return res.Sessions, nil
}

// SessionListEntry 是 session/list 的一条(实测只有 sessionId + cwd)。
type SessionListEntry struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

// session/resume:恢复持久化会话(log 恢复,不重放旧 update —— 探针第 6 项实测
// 重放=0 帧;0.2.0-rc.2 复证不变,跨进程 kill→list→resume 重放仍 0 帧)。
// 应答带 configOptions(探针修正②),调用方据此刷新模型缓存。
//
// 【2026-09-25 升级兼容探针(docs/verify/evidence/dsh-resume-compat-probe.md)】
// rc.3 的 resume 参数 zod 校验**要求 cwd 在场**:单发 {sessionId} 被 -32602
// "Invalid params" 拒绝,data 指明 cwd expected string received undefined ——
// 这就是"dsh 0.1.5-rc.2→rc.3 升级后旧会话发不出消息"的真因(daemon 现行发法
// 恰好只有 sessionId)。补 cwd 后 rc.2 时代的旧会话(同 sessionId、v3 日志)
// 直接恢复成功;rc.2 对多出的 cwd 也接受(P0 探针第 6 项即带 cwd 实测通过)。
// cwd 传空串则回退单参(不伪造目录 —— 调用方拿不出 cwd 时宁可失败走降级)。
//
// 【2026-09-30 0.2.0-rc.2 复证(proto020/resumecompat 探针)】行为延续:cwd 仍
// 必填(缺 cwd 的 -32602 现在带 zod data:{cwd:{_errors:[…]}});应答仍不回显
// sessionId;部署 home 的 57 条 rc.2/rc.3 时代旧会话全部在列且带 cwd 可恢复。
// 失败 message 细分(均 -32602,agent/dsh.go remountSession 据此分类):
// "session is already active: <id>"(已在当前桥内活跃)/"session is not
// resumable: <id>"(持久头缺席)/"session cwd does not match: <cwd>"(bundle,
// 0.2.0 起与持久头做 realpath 同源比对)。
//
// 【应答形状实测】rc.3 的 resume 应答**不再回显 sessionId**(顶层只有
// configOptions;探针 A2 帧全文 2073 字节无 sessionId 键)。探针 A3 实测
// "session is already active: <旧id>"、恢复后对原 id session/prompt 被受理
// (报 no API key 凭证错误而非会话不存在)—— 恢复后的会话仍以**原 id** 寻址,
// 故应答缺 sessionId 时回填请求参数里的 sessionID:这不是猜,是同一 id 的
// 双重实测。rc.2 应答回显同 id,兼容两种形状。
func (c *Client) SessionResume(ctx context.Context, sessionID, cwd string) (*SessionState, error) {
	params := map[string]interface{}{"sessionId": sessionID}
	if cwd != "" {
		params["cwd"] = cwd // rc.3 zod 必填;rc.2 接受(探针证据见上)
	}
	raw, err := c.CallCtx(ctx, "session/resume", params)
	if err != nil {
		return nil, fmt.Errorf("session/resume: %w", err)
	}
	return parseResumeState(raw, sessionID)
}

// parseResumeState 解析 session/resume 应答:与 session/new 不同,rc.3 不回显
// sessionId(探针实测,证据见 SessionResume 注释),恢复后的会话以请求里的
// 原 id 寻址 → 应答缺 sessionId 时回填请求参数,解析坏帧仍显式报错。
func parseResumeState(raw json.RawMessage, requested string) (*SessionState, error) {
	st := &SessionState{}
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, fmt.Errorf("session/resume 应答无法解析: %w", err)
	}
	if st.SessionID == "" {
		st.SessionID = requested
	}
	return st, nil
}

// SessionClose 静默取消+排空+落盘+销毁该会话(DeleteThread 收尾)。
func (c *Client) SessionClose(ctx context.Context, sessionID string) error {
	_, err := c.CallCtx(ctx, "session/close", map[string]interface{}{"sessionId": sessionID})
	if err != nil {
		return fmt.Errorf("session/close: %w", err)
	}
	return nil
}

// SetConfigOption 运行时改配置(model / reasoning_effort)。
//
// 【红线:禁止裸模型名】value 必须原样回传目录(options[].value / currentValue)
// 里的字符串 —— model 的 value 是双层 JSON 编码串(探针修正①),本层不做任何
// 加工;校验交给 ValidModelValue,由调用方决定"非法即拒绝"。
//
// 【2026-09-25 探针修正③:参数名 configId】rc.3 的 zod 校验要求参数名是
// configId;旧实现发 optionId 被 -32602 Invalid params 拒绝(意味着模型/强度
// 切换在 rc.3 上整体失效),见 docs/verify/evidence/dsh-config-persist-probe.md。
// 同探针实证:该改动**仅会话内生效、不持久**(close 后新会话回到原值)。
func (c *Client) SetConfigOption(ctx context.Context, sessionID, optionID, value string) error {
	_, err := c.CallCtx(ctx, "session/set_config_option", map[string]interface{}{
		"sessionId": sessionID,
		"configId":  optionID, // rc.3 实测参数名(configId);取值 = "model" / "reasoning_effort"(id 字段)
		"value":     value,
	})
	if err != nil {
		return fmt.Errorf("session/set_config_option(%s): %w", optionID, err)
	}
	return nil
}

// session/prompt:一轮输入。应答在轮次终局才回来(stopReason),不受 callTimeout
// 管(见 PromptCtx);逐帧过程走 session/update 通知。
// 块形状(ACP v1):{"type":"text","text":...};图片块仅当 promptCapabilities.image
// 为 true 时由上层组装(当前路由实测 false,如实报错不静默丢)。
func (c *Client) Prompt(ctx context.Context, sessionID string, blocks []map[string]interface{}) (json.RawMessage, error) {
	return c.PromptCtx(ctx, "session/prompt", map[string]interface{}{
		"sessionId": sessionID,
		"prompt":    blocks,
	})
}

// Cancel 发 session/cancel 通知(fire-and-forget,无应答)。
func (c *Client) Cancel(sessionID string) {
	_ = c.Notify("session/cancel", map[string]interface{}{"sessionId": sessionID})
}

// parseSessionState 解析 session/new|resume 应答(顶层 sessionId 必须在场,
// 缺了按协议错误报 —— 绝不把解析失败当"空成功")。
func parseSessionState(raw json.RawMessage, method string) (*SessionState, error) {
	st := &SessionState{}
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, fmt.Errorf("%s 应答无法解析: %w", method, err)
	}
	if st.SessionID == "" {
		return nil, fmt.Errorf("%s 应答缺 sessionId: %s", method, clipStr(string(raw), 300))
	}
	return st, nil
}

func clipStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[截断]"
}

// ---- 双层 JSON 编码(探针修正①,纯函数 + 单测钉死)----

// EncodeModelTuple 把 (route, model) 编码成 dsh config-option 期望的 value 字符串:
// 外层是 JSON 字符串,其内容又是 JSON 数组 [route, model]。
// 实测例:["deepseek-official","deepseek-flash"] 的编码串是
// "[\"deepseek-official\",\"deepseek-flash\"]"(探针第 2 项)。
func EncodeModelTuple(route, model string) string {
	b, err := json.Marshal([2]string{route, model})
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodeModelTuple 解码双层编码串,返回 (route, model, ok)。
// 非法输入(裸模型名/非 JSON/元素数≠2/空段)一律 ok=false:不猜 —— 调用方要么
// 原样透传、要么显式报错,绝不把裸名当合法值发出去。
// 注:用 []string 解而非 [2]string —— Go 的 json 对"数组长短与 Go 数组不匹配"
// 不报错(短则补零、长则截断),只有先验长度才能拒绝一元组/三元组。
func DecodeModelTuple(s string) (route, model string, ok bool) {
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return "", "", false
	}
	if len(arr) != 2 || arr[0] == "" || arr[1] == "" {
		return "", "", false
	}
	return arr[0], arr[1], true
}

// ValidModelValue 校验 value 是不是合法的双层编码串;合法则原样返回(不重编码
// —— "原样回传"原则:目录给的字符串按字节回传,避免转义差异)。
// 裸模型名("deepseek-flash")返回 ok=false,set 端必须拒绝。
func ValidModelValue(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	if _, _, ok := DecodeModelTuple(s); !ok {
		return "", false
	}
	return s, true
}

// ---- 模型目录解析(Models() 的唯一数据源,无静态名单)----

// ModelCatalog 是从一份 configOptions 解出的模型面(纯函数,agent 层直接消费)。
type ModelCatalog struct {
	// ModelValue 是当前 model 的双层编码串(currentValue 原样)。
	ModelValue string
	// Models 是可选模型:叶子 value(双层编码串,原样)→ 显示名。
	ModelOptions []ModelOption
	// EffortValue 是当前 reasoning_effort(实测普通字符串 "high",单层)。
	EffortValue string
	// Efforts 是可选档位值(实测 off/low/high/max)。
	Efforts []string
}

// ModelOption 一个可选模型。
type ModelOption struct {
	Value       string // 双层编码串,原样保存(set 时原样回传)
	Name        string // 显示名(如 "DeepSeek-V4-Flash")
	Description string
}

// configOptionIDModel / configOptionIDEffort 是探针实测的 optionId。
const (
	configOptionIDModel  = "model"
	configOptionIDEffort = "reasoning_effort"
)

// ParseModelCatalog 从 configOptions 解析模型面(纯函数)。
// model 选项按 group 分组(实测单层 group {group,name,options:[...]}),叶子
// 层级对未来多 group 天然兼容(遍历全部叶子);找不到的项留零值,不猜。
func ParseModelCatalog(opts []ConfigOption) ModelCatalog {
	var cat ModelCatalog
	for _, o := range opts {
		switch o.ID {
		case configOptionIDModel:
			cat.ModelValue = o.CurrentValue
			cat.ModelOptions = parseModelOptionLeaves(o.Options)
		case configOptionIDEffort:
			cat.EffortValue = o.CurrentValue
			cat.Efforts = parseEffortValues(o.Options)
		}
	}
	return cat
}

// parseModelOptionLeaves 遍历 model 选项树,收集叶子 {value,name,description}。
// 实测形状一层 group 包着叶子;递归下探对"未来多级分组"零假设(按有无 value 判叶子)。
func parseModelOptionLeaves(raw json.RawMessage) []ModelOption {
	if len(raw) == 0 {
		return nil
	}
	var nodes []modelOptionNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil
	}
	var out []ModelOption
	for _, n := range nodes {
		if n.Value != "" {
			out = append(out, ModelOption{Value: n.Value, Name: n.Name, Description: n.Description})
			continue
		}
		// 分组节点:递归其 options
		inner := parseModelOptionLeaves(n.Children)
		out = append(out, inner...)
	}
	return out
}

type modelOptionNode struct {
	Value       string          `json:"value"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Children    json.RawMessage `json:"options"`
}

// parseEffortValues 收集 reasoning_effort 的档位值(叶子 {value,name,description},
// 实测无分组,但同样按"有 value 即叶子"递归,形状漂移不炸)。
func parseEffortValues(raw json.RawMessage) []string {
	leaves := parseModelOptionLeaves(raw)
	out := make([]string, 0, len(leaves))
	for _, l := range leaves {
		out = append(out, l.Value)
	}
	return out
}

// ---- 反向请求(session/request_permission,审批桥的数据面)----

// RequestPermissionParams 是反向请求参数(形状按 2026-09-30 permwrite 探针实帧:
// {"sessionId","toolCall":{"toolCallId"},"options":[{optionId,name,kind}]};
// requestKind 实测缺席/空,容错兼容:字段读取全部容错,解析失败走显式报错路径)。
type RequestPermissionParams struct {
	SessionID string              `json:"sessionId"`
	ToolCall  *PermissionToolCall `json:"toolCall,omitempty"`
	Options   []PermissionOption  `json:"options"`
	Kind      string              `json:"kind,omitempty"`
	// RequestKind 是 ACP 规范侧的同类字段(蓝本/规范写 requestKind,探针实帧缺席)。
	// 双形状兼容:解析出来仅供审批卡展示,选择 allow/reject 只认 options[].kind。
	RequestKind string          `json:"requestKind,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// DisplayKind 返回审批卡展示用的请求类别:实测 kind 与规范 requestKind 双形状,
// 取先到者,都缺给空串(展示层自行兜底,不影响应答)。
func (p *RequestPermissionParams) DisplayKind() string {
	if p.Kind != "" {
		return p.Kind
	}
	return p.RequestKind
}

// PermissionToolCall 是请求里的可选工具上下文(ACP v1:toolCall 可缺席)。
type PermissionToolCall struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	RawInput   json.RawMessage `json:"rawInput"`
}

// PermissionOption 一个授权选项(实测口径:options 数组,kind 字段,optionId)。
type PermissionOption struct {
	OptionID    string `json:"optionId"`
	Name        string `json:"name"`
	Kind        string `json:"kind"` // allow_once / allow_always / reject_once / reject_always
	Description string `json:"description,omitempty"`
}

// ParseRequestPermission 从反向请求 params 解析(容错:坏参返回错误,由上层
// RespondError,绝不伪造 decision —— 红线 2)。
func ParseRequestPermission(params json.RawMessage) (*RequestPermissionParams, error) {
	p := &RequestPermissionParams{Raw: params}
	if err := json.Unmarshal(params, p); err != nil {
		return nil, fmt.Errorf("session/request_permission 参数无法解析: %w", err)
	}
	if len(p.Options) == 0 {
		return nil, errors.New("session/request_permission 缺 options 数组")
	}
	return p, nil
}

// AllowOptionID / RejectOptionID 按 kind 前缀挑选项(精确前缀,不猜名字含义):
// 找不到对应 kind 的选项返回空串,上层必须显式报错而不是瞎选。
func (p *RequestPermissionParams) AllowOptionID() string {
	for _, o := range p.Options {
		if strings.HasPrefix(strings.ToLower(o.Kind), "allow") {
			return o.OptionID
		}
	}
	return ""
}

func (p *RequestPermissionParams) RejectOptionID() string {
	for _, o := range p.Options {
		if strings.HasPrefix(strings.ToLower(o.Kind), "reject") {
			return o.OptionID
		}
	}
	// 兜底:最后一个选项按 ACP 惯例通常是拒绝侧(实测不可,防御性:宁可选中
	// 拒绝侧也不能把"没有显式 reject kind"误报成无法应答 —— 拒绝是失败安全方向)。
	last := p.Options[len(p.Options)-1]
	if strings.HasPrefix(strings.ToLower(last.Kind), "allow") {
		return ""
	}
	return last.OptionID
}

// PermissionOutcomeSelected 构造应答 result:用户做出了选择。
//
// 形状(ACP v1 规范):{"outcome":{"outcome":"selected","optionId":...}}。
// 注:蓝本 §4.3 把判别字段写作 "type",ACP 规范与探针口径(应答
// outcome={outcome:"selected",optionId})是 "outcome"。该链路缺 key 无法实测,
// 以规范为准,真机验证清单里有专项(见包注释与汇报)。
func PermissionOutcomeSelected(optionID string) map[string]interface{} {
	return map[string]interface{}{
		"outcome": map[string]interface{}{"outcome": "selected", "optionId": optionID},
	}
}

// PermissionOutcomeCancelled 构造"取消"应答(超时/断线走拒绝选项而非 cancel
// —— cancel 语义是"用户没答",dsh 侧行为未知;拒绝选项是失败安全且语义明确。
// 本函数保留给"选项表完全无法解析"的兜底)。
func PermissionOutcomeCancelled() map[string]interface{} {
	return map[string]interface{}{
		"outcome": map[string]interface{}{"outcome": "cancelled"},
	}
}

// ---- 错误归一化(探针修正③)----

// IsMissingCredential 按 code+message 归一化缺 key 错误。
// 探针实测:code=-32603、无 data 字段、message="Internal error: turn failed:
// llm-deepseek: no API key for provider route ..."。前缀会随上游措辞漂移,
// 按 "no API key" 子串判(内层 llm 错误的稳定词),有 data 的错误不归此类。
// 【2026-09-30 0.2.0-rc.2 复证(proto020 探针)】形状不变,归一化继续成立。
func IsMissingCredential(err error) bool {
	var re *RPCError
	if !errors.As(err, &re) {
		return false
	}
	if re.Data != nil || re.Code != -32603 {
		return false
	}
	return strings.Contains(re.Message, "no API key")
}

// RPCErrorMessage 提取底层 JSON-RPC 原文(dsh 原生 message 直接给人看,不加工)。
func RPCErrorMessage(err error) (string, bool) {
	var re *RPCError
	if !errors.As(err, &re) {
		return "", false
	}
	return re.Message, true
}

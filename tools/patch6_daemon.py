# -*- coding: utf-8 -*-
# P4 daemon 补丁:codex 设置(approval/sandbox)+ MCP 列表桥接 + thread/name/updated 标题同步
import re

# ---- codex.go ----
p = r'C:\dev\pocketagent\daemon\internal\agent\codex.go'
s = open(p, encoding='utf-8').read()

s = s.replace('''type CodexProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(两者独立,查模型不阻塞轮次)
	turnMu sync.Mutex
	lifeMu sync.Mutex
	client *codexapp.Client
	inited bool''',
'''type CodexProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(两者独立,查模型不阻塞轮次)
	turnMu sync.Mutex
	lifeMu sync.Mutex
	client *codexapp.Client
	inited bool
	// 桌面端设置(手机"设置"页写入,thread/start 时带上)
	setMu         sync.Mutex
	approvalPolicy string
	sandbox        string
	// 标题同步:codex 自动命名线程时回调(server 层落库,双端可见)
	TitleHandler func(codexThreadID, title string)''')

s = s.replace('''	startParams := map[string]interface{}{
		"threadId": codexTID,
		"input":    []map[string]string{{"type": "text", "text": userText.String()}},
	}''',
'''	startParams := map[string]interface{}{
		"threadId": codexTID,
		"input":    []map[string]string{{"type": "text", "text": userText.String()}},
	}
	// 手机设置页配置的桌面端策略
	p.setMu.Lock()
	if p.approvalPolicy != "" {
		startParams["approvalPolicy"] = p.approvalPolicy
	}
	if p.sandbox != "" {
		startParams["sandbox"] = p.sandbox
	}
	p.setMu.Unlock()''')

# 标题同步:捕获 thread/name/updated
s = s.replace('''		case method == "error":''',
'''		case method == "thread/name/updated":
			var n struct {
				ThreadId   string  `json:"threadId"`
				ThreadName *string `json:"threadName"`
			}
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.ThreadName != nil && *n.ThreadName != "" {
				if p.TitleHandler != nil {
					go p.TitleHandler(codexTID, *n.ThreadName)
				}
			}
		case method == "error":''')

# 设置读写 + MCP 列表方法
s = s.replace('''func mustJSON(v interface{}) string {''',
'''// SetDesktopSettings 写入桌面端策略(手机设置页调用)。
func (p *CodexProvider) SetDesktopSettings(approvalPolicy, sandbox string) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	p.approvalPolicy, p.sandbox = approvalPolicy, sandbox
}

// DesktopSettings 读取。
func (p *CodexProvider) DesktopSettings() (approvalPolicy, sandbox string) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return p.approvalPolicy, p.sandbox
}

// McpStatus 桥接 codex 的 listMcpServerStatus:返回 MCP 服务器名与工具数。
func (p *CodexProvider) McpStatus() ([]map[string]interface{}, error) {
	if err := p.withClient(func(c *codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	raw, err := p.client.Call("listMcpServerStatus", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data []struct {
			Name       string `json:"name"`
			ServerInfo *struct {
				Title string `json:"title"`
			} `json:"serverInfo"`
			Tools map[string]interface{} `json:"tools"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return nil, nil
	}
	out := make([]map[string]interface{}, 0, len(res.Data))
	for _, m := range res.Data {
		out = append(out, map[string]interface{}{
			"name":  m.Name,
			"tools": len(m.Tools),
		})
	}
	return out, nil
}

// Account 从模型列表里取登录账号(形如 "Codex · xxx@qq.com" 的账号组条目)。
func (p *CodexProvider) Account() string {
	for _, m := range p.Models() {
		if strings.Contains(m.DisplayName, "·") {
			return m.DisplayName
		}
	}
	return ""
}

func mustJSON(v interface{}) string {''')
open(p, 'w', encoding='utf-8').write(s)
print('codex.go ok')

# ---- server/model.go: 三个 RPC 实现 ----
p = r'C:\dev\pocketagent\daemon\internal\server\model.go'
s = open(p, encoding='utf-8').read()
s = s.rstrip() + '''

// ---- Codex 桌面端设置 / MCP ----

func (s *Server) codexProvider() *agent.CodexProvider {
	if p, ok := s.cfg.Providers["codex"].(*agent.CodexProvider); ok {
		return p
	}
	return nil
}

func (s *Server) GetCodexSettings(ctx context.Context, _ *pb.GetCodexSettingsRequest) (*pb.CodexSettings, error) {
	out := &pb.CodexSettings{}
	p := s.codexProvider()
	if p == nil {
		return out, nil
	}
	approval, sandbox := p.DesktopSettings()
	out.ApprovalPolicy = approval
	out.Sandbox = sandbox
	out.Account = p.Account()
	out.Model = s.cfg.Store.DefaultModelID()
	out.Effort = s.cfg.Store.DefaultEffort()
	return out, nil
}

func (s *Server) SetCodexSettings(ctx context.Context, req *pb.SetCodexSettingsRequest) (*pb.CodexSettings, error) {
	p := s.codexProvider()
	if p != nil {
		p.SetDesktopSettings(req.ApprovalPolicy, req.Sandbox)
	}
	if req.Model != "" {
		_ = s.cfg.Store.SetDefaultModelID(req.Model)
	}
	if req.Effort != "" {
		_ = s.cfg.Store.SetDefaultEffort(req.Effort)
	}
	return s.GetCodexSettings(ctx, &pb.GetCodexSettingsRequest{})
}

func (s *Server) ListMcpServers(ctx context.Context, _ *pb.ListMcpServersRequest) (*pb.ListMcpServersResponse, error) {
	out := &pb.ListMcpServersResponse{}
	p := s.codexProvider()
	if p == nil {
		return out, nil
	}
	list, err := p.McpStatus()
	if err != nil {
		return out, nil // 拉不到就返回空列表(不阻塞设置页)
	}
	for _, m := range list {
		name, _ := m["name"].(string)
		tools, _ := m["tools"].(int)
		out.Servers = append(out.Servers, &pb.McpServerStatus{Name: name, Tools: int32(tools)})
	}
	return out, nil
}
'''
s = s.replace('''import (
	"context"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/agent"
)''',
'''import (
	"context"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/agent"
)''')
open(p, 'w', encoding='utf-8').write(s)
print('server/model.go ok')

# ---- store: DefaultEffort ----
p = r'C:\dev\pocketagent\daemon\internal\store\store.go'
s = open(p, encoding='utf-8').read()
s = s.replace('''func (s *FileStore) SetDefaultModelID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultModelID = id
	return s.saveLocked()
}''',
'''func (s *FileStore) SetDefaultModelID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultModelID = id
	return s.saveLocked()
}
func (s *FileStore) DefaultEffort() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultEffort
}
func (s *FileStore) SetDefaultEffort(e string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultEffort = e
	return s.saveLocked()
}''')
s = s.replace('''		DefaultModelID   string       `json:"default_model_id"`''',
'''		DefaultModelID   string       `json:"default_model_id"`
		DefaultEffort    string       `json:"default_effort"`''')
open(p, 'w', encoding='utf-8').write(s)
print('store ok')

# ---- server/server.go: 接线 TitleHandler(codex 自动命名 → 落库,双端可见)----
p = r'C:\dev\pocketagent\daemon\internal\server\server.go'
s = open(p, encoding='utf-8').read()
s = s.replace('''	return &Server{
		cfg:         cfg,
		hostID:      randomID("host"),
		activeTurns: newActiveTurns(),
	}''',
'''	srv := &Server{
		cfg:         cfg,
		hostID:      randomID("host"),
		activeTurns: newActiveTurns(),
	}
	// codex 自动给线程起名 → 同步到我们的存储(手机/桌面双端一致)
	if p, ok := cfg.Providers["codex"].(*agent.CodexProvider); ok {
		p.TitleHandler = func(codexTID, title string) {
			for _, t := range cfg.Store.List(true) {
				if t.CodexThreadID == codexTID && t.Title != title {
					t.Title = title
					_ = cfg.Store.Update(t)
				}
			}
		}
	}
	return srv''')
open(p, 'w', encoding='utf-8').write(s)
print('server.go ok')

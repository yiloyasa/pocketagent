package server

import (
	"context"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/agent"
)

func (s *Server) ListModels(ctx context.Context, _ *pb.ListModelsRequest) (*pb.ListModelsResponse, error) {
	out := &pb.ListModelsResponse{}
	// 稳定顺序:echo 排最后兜底
	order := []string{"codex", "glm", "deepseek", "anthropic", "echo"}
	for _, id := range order {
		p, ok := s.cfg.Providers[id]
		if !ok {
			continue
		}
		for _, m := range p.Models() {
			m.Id = p.ID() + "/" + m.Id // 全局唯一:"glm/glm-4.7"
			out.Models = append(out.Models, &m)
		}
	}
	return out, nil
}

func (s *Server) SetDefaultModel(ctx context.Context, req *pb.SetDefaultModelRequest) (*pb.Empty, error) {
	if err := s.cfg.Store.SetDefaultModelID(req.ModelId); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) TestModel(ctx context.Context, req *pb.TestModelRequest) (*pb.TestModelResponse, error) {
	p := s.providerFor(req.ModelId)
	if p == nil {
		return &pb.TestModelResponse{Ok: false, Error: "找不到 provider"}, nil
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = "回复 pong 两个字母即可,不要多余内容。"
	}
	start := time.Now()
	blocks, _, err := p.Run(ctx, agent.TurnRequest{
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(pb.TurnServerMessage) {}, nil)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &pb.TestModelResponse{Ok: false, Error: err.Error(), LatencyMs: elapsed}, nil
	}
	reply := ""
	for _, b := range blocks {
		if t, ok := b.Block.(*pb.ContentBlock_Text); ok {
			reply += t.Text.Text
		}
	}
	return &pb.TestModelResponse{Ok: true, Reply: reply, LatencyMs: elapsed}, nil
}

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
		p.SetDesktopSettings(req.ApprovalPolicy, req.Sandbox, req.Model, req.Effort)
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

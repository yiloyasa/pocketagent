// native_config_options_rpc_test.go —— ListAgentConfigOptions 的 server 层契约:
//   - 注册表校验:未知 agent 回 InvalidArgument,文案动态列举已注册名单;
//   - 目录透传:provider 返回的行原样进 pb(不加工);
//   - 读取失败落 resp.Error(RPC 本身不炸,"读不到 ≠ 没有值");
//   - 线程归属校验:thread_id 不属于该 agent → InvalidArgument;
//   - 能力位:实现 NativeConfigOptions 的 provider 带 "agent_native_settings:<id>"。
package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeNativeOptionsProvider 可编程 NativeConfigOptions 假 provider(嵌 seqProvider
// 复用 Provider 接口的其余方法;server 层只走目录方法)。
type fakeNativeOptionsProvider struct {
	seqProvider
	rows        []agent.NativeConfigOption
	err         error
	setOptionID string
	setValue    string
	setErr      error
}

func (p *fakeNativeOptionsProvider) ConfigOptionsCatalog(ctx context.Context, t *store.Thread) ([]agent.NativeConfigOption, error) {
	return p.rows, p.err
}

func (p *fakeNativeOptionsProvider) SetNativeConfigOption(ctx context.Context, t *store.Thread, optionID, value string) error {
	p.setOptionID, p.setValue = optionID, value
	return p.setErr
}

func newNativeOptionsServer(t *testing.T, prov *fakeNativeOptionsProvider) *Server {
	t.Helper()
	return New(Config{
		Store: mustStore(t),
		Providers: map[string]agent.Provider{
			"dsh": prov,
		},
	})
}

func probeNativeRows() []agent.NativeConfigOption {
	return []agent.NativeConfigOption{
		{
			ID: "model", Name: "Model", Category: "model", Type: "select",
			CurrentValue: `["deepseek-official","deepseek-v4-flash"]`, Writable: true, Note: "按会话生效",
			Options: []agent.NativeConfigOptionValue{
				{Value: `["deepseek-official","deepseek-v4-pro"]`, Name: "DeepSeek-V4-Pro", Group: "deepseek-official"},
				{Value: "low", Name: "Low"},
			},
		},
		{ID: "reasoning_effort", Name: "Reasoning effort", Type: "select", CurrentValue: "high", Writable: true},
	}
}

// TestListAgentConfigOptionsRegistryValidation 未知 agent → InvalidArgument +
// 动态名单(与 ListThreadMemories 同一句式)。
func TestListAgentConfigOptionsRegistryValidation(t *testing.T) {
	prov := &fakeNativeOptionsProvider{rows: probeNativeRows()}
	srv := newNativeOptionsServer(t, prov)

	_, err := srv.ListAgentConfigOptions(context.Background(), &pb.ListAgentConfigOptionsRequest{AgentId: "nope"})
	if err == nil || !strings.Contains(err.Error(), "已注册: dsh") {
		t.Fatalf("未知 agent 应回 InvalidArgument 并动态列出注册名单: %v", err)
	}
	if _, err := srv.ListAgentConfigOptions(context.Background(), &pb.ListAgentConfigOptionsRequest{}); err == nil {
		t.Fatal("缺 agent_id 应报错")
	}
}

// TestListAgentConfigOptionsPassthrough 目录行原样进 pb(编码串逐字节);时间戳非零。
func TestListAgentConfigOptionsPassthrough(t *testing.T) {
	prov := &fakeNativeOptionsProvider{rows: probeNativeRows()}
	srv := newNativeOptionsServer(t, prov)

	resp, err := srv.ListAgentConfigOptions(context.Background(), &pb.ListAgentConfigOptionsRequest{AgentId: "dsh"})
	if err != nil {
		t.Fatalf("应成功: %v", err)
	}
	if resp.Error != "" || resp.SampledAtUnixMs == 0 || len(resp.Items) != 2 {
		t.Fatalf("应答形状不符: %+v", resp)
	}
	model := resp.Items[0]
	if model.Id != "model" || !model.Writable || len(model.Options) != 2 {
		t.Fatalf("model 行透传不符: %+v", model)
	}
	if got, want := model.Options[0].Value, `["deepseek-official","deepseek-v4-pro"]`; got != want {
		t.Fatalf("编码串应逐字节: got=%q want=%q", got, want)
	}
	if model.Options[0].Group != "deepseek-official" || model.Options[1].Value != "low" {
		t.Fatalf("叶子字段透传不符: %+v", model.Options)
	}
}

// TestListAgentConfigOptionsErrorNotFatal 读取失败 → resp.Error 带原因,不炸 RPC。
func TestListAgentConfigOptionsErrorNotFatal(t *testing.T) {
	prov := &fakeNativeOptionsProvider{err: errors.New("dsh 正忙")}
	srv := newNativeOptionsServer(t, prov)

	resp, err := srv.ListAgentConfigOptions(context.Background(), &pb.ListAgentConfigOptionsRequest{AgentId: "dsh"})
	if err != nil {
		t.Fatalf("读取失败不应炸 RPC: %v", err)
	}
	if !strings.Contains(resp.Error, "正忙") {
		t.Fatalf("错误原因应如实带回: %q", resp.Error)
	}
}

// TestListAgentConfigOptionsThreadOwnership thread_id 不属于该 agent → InvalidArgument。
func TestListAgentConfigOptionsThreadOwnership(t *testing.T) {
	prov := &fakeNativeOptionsProvider{rows: probeNativeRows()}
	srv := newNativeOptionsServer(t, prov)
	// 造一条 echo 名下的会话(归属不匹配的样本)。
	th := &store.Thread{ID: "th-x", AgentID: "echo", Workspace: srv.cfg.Store.DefaultWorkspace()}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("落库失败: %v", err)
	}

	if _, err := srv.ListAgentConfigOptions(context.Background(), &pb.ListAgentConfigOptionsRequest{AgentId: "dsh", ThreadId: "th-x"}); err == nil {
		t.Fatal("他 agent 的会话应被拒绝")
	}
}

// TestSetAgentNativeConfigOptionRoute 写通道:SetCodexSettings 的原生配置项分支
// 透传 provider(optionID/value 原样),双字段缺一报错。
func TestSetAgentNativeConfigOptionRoute(t *testing.T) {
	prov := &fakeNativeOptionsProvider{rows: probeNativeRows()}
	srv := newNativeOptionsServer(t, prov)

	if _, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		AgentId:              "dsh",
		NativeConfigOptionId: &[]string{"reasoning_effort"}[0],
		NativeConfigValue:    &[]string{"low"}[0],
	}); err != nil {
		t.Fatalf("原生配置项写入应成功: %v", err)
	}
	if prov.setOptionID != "reasoning_effort" || prov.setValue != "low" {
		t.Fatalf("provider 应收到原样取值: %+v", prov)
	}

	// 只给一个字段 → InvalidArgument。
	if _, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		AgentId:              "dsh",
		NativeConfigOptionId: &[]string{"reasoning_effort"}[0],
	}); err == nil {
		t.Fatal("双字段缺一应报错")
	}
	// provider 拒绝(如 pi 的只读边界)→ 错误如实上抛。
	prov.setErr = errors.New("该 Agent 未提供原生设置写通道")
	if _, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		AgentId:              "dsh",
		NativeConfigOptionId: &[]string{"reasoning_effort"}[0],
		NativeConfigValue:    &[]string{"low"}[0],
	}); err == nil {
		t.Fatal("provider 拒绝的写应上抛")
	}
}

// TestHostCapabilityNativeSettings 能力位:实现 NativeConfigOptions 的 provider
// 带 "agent_native_settings:<id>";未实现的不带。
func TestHostCapabilityNativeSettings(t *testing.T) {
	prov := &fakeNativeOptionsProvider{rows: probeNativeRows()}
	srv := newNativeOptionsServer(t, prov)
	caps, err := srv.GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatalf("GetHostInfo 应成功: %v", err)
	}
	found := false
	for _, c := range caps.Capabilities {
		if c == "agent_native_settings:dsh" {
			found = true
		}
	}
	if !found {
		t.Fatalf("应声明 agent_native_settings:dsh: %v", caps.Capabilities)
	}
}

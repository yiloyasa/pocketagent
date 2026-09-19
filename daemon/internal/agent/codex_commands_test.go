// codex_commands_test.go —— CodexProvider.SlashCommands(skills → SlashCommand)映射钉子。
//
// 【钉什么】产品决策"skills 即命令"的映射规则:
//   - 只保留 Enabled 的技能(禁用技能列出来只会点出没反应的命令);
//   - Name/Description 一一映射,InputHint 恒空(手机端据此走自然语言填充);
//   - hooks/plugins 与"加载错误"占位项不混入;
//   - 读取失败且无缓存 → nil(空面板,不是错误);
//   - cwd 作为缓存 key 原样透传(技能按目录算)。
//
// 不依赖真 codex 子进程:capCache.fetch 注入假数据(与 capabilities_cache_test 同款)。
package agent

import (
	"errors"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestSlashCommandsMapsEnabledSkillsOnly(t *testing.T) {
	p := NewCodexProvider()
	var seenCwd string
	p.capCache.fetch = func(cwd string) ([]*pb.CodexCapability, error) {
		seenCwd = cwd
		return []*pb.CodexCapability{
			{Kind: "skill", Name: "commit", Description: "提交代码", Enabled: true, Path: "C:/s/commit"},
			{Kind: "skill", Name: "review", Description: "", Enabled: true},
			{Kind: "skill", Name: "expired", Description: "被禁用的技能", Enabled: false},
			{Kind: "skill", Name: "加载错误", Enabled: false, DisabledReason: "frontmatter 解析失败"},
			{Kind: "skill", Name: "", Description: "无名条目", Enabled: true},
			{Kind: "hook", Name: "sessionStart", Description: "hook 不算命令", Enabled: true},
			{Kind: "plugin", Name: "demo-plugin", Description: "插件不算命令", Enabled: true},
		}, nil
	}

	got := p.SlashCommands("G:/demo/repo")
	if seenCwd != "G:/demo/repo" {
		t.Fatalf("cwd 应原样透传给能力查询(缓存 key),得到 %q", seenCwd)
	}
	if len(got) != 2 {
		t.Fatalf("应只保留 2 个启用技能,得到 %d 条: %+v", len(got), got)
	}
	if got[0].Name != "commit" || got[0].Description != "提交代码" {
		t.Fatalf("第 1 条映射错误: %+v", got[0])
	}
	if got[1].Name != "review" || got[1].Description != "" {
		t.Fatalf("第 2 条映射错误: %+v", got[1])
	}
	for _, c := range got {
		if c.InputHint != "" {
			t.Fatalf("codex 技能无参数模板,InputHint 必须为空: %+v", c)
		}
	}
}

func TestSlashCommandsErrorWithoutCacheReturnsNil(t *testing.T) {
	p := NewCodexProvider()
	p.capCache.fetch = func(string) ([]*pb.CodexCapability, error) {
		return nil, errors.New("codex 不可达")
	}
	if got := p.SlashCommands("/w"); got != nil {
		t.Fatalf("读取失败且无缓存应返回 nil,得到 %+v", got)
	}
}

func TestSlashCommandsStaleCacheStillServes(t *testing.T) {
	// 与 capabilities 缓存的"过期兜底"语义对齐:重拉失败但有旧值时返回旧值,
	// SlashCommands 应吃到这一兜底(转成命令)而不是返回空面板。
	p := NewCodexProvider()
	calls := 0
	p.capCache.fetch = func(string) ([]*pb.CodexCapability, error) {
		calls++
		if calls == 1 {
			return []*pb.CodexCapability{{Kind: "skill", Name: "commit", Description: "提交", Enabled: true}}, nil
		}
		return nil, errors.New("子进程繁忙")
	}
	if got := p.SlashCommands("/w"); len(got) != 1 {
		t.Fatalf("首次应取到技能: %+v", got)
	}
	got := p.SlashCommands("/w")
	if len(got) != 1 || got[0].Name != "commit" {
		t.Fatalf("重拉失败应回退旧缓存,得到 %+v", got)
	}
}

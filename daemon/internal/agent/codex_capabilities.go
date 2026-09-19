// codex_capabilities.go —— 用量/限流、托管策略、能力可见性、MCP 状态。
//
// 【为什么这三件事放在一起】它们都是"只读地照镜子":不改 codex 的行为,
// 只把 codex 那边本来就有的信息如实搬到手机上。而它们回答的恰好是用户
// 最常问、以前却完全答不上来的三个问题:
//
//	还能不能用?     → account/rateLimits/read + account/usage/read
//	为什么这个选项不见了/点了没反应? → configRequirements/read
//	它到底有哪些能力,为什么某个技能不生效? → skills/list + hooks/list + plugin/installed
//
// 以前这三处全是空白:限流只当告警转发、托管策略完全没读、能力列表干脆没有入口。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// CodexRequirements 是从 configRequirements/read 提取出来的"策略约束"。
// 只保留会影响手机端可选项的那几项 —— 其余字段(日志目录、sqlite 路径等)
// 对手机没有意义,搬过来只是噪音。
type CodexRequirements struct {
	Managed                 bool
	AllowedApprovalPolicies []string
	AllowedSandboxModes     []string
	AllowedWebSearchModes   []string
	Note                    string
}

// Requirements 读一次托管策略。
//
// 读不到不是错误:绝大多数用户没有 requirements.toml/MDM 策略,返回零值
// (Managed=false,三个列表都空)表示"无限制",手机端据此正常渲染全部选项。
func (p *CodexProvider) Requirements() CodexRequirements {
	req, _ := p.RequirementsCtx(context.Background())
	return req
}

// RequirementsCtx 是带 ctx 的版本,并把"读取失败"**如实返回**给调用方。
//
// 【为什么要把 err 暴露出来】读不到时 out 是零值(Managed=false、白名单全空 =
// 不限制)。但这不等于"真的没有策略" —— 设置页需要区分:读不到时留一句
// "未能读取电脑端托管策略",别把"读不到"伪装成"没有策略"。
func (p *CodexProvider) RequirementsCtx(ctx context.Context) (CodexRequirements, error) {
	var out CodexRequirements
	err := p.withClientCtx(ctx, clientInitBudget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "configRequirements/read", map[string]interface{}{})
		if err != nil {
			return err
		}
		var res struct {
			Requirements *struct {
				AllowedApprovalPolicies []string `json:"allowedApprovalPolicies"`
				AllowedSandboxModes     []string `json:"allowedSandboxModes"`
				AllowedWebSearchModes   []string `json:"allowedWebSearchModes"`
				AutoReview              *struct {
					Enabled *bool `json:"enabled"`
				} `json:"autoReview"`
			} `json:"requirements"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if res.Requirements == nil {
			return nil // 没有策略 = 无限制
		}
		r := res.Requirements
		out.Managed = true
		out.AllowedApprovalPolicies = r.AllowedApprovalPolicies
		out.AllowedSandboxModes = r.AllowedSandboxModes
		out.AllowedWebSearchModes = r.AllowedWebSearchModes

		var notes []string
		if len(r.AllowedApprovalPolicies) > 0 {
			notes = append(notes, "审批策略仅限 "+strings.Join(r.AllowedApprovalPolicies, " / "))
		}
		if len(r.AllowedSandboxModes) > 0 {
			notes = append(notes, "沙箱模式仅限 "+strings.Join(r.AllowedSandboxModes, " / "))
		}
		if len(r.AllowedWebSearchModes) > 0 {
			notes = append(notes, "联网搜索仅限 "+strings.Join(r.AllowedWebSearchModes, " / "))
		}
		if r.AutoReview != nil && r.AutoReview.Enabled != nil && *r.AutoReview.Enabled {
			notes = append(notes, "自动审查已由策略开启")
		}
		if len(notes) == 0 {
			// managed=true 但没有我们关心的限制:如实说明,避免用户以为哪里坏了。
			notes = append(notes, "这台电脑的 codex 受托管策略约束(当前策略未限制 App 上的可选项)")
		}
		out.Note = "来自 requirements.toml / 管理员策略:" + strings.Join(notes, "；")
		return nil
	})
	if err != nil {
		log.Printf("[codex] 读托管策略失败(按无限制处理): %v", err)
	}
	return out, err
}

// CodexUsage 读用量与限流。
//
// 返回的 CodexUsage 里 error 字段非空表示"读不到",而不是"额度为 0" ——
// 这个区分很重要:把拉取失败显示成 0% 会让用户以为额度用完了。
//
// 【第 28 轮】外面包了一层 TTL 缓存(60 秒):用量面板可能被多次打开/刷新,
// 60s 内零 codex 调用;重拉失败且有旧值时返回旧值(哪怕过期),避免"暂时读不到"。
func (p *CodexProvider) Usage() *pb.CodexUsage {
	out, _ := p.usageCache.read("", "usage")
	return out
}

// fetchUsageUncached 是 Usage 的真实拉取(无缓存)。
func (p *CodexProvider) fetchUsageUncached() *pb.CodexUsage {
	out := &pb.CodexUsage{Primary: &pb.RateLimitWindow{}, Secondary: &pb.RateLimitWindow{}}
	var firstErr error

	// 1) 限流窗口 + 积分。这是"还能不能用"的正面回答。
	err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "account/rateLimits/read", map[string]interface{}{})
		if err != nil {
			return err
		}
		var res struct {
			RateLimits *struct {
				PlanType  string         `json:"planType"`
				Primary   rateWindowJSON `json:"primary"`
				Secondary rateWindowJSON `json:"secondary"`
				Credits   *struct {
					HasCredits bool   `json:"hasCredits"`
					Unlimited  bool   `json:"unlimited"`
					Balance    string `json:"balance"`
				} `json:"credits"`
				SpendControlReached  bool   `json:"spendControlReached"`
				RateLimitReachedType string `json:"rateLimitReachedType"`
			} `json:"rateLimits"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if res.RateLimits == nil {
			return fmt.Errorf("响应里没有 rateLimits")
		}
		rl := res.RateLimits
		out.PlanType = rl.PlanType
		out.Primary = &pb.RateLimitWindow{
			UsedPercent:        int32(rl.Primary.UsedPercent),
			ResetsAtUnixS:      rl.Primary.ResetsAt,
			WindowDurationMins: rl.Primary.WindowDurationMins,
		}
		out.Secondary = &pb.RateLimitWindow{
			UsedPercent:        int32(rl.Secondary.UsedPercent),
			ResetsAtUnixS:      rl.Secondary.ResetsAt,
			WindowDurationMins: rl.Secondary.WindowDurationMins,
		}
		out.SpendControlReached = rl.SpendControlReached
		out.RateLimitReachedType = rl.RateLimitReachedType
		if rl.Credits != nil {
			out.HasCredits = rl.Credits.HasCredits
			out.UnlimitedCredits = rl.Credits.Unlimited
			out.CreditBalance = rl.Credits.Balance
		}
		return nil
	})
	if err != nil {
		firstErr = err
	}

	// 2) 累计用量(锦上添花:失败不影响限流数据)。
	if err := p.withStatusClient(func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "account/usage/read", map[string]interface{}{})
		if err != nil {
			return err
		}
		var res struct {
			Summary *struct {
				LifetimeTokens    int64 `json:"lifetimeTokens"`
				PeakDailyTokens   int64 `json:"peakDailyTokens"`
				CurrentStreakDays int32 `json:"currentStreakDays"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if res.Summary != nil {
			out.LifetimeTokens = res.Summary.LifetimeTokens
			out.PeakDailyTokens = res.Summary.PeakDailyTokens
			out.CurrentStreakDays = res.Summary.CurrentStreakDays
		}
		return nil
	}); err != nil {
		log.Printf("[codex] 读累计用量失败(不影响限流数据): %v", err)
	}

	out.FetchedAtUnixMs = time.Now().UnixMilli()
	if firstErr != nil {
		// 只把"限流读不到"当作 error 报给手机:那才是决定能不能用的数据。
		out.Error = firstErr.Error()
	}
	return out
}

type rateWindowJSON struct {
	UsedPercent        int   `json:"usedPercent"`
	ResetsAt           int64 `json:"resetsAt"`
	WindowDurationMins int64 `json:"windowDurationMins"`
}

// Capabilities 列出这台电脑上 codex 实际具备的技能、hooks 与插件。
//
// 全部**只读**。价值不在"能改",而在回答"为什么某个技能不生效" ——
// 所以 disabled_reason 比 enabled 更值得搬出来。
//
// 【第 28 轮】外面包了一层 TTL 缓存(30 分钟,key=cwd):以前每次进设置页都要
// 连打 skills/list + hooks/list + plugin/installed 三个查询,codex 子进程繁忙时
// 超 10s 预算就整页红字。现在 30 分钟内秒回;重拉预算放宽到 45s;重拉失败且
// 有旧值时返回旧值。cwd 是缓存 key —— 技能列表按目录算,目录变了必须重拉。
func (p *CodexProvider) CapabilitiesDetailed(cwd string) ([]*pb.CodexCapability, error) {
	return p.capCache.read(cwd, "capabilities")
}

// fetchCapabilitiesDetailed 是 CapabilitiesDetailed 的真实拉取(无缓存),
// 三个子查询各自受 budget 约束。
func (p *CodexProvider) fetchCapabilitiesDetailed(cwd string, budget time.Duration) ([]*pb.CodexCapability, error) {
	out := []*pb.CodexCapability{}
	var errs []string
	if cwd == "" {
		cwd = "."
	}

	// ---- 技能 ----
	if err := p.withStatusClientBudget(budget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "skills/list", map[string]interface{}{"cwds": []string{cwd}})
		if err != nil {
			return err
		}
		var res struct {
			Data []struct {
				Cwd    string            `json:"cwd"`
				Errors []json.RawMessage `json:"errors"`
				Skills []struct {
					Name             string  `json:"name"`
					Description      string  `json:"description"`
					ShortDescription *string `json:"shortDescription"`
					Enabled          bool    `json:"enabled"`
					Path             string  `json:"path"`
				} `json:"skills"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		for _, entry := range res.Data {
			for _, s := range entry.Skills {
				desc := s.Description
				if desc == "" && s.ShortDescription != nil {
					desc = *s.ShortDescription
				}
				item := &pb.CodexCapability{
					Kind: "skill", Name: s.Name, Description: desc,
					Enabled: s.Enabled, Path: s.Path, Source: entry.Cwd,
				}
				if !s.Enabled {
					item.DisabledReason = "该技能当前被禁用"
				}
				out = append(out, item)
			}
			// 加载过程中的错误也要暴露:技能"看不见"往往就是这里出的问题。
			for _, e := range entry.Errors {
				out = append(out, &pb.CodexCapability{
					Kind: "skill", Name: "加载错误", Enabled: false,
					DisabledReason: clip(string(e), 300), Source: entry.Cwd,
				})
			}
		}
		return nil
	}); err != nil {
		errs = append(errs, "skills: "+err.Error())
	}

	// ---- hooks ----
	if err := p.withStatusClientBudget(budget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "hooks/list", map[string]interface{}{"cwds": []string{cwd}})
		if err != nil {
			return err
		}
		var res struct {
			Data []struct {
				Cwd   string `json:"cwd"`
				Hooks []struct {
					EventName     string  `json:"eventName"`
					Status        string  `json:"status"`
					StatusMessage *string `json:"statusMessage"`
					SourcePath    string  `json:"sourcePath"`
					HandlerType   string  `json:"handlerType"`
				} `json:"hooks"`
				Warnings []json.RawMessage `json:"warnings"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		for _, entry := range res.Data {
			for _, h := range entry.Hooks {
				out = append(out, &pb.CodexCapability{
					Kind: "hook", Name: h.EventName,
					Description: derefTrim(h.StatusMessage),
					Enabled:     h.Status != "failed",
					Path:        h.SourcePath, Source: entry.Cwd,
					Status: h.Status,
				})
			}
			for _, w := range entry.Warnings {
				out = append(out, &pb.CodexCapability{
					Kind: "hook", Name: "hooks 警告", Enabled: true,
					Status: clip(string(w), 300), Source: entry.Cwd,
				})
			}
		}
		return nil
	}); err != nil {
		errs = append(errs, "hooks: "+err.Error())
	}

	// ---- 已安装插件 ----
	if err := p.withStatusClientBudget(budget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "plugin/installed", map[string]interface{}{"cwds": []string{cwd}})
		if err != nil {
			return err
		}
		var res struct {
			Marketplaces []struct {
				Name    string `json:"name"`
				Plugins []struct {
					Id             string  `json:"id"`
					Name           string  `json:"name"`
					Enabled        bool    `json:"enabled"`
					Installed      bool    `json:"installed"`
					Version        string  `json:"version"`
					LocalVersion   string  `json:"localVersion"`
					DisabledReason *string `json:"disabledReason"`
					Availability   string  `json:"availability"`
				} `json:"plugins"`
			} `json:"marketplaces"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		for _, mkt := range res.Marketplaces {
			for _, pl := range mkt.Plugins {
				version := pl.Version
				if version == "" {
					version = pl.LocalVersion
				}
				out = append(out, &pb.CodexCapability{
					Kind: "plugin", Name: firstNonEmpty(pl.Name, pl.Id),
					Enabled: pl.Enabled, Version: version,
					Source: mkt.Name, Status: pl.Availability,
					DisabledReason: derefTrim(pl.DisabledReason),
				})
			}
		}
		return nil
	}); err != nil {
		errs = append(errs, "plugins: "+err.Error())
	}

	// 稳定排序:同名能力在不同机器上顺序一致,便于肉眼对比与截图核对。
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	if len(errs) > 0 {
		return out, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return out, nil
}

// Capabilities 保留旧的只返回列表的调用形状,兼容已有诊断工具与测试。
// 新的 RPC 路径使用 CapabilitiesDetailed,不会再吞掉读取错误。
func (p *CodexProvider) Capabilities(cwd string) []*pb.CodexCapability {
	out, _ := p.CapabilitiesDetailed(cwd)
	return out
}

// McpStatusDetailed 是 McpStatus 的加强版:把以前丢掉的 authStatus/serverInfo 一起带出来。
//
// 【为什么必须补】只转发 name/tools/status 的话,"某个 MCP 用不了"这件事在手机上
// 完全看不出来 —— 用户只会觉得"这个工具怎么不好使"。authStatus 直接回答"是不是授权问题"。
//
// 【第 28 轮】外面包了一层 TTL 缓存(5 分钟):mcpServerStatus/list 是重查询,
// codex 繁忙时常超 10s;缓存 miss 的重拉放宽到 45s,失败且有旧值时返回旧值。
func (p *CodexProvider) McpStatusDetailed() ([]*pb.McpServerStatus, error) {
	return p.mcpCache.read("", "mcpServers")
}

// fetchMcpStatusDetailed 是 McpStatusDetailed 的真实拉取(无缓存)。
func (p *CodexProvider) fetchMcpStatusDetailed(budget time.Duration) ([]*pb.McpServerStatus, error) {
	out := []*pb.McpServerStatus{}
	err := p.withStatusClientBudget(budget, func(ctx context.Context, c *codexapp.Client) error {
		raw, err := c.CallCtx(ctx, "mcpServerStatus/list", map[string]interface{}{})
		if err != nil {
			return err
		}
		// 【实测纠正】两处与直觉不同,两处都踩过:
		//   1. result.data 是**数组**(JSON Schema 里的 "items" 是数组元素的声明,
		//      不是字段名);
		//   2. tools 是**对象**(工具名 → 工具定义),不是数组 —— 按数组解析会得到
		//      "cannot unmarshal object into Go struct field ... of type []json.RawMessage",
		//      结果是整个 MCP 面板空掉,而且只在真机上才看得出来。
		var res struct {
			Data []struct {
				Name       string  `json:"name"`
				PluginId   string  `json:"pluginId"`
				AuthStatus string  `json:"authStatus"`
				Error      *string `json:"error"`
				ServerInfo *struct {
					Name    string  `json:"name"`
					Title   *string `json:"title"`
					Version string  `json:"version"`
				} `json:"serverInfo"`
				Tools             map[string]json.RawMessage `json:"tools"`
				Resources         []json.RawMessage          `json:"resources"`
				ResourceTemplates []json.RawMessage          `json:"resourceTemplates"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return fmt.Errorf("mcpServerStatus/list 应答无法解析: %w", err)
		}
		for _, s := range res.Data {
			item := &pb.McpServerStatus{
				Name:                  s.Name,
				Tools:                 int32(len(s.Tools)),
				AuthStatus:            s.AuthStatus,
				ResourceCount:         int32(len(s.Resources)),
				ResourceTemplateCount: int32(len(s.ResourceTemplates)),
				Error:                 derefTrim(s.Error),
			}
			// codex 不直接给 "ready/failed" 这种状态串:状态信息分散在 authStatus
			// 与 error 里。这里合成一个手机端能直接判断的结论 ——
			// 否则前端得同时理解两个字段还要自己推,很容易漏掉一种组合。
			if item.Error != "" {
				item.Status = "failed"
			} else {
				item.Status = "ready"
			}
			if s.ServerInfo != nil {
				item.ServerTitle = firstNonEmpty(derefTrim(s.ServerInfo.Title), s.ServerInfo.Name)
				item.ServerVersion = s.ServerInfo.Version
			}
			if item.ServerTitle == "" {
				// serverInfo 为 null 时(实测 codex_app 就是这样)退回插件 id,
				// 否则手机上会显示一条没有名字的记录。
				item.ServerTitle = s.PluginId
			}
			out = append(out, item)
		}
		return nil
	})
	return out, err
}

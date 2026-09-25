// configpersist.go —— dshprobe 的 config_option 持久性探针(-run configpersist)。
//
// 【目的】原生设置透出轮(2026-09-25)的前置实证:session/set_config_option 改
// reasoning_effort 之后,这个值
//   a) 本会话内是否生效(configOptions 的 currentValue 是否变化);
//   b) 是否被 dsh 持久化 —— 新开的 session/new 是否带着改后的值(即:手机设置页
//      的切换要不要绑定具体会话,还是可以像桌面端一样"改默认")。
//
// 结论直接决定 DshProvider.SetNativeConfigOption 的写语义,以及手机端「原生设置」
// 分组的副文案(持久 = 写默认;不持久 = 明确说按会话生效)。全程不跑 LLM 轮次
// (零费用),DSH_HOME 用临时目录,不碰用户 ~/.dsh(红线)。
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// configOptionCurrent 从一份 session/new|set_config_option 相关报文里抽某个
// option 的 currentValue(找不到返回 "",ok=false,不猜)。
func configOptionCurrent(raw json.RawMessage, optionID string) (string, bool) {
	var p struct {
		ConfigOptions []struct {
			ID           string `json:"id"`
			CurrentValue string `json:"currentValue"`
		} `json:"configOptions"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return "", false
	}
	for _, o := range p.ConfigOptions {
		if o.ID == optionID {
			return o.CurrentValue, true
		}
	}
	return "", false
}

// doConfigPersist 实证 set_config_option 的会话内生效与跨会话持久性。
func doConfigPersist(c *client, workDir string) item {
	const optionID = "reasoning_effort"
	const target = "low" // rc.3 实测默认 high;选 low 拉开差异,易辨认
	it := item{
		name: "set_config_option 持久性(会话内生效 + 跨会话默认)",
		cmd: fmt.Sprintf(`session/set_config_option {"sessionId":…,"optionId":%q,"value":%q}`+
			` → session/close → 新 session/new 读 configOptions`, optionID, target),
	}

	// ① 基线会话:读 reasoning_effort 基线值
	res, _, _, err := c.call("session/new", map[string]any{"cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	if err != nil {
		it.status, it.note = "失败", "基线 session/new 失败: "+err.Error()
		return it
	}
	var base struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(res, &base)
	if base.SessionID == "" {
		it.status, it.note = "失败", "基线 session/new 无 sessionId"
		return it
	}
	baseVal, ok := configOptionCurrent(res, optionID)
	it.frames = append(it.frames, trunc(pretty(res), 3000))

	// ② 会话内改档 + 读回。参数名双探:rc.3 首测 optionId 被拒(zod 报 configId
	// expected string),故先试 configId,失败再回落 optionId —— 两种参数名哪边
	// 能过,证据里都留原帧,给 daemon 侧参数名适配钉死地面真值。
	setRaw, _, errRaw, err := c.call("session/set_config_option", map[string]any{
		"sessionId": base.SessionID, "configId": optionID, "value": target,
	}, 30*time.Second)
	paramName := "configId"
	if err != nil {
		it.frames = append(it.frames, "configId 尝试被拒: "+trunc(string(errRaw), 800))
		setRaw, _, errRaw, err = c.call("session/set_config_option", map[string]any{
			"sessionId": base.SessionID, "optionId": optionID, "value": target,
		}, 30*time.Second)
		paramName = "optionId"
	}
	if err != nil {
		it.status = "失败"
		it.note = fmt.Sprintf("set_config_option 失败(configId/optionId 均被拒): %v", err)
		it.frames = append(it.frames, trunc(string(errRaw), 1200))
		_, _, _, _ = c.call("session/close", map[string]any{"sessionId": base.SessionID}, 15*time.Second)
		return it
	}
	it.frames = append(it.frames, trunc(pretty(setRaw), 1500))
	afterVal, _ := configOptionCurrent(setRaw, optionID)

	// ③ 关会话 → 新会话:看新会话的 currentValue 是否继承改后的值
	_, _, _, _ = c.call("session/close", map[string]any{"sessionId": base.SessionID}, 15*time.Second)
	res2, _, _, err := c.call("session/new", map[string]any{"cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	if err != nil {
		it.status, it.note = "失败", "复核 session/new 失败: "+err.Error()
		return it
	}
	it.frames = append(it.frames, trunc(pretty(res2), 3000))
	reVal, ok2 := configOptionCurrent(res2, optionID)

	if !ok || !ok2 {
		it.status, it.note = "失败", fmt.Sprintf("configOptions 缺 %s 项(基线读=%v 复核读=%v)", optionID, ok, ok2)
		return it
	}
	inSession := afterVal == target
	persist := reVal == target
	it.status = "通过"
	it.note = fmt.Sprintf("参数名=%s;基线=%s → set %s → 会话内 currentValue=%s → close 后新会话 currentValue=%s",
		paramName, baseVal, target, afterVal, reVal)
	it.conclusion = fmt.Sprintf("可用的参数名=%s;会话内生效=%v;跨会话持久=%v。", paramName, inSession, persist)
	if persist {
		it.impact = "set_config_option 由 dsh 持久化为默认:手机设置页切换(无线程上下文)用瞬态会话写默认即可,语义与桌面端一致。"
	} else {
		it.impact = "set_config_option 仅会话内生效:设置页切换必须绑定具体会话(thread_id),无线程时如实说明按会话生效。"
	}
	return it
}

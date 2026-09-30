// dshproto020.go —— dshprobe 的 0.2.0-rc.2 协议专项探针(-run proto020)。
//
// 【目的】dsh 0.1.5-rc.3 → 0.2.0-rc.2 升级重探。bundle 静态提取(@deepseek-ai/dsh-acp
// 0.2.0-rc.2 + @agentclientprotocol/sdk 1.4.0)指认了若干"无 DEEPSEEK_API_KEY 也能
// 实证"的新形状,本探针逐项取 live 地面真值:
//
//	a) initialize 的 promptCapabilities.image 是否翻真(0.2.0 起按模型 inputModalities
//	   动态判定,bundle:supportsAcpImagePrompts);
//	b) session/new 应答 configOptions 形状(reasoning_effort 是否出现 "" "Provider
//	   default" 档;bundle:PROVIDER_DEFAULT_REASONING_VALUE);
//	c) 图片块准入:prompt 带 {type:"image"} —— 未翻真时应报 invalid-params
//	   "inline image prompts were not advertised";翻真时准入成功、no-key 在 LLM 层
//	   才报错(两种错误形状都能定性 image 门的行为);
//	d) resource_link 块准入(0.2.0 新接受为 prompt 合法块);
//	e) session/list 的 cwd 过滤与 cursor 分页参数(0.2.0 新增);
//	f) session/set_config_option 空串档位(configId=reasoning_effort value="")与
//	   未知 configId 的错误形状;
//	g) resume 形态矩阵(0.2.0 失败语义细化,全部 -32602 但 message 可区分):
//	   会话 active 中 resume / close 后 resume / 缺 cwd / cwd 不匹配 / 幽灵 id;
//	h) 并发 prompt 的 "a prompt is already in flight" 错误形状(best-effort,时序敏感)。
//
// 全程 DSH_HOME 隔离临时目录,不碰用户 ~/.dsh(红线);LLM 轮次零跑(无 key,
// 各 prompt 预期在准入或 LLM 凭证层终止,错误形状即素材)。
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// probe1x1PNG 是 1x1 透明 PNG 的 canonical base64(探针专用固定素材)。
const probe1x1PNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// imageAdmissionVerdict 从一次"带图 prompt"的错误形状定性 image 门行为。
func imageAdmissionVerdict(errRaw json.RawMessage, err error) string {
	if err == nil {
		return "prompt 竟然成功(本机无 key 不应有此结果,需人工核对)"
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(errRaw, &e)
	l := strings.ToLower(e.Message)
	switch {
	case strings.Contains(l, "not advertised"):
		return fmt.Sprintf("image=false(未翻真):code=%d message=%q —— 未广播就收图,准入层拒绝", e.Code, clip(e.Message, 200))
	case strings.Contains(l, "no api key"), strings.Contains(l, "api key"), strings.Contains(l, "credential"):
		return fmt.Sprintf("image=true(已翻真):图片块通过准入(base64/mime 校验+落盘),失败在 LLM 凭证层 code=%d message=%q", e.Code, clip(e.Message, 200))
	case strings.Contains(l, "mime"), strings.Contains(l, "base64"):
		return fmt.Sprintf("图片块被格式校验拒绝(说明门是开的,素材不合规):code=%d message=%q", e.Code, clip(e.Message, 200))
	default:
		return fmt.Sprintf("其他错误形状 code=%d message=%q(需人工核对)", e.Code, clip(e.Message, 200))
	}
}

// configOptionsBrief 摘一份 configOptions 的一行描述(id/currentValue/候选叶子值)。
func configOptionsBrief(raw json.RawMessage) string {
	var p struct {
		ConfigOptions []struct {
			ID           string `json:"id"`
			CurrentValue string `json:"currentValue"`
			Options      []struct {
				Group   string `json:"group"`
				Name    string `json:"name"`
				Options []struct {
					Value string `json:"value"`
					Name  string `json:"name"`
				} `json:"options"`
				Value string `json:"value"`
			} `json:"options"`
		} `json:"configOptions"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return "(解析失败)"
	}
	out := ""
	for _, o := range p.ConfigOptions {
		var leaves []string
		for _, g := range o.Options {
			if g.Value != "" {
				leaves = append(leaves, g.Value)
				continue
			}
			for _, leaf := range g.Options {
				leaves = append(leaves, leaf.Value)
			}
		}
		out += fmt.Sprintf("{id:%s currentValue:%q 叶子值=%v} ", o.ID, o.CurrentValue, leaves)
	}
	if out == "" {
		return "(configOptions 空)"
	}
	return out
}

// doProto020 跑 0.2.0-rc.2 协议专项(a~h 顺序,单项失败不中断后续项)。
func doProto020(c *client, workDir string) item {
	it := item{
		name: "0.2.0-rc.2 协议专项(image 准入/configOptions 新档/list 参数/resume 失败语义细化)",
		cmd:  `initialize → session/new → prompt(image 块/resource_link 块)→ session/list{cwd,cursor} → set_config_option("" / 未知 id)→ resume 形态矩阵`,
	}
	addFrame := func(label string, raw json.RawMessage) {
		it.frames = append(it.frames, "── "+label+" ──", trunc(pretty(raw), 4000))
	}

	// a) initialize:能力快照已在握手里,这里只摘 image 位与版本。
	var initP struct {
		AgentCapabilities struct {
			PromptCapabilities struct {
				Image bool `json:"image"`
			} `json:"promptCapabilities"`
		} `json:"agentCapabilities"`
		AgentInfo struct {
			Version string `json:"version"`
		} `json:"agentInfo"`
	}
	_ = json.Unmarshal(c.initRes, &initP)
	it.frames = append(it.frames, fmt.Sprintf("[initialize] image=%t agentVersion=%s 冷启动=%s",
		initP.AgentCapabilities.PromptCapabilities.Image, initP.AgentInfo.Version, c.initDur.Round(time.Millisecond)))

	// b) session/new:configOptions 形状(找 reasoning_effort 的 "" Provider default 档)。
	snRes, _, snErrRaw, snErr := c.call("session/new", map[string]any{"cwd": workDir, "mcpServers": []any{}}, 60*time.Second)
	if snErr != nil {
		it.status, it.note = "失败", "session/new 失败: "+snErr.Error()+" "+clip(string(snErrRaw), 300)
		return it
	}
	var sn struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(snRes, &sn)
	addFrame("session/new 应答", snRes)
	brief := configOptionsBrief(snRes)
	it.frames = append(it.frames, "[configOptions 摘要] "+brief)
	if sn.SessionID == "" {
		it.status, it.note = "失败", "session/new 无 sessionId"
		return it
	}

	// c) 图片块准入(不依赖 key:未翻真在准入层拒绝;翻真则死在 LLM 凭证层)。
	_, _, imgErrRaw, imgErr := c.call("session/prompt", map[string]any{
		"sessionId": sn.SessionID,
		"prompt": []any{
			map[string]any{"type": "image", "data": probe1x1PNG, "mimeType": "image/png"},
			map[string]any{"type": "text", "text": "这张图里是什么颜色?(探针素材:1x1 透明 PNG)"},
		},
	}, 60*time.Second)
	imgVerdict := imageAdmissionVerdict(imgErrRaw, imgErr)
	if len(imgErrRaw) > 0 {
		it.frames = append(it.frames, "── prompt(image 块) error ──", trunc(string(imgErrRaw), 1500))
	}
	it.frames = append(it.frames, "[image 准入判定] "+imgVerdict)

	// d) resource_link 块准入(0.2.0 新接受;接受 → 死在凭证层,拒绝 → invalid-params)。
	_, _, rlErrRaw, rlErr := c.call("session/prompt", map[string]any{
		"sessionId": sn.SessionID,
		"prompt": []any{
			map[string]any{"type": "resource_link", "name": "probe-link", "uri": "file:///probe.txt"},
			map[string]any{"type": "text", "text": "引用上面的链接(探针)"},
		},
	}, 60*time.Second)
	rlVerdict := "(resource_link prompt 成功?无 key 不应有此结果,需人工核对)"
	if rlErr != nil {
		var e struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(rlErrRaw, &e)
		if strings.Contains(strings.ToLower(e.Message), "no api key") || keyRelated(string(rlErrRaw)) {
			rlVerdict = fmt.Sprintf("resource_link 被接受(失败在凭证层):code=%d message=%q", e.Code, clip(e.Message, 200))
		} else {
			rlVerdict = fmt.Sprintf("resource_link 形状:code=%d message=%q(被准入拒绝或凭证错误,以 message 定性)", e.Code, clip(e.Message, 200))
		}
		it.frames = append(it.frames, "── prompt(resource_link 块) error ──", trunc(string(rlErrRaw), 1500))
	}
	it.frames = append(it.frames, "[resource_link 判定] "+rlVerdict)

	// e) session/list:cwd 过滤 + 坏 cursor。
	listRes, _, listErrRaw, listErr := c.call("session/list", map[string]any{"cwd": workDir}, 30*time.Second)
	if listErr != nil {
		it.frames = append(it.frames, "── session/list{cwd} error ──", trunc(string(listErrRaw), 1200))
	} else {
		addFrame("session/list{cwd:workDir} 应答", listRes)
	}
	_, _, curErrRaw, curErr := c.call("session/list", map[string]any{"cursor": "dshprobe-bad-cursor"}, 30*time.Second)
	cursorVerdict := "cursor 参数被接受(需人工核对)"
	if curErr != nil {
		cursorVerdict = fmt.Sprintf("坏 cursor 被拒:%s", clip(string(curErrRaw), 300))
	}
	it.frames = append(it.frames, "[list cursor 判定] "+cursorVerdict)

	// f) set_config_option:空串档位(Provider default)与未知 configId。
	setRes, _, setErrRaw, setErr := c.call("session/set_config_option", map[string]any{
		"sessionId": sn.SessionID, "configId": "reasoning_effort", "value": "",
	}, 30*time.Second)
	emptyEffort := "(reasoning_effort 空串档位:prompt 成功)"
	if setErr != nil {
		emptyEffort = fmt.Sprintf("空串档位被拒:%s", clip(string(setErrRaw), 300))
	} else {
		it.frames = append(it.frames, "[set 后 configOptions] "+configOptionsBrief(setRes))
	}
	it.frames = append(it.frames, "[set_config_option 空串] "+emptyEffort)
	_, _, unkErrRaw, unkErr := c.call("session/set_config_option", map[string]any{
		"sessionId": sn.SessionID, "configId": "dshprobe-unknown", "value": "x",
	}, 30*time.Second)
	unknownID := "(未知 configId 竟被接受,需人工核对)"
	if unkErr != nil {
		unknownID = fmt.Sprintf("未知 configId 被拒:%s", clip(string(unkErrRaw), 300))
	}
	it.frames = append(it.frames, "[set_config_option 未知 id] "+unknownID)

	// g) resume 形态矩阵。0.2.0 的失败语义按 bundle 细化为:active 中 → "already
	//    active";缺 cwd → "cwd must be an absolute path";持久头缺席/subagent →
	//    "not resumable";cwd 不匹配 → "cwd does not match"。全部 -32602,message
	//    才是可操作判别 —— daemon 降级条件若要细化,以此为准。
	_, _, actErrRaw, actErr := c.call("session/resume", map[string]any{"sessionId": sn.SessionID, "cwd": workDir}, 30*time.Second)
	activeVerdict := "(active 中 resume 竟成功,需人工核对)"
	if actErr != nil {
		activeVerdict = "active 中 resume 被拒:" + clip(string(actErrRaw), 300)
	}
	it.frames = append(it.frames, "[resume·active 中] "+activeVerdict)

	// close 后:持久化是否幸存直接决定 daemon DeleteDshSession 之后的语义。
	_, _, _, _ = c.call("session/close", map[string]any{"sessionId": sn.SessionID}, 30*time.Second)
	_, _, ncErrRaw, ncErr := c.call("session/resume", map[string]any{"sessionId": sn.SessionID}, 30*time.Second)
	noCwdVerdict := "(缺 cwd resume 竟成功,需人工核对)"
	if ncErr != nil {
		noCwdVerdict = "缺 cwd resume 被拒:" + clip(string(ncErrRaw), 300)
	}
	it.frames = append(it.frames, "[resume·缺 cwd(close 后)] "+noCwdVerdict)

	resRes, _, withCwdErrRaw, withCwdErr := c.call("session/resume", map[string]any{"sessionId": sn.SessionID, "cwd": workDir}, 30*time.Second)
	withCwdVerdict := "(close 后带 cwd resume 成功 → 持久化幸存)"
	if withCwdErr != nil {
		withCwdVerdict = "close 后带 cwd resume 被拒:" + clip(string(withCwdErrRaw), 300)
	} else {
		addFrame("resume 应答(close 后,带 cwd)", resRes)
	}
	it.frames = append(it.frames, "[resume·close 后带 cwd] "+withCwdVerdict)

	// cwd 不匹配(仅当持久化幸存才有意义;否则记录跳过原因)。
	var mismatchVerdict string
	if withCwdErr == nil {
		other := workDir + "-probe-other"
		_, _, mmErrRaw, mmErr := c.call("session/resume", map[string]any{"sessionId": sn.SessionID, "cwd": other}, 30*time.Second)
		if mmErr != nil {
			mismatchVerdict = "cwd 不匹配 resume 被拒:" + clip(string(mmErrRaw), 300)
		} else {
			mismatchVerdict = "(cwd 不匹配 resume 竟成功,需人工核对)"
		}
	} else {
		mismatchVerdict = "(跳过:close 后持久化未幸存,无法构造)"
	}
	it.frames = append(it.frames, "[resume·cwd 不匹配] "+mismatchVerdict)

	// 幽灵 id(带 cwd):数据错位的真实形状。
	_, _, ghErrRaw, ghErr := c.call("session/resume", map[string]any{
		"sessionId": "00000000-0000-0000-0000-000000000000", "cwd": workDir,
	}, 30*time.Second)
	ghostVerdict := "(幽灵 id resume 竟成功,需人工核对)"
	if ghErr != nil {
		ghostVerdict = "幽灵 id resume 被拒:" + clip(string(ghErrRaw), 300)
	}
	it.frames = append(it.frames, "[resume·幽灵 id] "+ghostVerdict)

	// h) 并发 prompt(时序敏感,best-effort):无 key 下首个 prompt 在凭证层失败前
	//    有一个 in-flight 窗口,立刻发第二个应撞 "a prompt is already in flight"。
	concVerdict := "(未观测到并发拒接:首个 prompt 失败快于第二次发送,时序未捕获)"
	doneCh := make(chan string, 1)
	go func() {
		_, _, e1raw, e1 := c.call("session/prompt", map[string]any{
			"sessionId": sn.SessionID,
			"prompt":    []any{map[string]any{"type": "text", "text": "第一个探针 prompt(预期凭证失败)"}},
		}, 60*time.Second)
		if e1 != nil {
			doneCh <- "第一发失败:" + clip(string(e1raw), 200)
		} else {
			doneCh <- "第一发成功(无 key 不应有此结果)"
		}
	}()
	time.Sleep(300 * time.Millisecond)
	_, _, c2raw, c2err := c.call("session/prompt", map[string]any{
		"sessionId": sn.SessionID,
		"prompt":    []any{map[string]any{"type": "text", "text": "第二个探针 prompt(预期并发拒接)"}},
	}, 60*time.Second)
	if c2err != nil {
		concVerdict = "并发第二发被拒:" + clip(string(c2raw), 300)
	} else {
		concVerdict = "(并发第二发竟成功,需人工核对)"
	}
	firstVerdict := <-doneCh
	it.frames = append(it.frames, "[并发 prompt] 第一发:"+firstVerdict+" | 第二发:"+concVerdict)

	it.status = "通过"
	it.note = fmt.Sprintf("image=%t;configOptions=%s", initP.AgentCapabilities.PromptCapabilities.Image, clip(brief, 260))
	it.conclusion = fmt.Sprintf(
		"image 准入:%s;resource_link:%s;list 参数:cwd 过滤与 cursor 见原始帧(%s);空串 effort:%s;未知 configId:%s;"+
			"resume 矩阵:active 中=%s;缺 cwd=%s;close 后带 cwd=%s;cwd 不匹配=%s;幽灵 id=%s;并发:%s。",
		imgVerdict, rlVerdict, cursorVerdict, emptyEffort, unknownID,
		clip(activeVerdict, 120), clip(noCwdVerdict, 120), clip(withCwdVerdict, 120), clip(mismatchVerdict, 120), clip(ghostVerdict, 120), clip(concVerdict, 120))
	it.impact = "daemon 适配的地面真值:image 块构造是否放行、resume 降级条件是否需按 message 细化、" +
		"DeleteDshSession(close)后的持久化语义、config_option_update/空串档位对 Models() 的影响。"
	return it
}

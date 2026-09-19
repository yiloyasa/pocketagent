// image.go —— zcprobe -image 子命令:实测 session/send 的 attachments 图片形状。
//
// 【目的】审计 M 项"打通 zcode 图片输入"的形状实证先行一步:
//   - zcode.cjs 内嵌 zod schema 已核(TKe):session/send 参数
//     {sessionId*, content*, attachments?: record(string,unknown)[]}(宽松 record,
//     schema 层不约束元素形状);服务器侧归一化函数(反混淆 MEs/OEs)按
//     {kind:"image", filename, mimeType, sizeBytes?, localPath? | dataBase64?}
//     消费,归一成 {path|content, type:"image", filename, mimeType}。
//   - 本探针生成一张真实的纯红 PNG,按两种形状发给 supportsImage=true 的模型:
//     A. localPath 形状(本机绝对路径 —— 生产代码拟采用,避免 base64 撑爆 stdio 行);
//     B. dataBase64 内联形状(兜底)。
//     提问"这张图片是什么颜色",以模型回答是否认出"红色"作为"zcode 真看到图"的判据。
//   - 同时打印快照 settings.model.available 的 inputFormat.supportsImage 全表
//     (能力门控的数据源地面真值)与 settings.model.current(当前模型定位)。
//
// 【多候选 + 限流自保】第一轮实证(2026-09-19)踩坑:快照 current=MiniMax-M3
// 声明 supportsImage=true 但上游配额耗尽(错误 2056 "已达到 Token Plan 用量上限",
// 服务器自动重试 maxAttempts=11 拖满 240s)。所以本探针按候选表轮换:每个候选
// 120s 预算,期间一旦观测到 rate_limited/配额类重试(含 1302)≥2 次就 session/stop
// 换下一个,绝不硬扛;A 轮成功的候选才继续 B 轮。
//
// 【用法】go run ./cmd/zcprobe -image [-ws 工作区]
// 【红线】权限模式显式 build(绝不 yolo);不写用户配置,只开临时工作区。
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// redTestPNG 生成一张 96x96 纯红 PNG(zcode/模型必须答"红"才算看到图)。
func redTestPNG() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 96, 96))
	red := color.RGBA{R: 0xe0, G: 0x10, B: 0x10, A: 0xff}
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			img.Set(x, y, red)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zcImgCandidate 一个 supportsImage=true 的候选模型。
type zcImgCandidate struct {
	ProviderID   string
	ModelID      string
	DefaultLevel string
}

// runImage 图片附件实测主流程:A(localPath)/ B(dataBase64)两形状。
func runImage(p *probe, ws string, res *results) {
	fmt.Println("\n== image 实证:session/send attachments 两种形状")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// ---- 建会话(mode 显式 build,红线)----
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("image建会话", err)
		return
	}
	var cr struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
		Settings struct {
			Model struct {
				Current *struct {
					ProviderID string `json:"providerId"`
					ModelID    string `json:"modelId"`
				} `json:"current"`
				Available []struct {
					Ref struct {
						ProviderID string `json:"providerId"`
						ModelID    string `json:"modelId"`
					} `json:"ref"`
					Label     string `json:"label"`
					Reasoning struct {
						DefaultLevel string `json:"defaultLevel"`
					} `json:"reasoning"`
					Properties struct {
						InputFormat struct {
							SupportsText  bool `json:"supportsText"`
							SupportsImage bool `json:"supportsImage"`
						} `json:"inputFormat"`
					} `json:"properties"`
				} `json:"available"`
			} `json:"model"`
		} `json:"settings"`
	}
	if json.Unmarshal(craw, &cr) != nil {
		res.bad("image快照", fmt.Errorf("create 快照无法解析: %.300s", string(craw)))
		return
	}
	sid := cr.Session.SessionID
	if sid == "" {
		res.bad("image建会话", fmt.Errorf("无 sessionId"))
		return
	}

	// ---- 打印能力表(门控数据源的地面真值)----
	fmt.Println("== settings.model(门控地面真值):")
	curID := ""
	if cr.Settings.Model.Current != nil {
		curID = cr.Settings.Model.Current.ModelID
		fmt.Printf("   settings.model.current = %s/%s\n", cr.Settings.Model.Current.ProviderID, curID)
	} else {
		fmt.Println("   settings.model.current = (缺省)")
	}
	imgCandidates := make([]zcImgCandidate, 0)
	imgNames := ""
	for _, m := range cr.Settings.Model.Available {
		flag := "false"
		if m.Properties.InputFormat.SupportsImage {
			flag = "TRUE "
			imgCandidates = append(imgCandidates, zcImgCandidate{
				ProviderID:   m.Ref.ProviderID,
				ModelID:      m.Ref.ModelID,
				DefaultLevel: m.Reasoning.DefaultLevel,
			})
			imgNames += m.Ref.ModelID + " "
		}
		fmt.Printf("   %-30s image=%s text=%-5s (provider=%s defaultLevel=%s)\n",
			m.Ref.ModelID, flag, boolStr(m.Properties.InputFormat.SupportsText), m.Ref.ProviderID, m.Reasoning.DefaultLevel)
	}
	if len(imgCandidates) == 0 {
		res.bad("image候选", fmt.Errorf("没有任何模型声明 supportsImage=true"))
		return
	}
	res.CreateSend = "supportsImage=true: " + imgNames + "; current=" + curID

	// 候选排序:current 若在表里排最前(切模型成本最低),其余按快照顺序。
	cands := make([]zcImgCandidate, 0, len(imgCandidates))
	for _, c := range imgCandidates {
		if c.ModelID == curID {
			cands = append(cands, c)
			break
		}
	}
	for _, c := range imgCandidates {
		if c.ModelID != curID {
			cands = append(cands, c)
		}
	}

	// ---- 订阅 ----
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("image订阅", err)
		return
	}

	// ---- 生成真图并落盘 ----
	pngBytes, err := redTestPNG()
	if err != nil {
		res.bad("image生成", err)
		return
	}
	imgPath := filepath.Join(ws, "pa-red.png")
	if err := os.WriteFile(imgPath, pngBytes, 0o644); err != nil {
		res.bad("image落盘", err)
		return
	}
	question := "附件里这张图片是什么颜色?只回答颜色名,两个汉字。"

	// ---- A 轮:localPath 形状,候选轮换 + 限流自保 ----
	ansA := ""
	winner := ""
	quotaHits := []string{}
	for _, cand := range cands {
		if cand.ModelID != curID {
			sel := map[string]interface{}{"providerId": cand.ProviderID, "modelId": cand.ModelID}
			if cand.DefaultLevel != "" {
				sel["options"] = map[string]string{"reasoningLevel": cand.DefaultLevel}
			}
			if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{
				"sessionId": sid, "model": sel,
			}, 30*time.Second); err != nil {
				fmt.Printf("== [候选 %s] setModel 失败跳过: %v\n", cand.ModelID, err)
				continue
			}
		}
		fmt.Printf("== [A localPath] 候选模型: %s\n", cand.ModelID)
		ansA, aborted := imageTurn(p, stdin, next, sid, question, []map[string]interface{}{{
			"kind":      "image",
			"filename":  "pa-red.png",
			"mimeType":  "image/png",
			"sizeBytes": len(pngBytes),
			"localPath": imgPath,
		}})
		fmt.Printf("== [A localPath / %s] 回答: %s\n", cand.ModelID, clip(ansA, 200))
		if colorVerdict(ansA) {
			winner = cand.ModelID
			res.ok("A localPath", "模型 "+cand.ModelID+" 认出红色 → zcode 真看到图。回答: "+clip(ansA, 120))
			break
		}
		if aborted {
			quotaHits = append(quotaHits, cand.ModelID)
			// 止损已完成(session/stop);等终局帧落地再试下一个候选。
			time.Sleep(2 * time.Second)
		}
	}
	if winner == "" {
		res.bad("A localPath", fmt.Errorf("全部候选未见红色回答(配额止损 %d 个: %s): %s",
			len(quotaHits), strings.Join(quotaHits, ","), clip(ansA, 120)))
		res.Image = "localPath=false dataBase64=未测(A 未通过)"
		return
	}

	// ---- B 轮:dataBase64 内联形状(用 A 轮成功的模型,会话此刻空闲)----
	fmt.Printf("== [B dataBase64] 模型: %s\n", winner)
	ansB, aborted := imageTurn(p, stdin, next, sid, question, []map[string]interface{}{{
		"kind":       "image",
		"filename":   "pa-red.png",
		"mimeType":   "image/png",
		"sizeBytes":  len(pngBytes),
		"dataBase64": base64.StdEncoding.EncodeToString(pngBytes),
	}})
	fmt.Printf("== [B dataBase64 / %s] 回答: %s\n", winner, clip(ansB, 200))
	verdictB := colorVerdict(ansB) && !aborted
	if verdictB {
		res.ok("B dataBase64", "模型认出红色 → 内联形状也可用。回答: "+clip(ansB, 120))
	} else {
		res.bad("B dataBase64", fmt.Errorf("未见红色回答: %s", clip(ansB, 160)))
	}
	res.Image = fmt.Sprintf("localPath=true(%s) dataBase64=%v", winner, verdictB)
}

// imageTurn 发一轮带 attachments 的 session/send 并等到终局,返回 (流式文本累计, 是否配额/限流止损)。
//
// 【止损规则】轮次进行中观测到 ≥2 次 rate_limited/配额类重试通知(错误 2056、
// 1302 等)就 session/stop 放弃该候选 —— 服务器自动重试 11 次会拖满整个预算。
func imageTurn(p *probe, stdin ioWriter, next func() int64, sid, question string, attachments []map[string]interface{}) (string, bool) {
	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId":   sid,
		"content":     question,
		"attachments": attachments,
	}, 30*time.Second); err != nil {
		return "(session/send 被拒: " + err.Error() + ")", false
	}
	var text strings.Builder
	retries := 0
	stopped := false
	deadline := time.After(120 * time.Second)
	for {
		select {
		case <-deadline:
			if !stopped {
				_, _ = p.call(stdin, next(), "session/stop", map[string]string{"sessionId": sid}, 10*time.Second)
			}
			return text.String() + "(120s 未等到终局,已 stop)", true
		case raw := <-p.events:
			var ev eventEnvelope
			if json.Unmarshal(raw, &ev) != nil || ev.SessionID != sid {
				continue
			}
			// 重试遥测(session.updated):数清 rate_limited/配额,超限即止损
			if ev.Type == "session.updated" {
				if isQuotaRetry(ev.Payload) {
					retries++
					if retries >= 2 && !stopped {
						stopped = true
						fmt.Println("== !! 观测到配额/限流重试,session/stop 止损换候选")
						_, _ = p.call(stdin, next(), "session/stop", map[string]string{"sessionId": sid}, 10*time.Second)
					}
				}
				continue
			}
			switch ev.Type {
			case "turn.completed", "turn.failed":
				var pl struct {
					Response string `json:"response"`
				}
				_ = json.Unmarshal(ev.Payload, &pl)
				if text.Len() == 0 {
					text.WriteString(pl.Response)
				}
				if ev.Type == "turn.failed" {
					text.WriteString("(turn.failed: " + clip(string(ev.Payload), 200) + ")")
				}
				return text.String(), stopped
			case "model.streaming":
				var pl struct {
					Kind  string `json:"kind"`
					Delta string `json:"delta"`
				}
				_ = json.Unmarshal(ev.Payload, &pl)
				if pl.Kind == "text_delta" {
					text.WriteString(pl.Delta)
				}
			}
		}
	}
}

// isQuotaRetry 识别"上游配额/限流"类重试遥测(错误 2056 Token Plan 用量上限、
// 1302 限流,以及 reason=rate_limited / 用量上限字样)。
func isQuotaRetry(payload json.RawMessage) bool {
	s := string(payload)
	if !strings.Contains(s, `"reason":"rate_limited"`) && !strings.Contains(s, "2056") &&
		!strings.Contains(s, "1302") && !strings.Contains(s, "用量上限") {
		return false
	}
	return true
}

// colorVerdict 判断回答是否认出了红色(中英文口径)。
func colorVerdict(ans string) bool {
	a := strings.ToLower(ans)
	return strings.Contains(a, "红") || strings.Contains(a, "red")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

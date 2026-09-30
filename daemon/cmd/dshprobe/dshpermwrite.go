// dshpermwrite.go —— dshprobe 的 live 审批触发专项(-run permwrite)。
//
// 【为什么要这一专项】dsh live 验证轮(2026-09-30)实测:P0 第 4 项的 echo 提示
// 在本机(0.2.0-rc.2,virgin DSH_HOME)构造不出反向请求 —— 基座
// @deepseek-ai/dsh-base cordis.patch.yml 的默认权限面是
// DSH_PERMISSION_MODE ?? "workspace-write" + approval "ask":无文件副作用的命令
// (echo)直接放行。session/request_permission 的真实发射链只有一条:
//   沙箱(Windows ACL)拒绝工作目录外写入 → 模型看到 escalation 提示 → 调
//   escalation 工具(dsh-sandbox approveEscalation,approval 请求带 callId)→
//   ACP 适配器 approval/request 监听 → session/request_permission。
// 本专项用"工作目录外写文件"诱导完整链路,allow/reject 双路各一轮,并以落盘
// 副作用(allow 路文件在、reject 路文件不在)作为审批回路的实判 —— 不是形状
// 推断,是真实审批回路。
package main

import (
	"fmt"
	"os"
	"strings"
)

// doPermissionWrite 跑双路真实审批:先清除残留副作用文件,allow 路诱导写入
// perm-allow.txt、reject 路诱导写入 perm-reject.txt,末尾按文件在/不在裁定。
func doPermissionWrite(c *client, sid, workDir string) item {
	it := item{
		name: "session/request_permission 真实审批回路(工作目录外写文件诱导,allow/reject 双路)",
		cmd:  `session/prompt{"text":"把 hello-dshprobe 写入 <TEMP>\\dshprobe-perm-*.txt(工作目录之外)"} → 沙箱拒绝 → escalation → request_permission 自动应答`,
	}
	allowPath := os.TempDir() + `\dshprobe-perm-allow.txt`
	rejectPath := os.TempDir() + `\dshprobe-perm-reject.txt`
	_ = os.Remove(allowPath)
	_ = os.Remove(rejectPath)

	promptAllow := "请用 pwsh 工具把字符串 hello-dshprobe 写入文件 " + allowPath +
		"(注意:该路径在当前工作目录之外)。如果被沙箱拒绝,请按提示申请提升权限完成写入。写完告诉我结果。"
	promptReject := "请再用 pwsh 工具把字符串 hello-dshprobe 写入文件 " + rejectPath +
		"(该路径同样在当前工作目录之外)。如果被沙箱拒绝,请按提示申请提升权限完成写入。写完告诉我结果。"

	allowFrames, allowNote := doPermissionRound(c, sid, promptAllow, "allow")
	rejectFrames, rejectNote := doPermissionRound(c, sid, promptReject, "reject")

	_, allowErr := os.Stat(allowPath)
	_, rejectErr := os.Stat(rejectPath)
	allowSideEffect := allowErr == nil
	rejectSideEffect := rejectErr == nil
	if allowSideEffect {
		if b, rerr := os.ReadFile(allowPath); rerr == nil {
			allowFrames = append(allowFrames, "[副作用文件内容] "+clip(string(b), 100))
		}
	}
	// 探针不留痕:证据已落记录,副作用文件即删。
	_ = os.Remove(allowPath)
	_ = os.Remove(rejectPath)

	it.frames = append(append([]string{"── allow 路 ──"}, allowFrames...), append([]string{"── reject 路 ──"}, rejectFrames...)...)
	askedAllow := strings.Contains(allowNote, "request_permission")
	askedReject := strings.Contains(rejectNote, "request_permission")
	switch {
	case strings.Contains(allowNote, "受阻") || strings.Contains(rejectNote, "受阻"):
		it.status = "受阻(缺 key)"
	case strings.Contains(allowNote, "失败") || strings.Contains(rejectNote, "失败"):
		it.status = "失败"
	case askedAllow && askedReject && allowSideEffect && !rejectSideEffect:
		it.status = "通过"
	default:
		it.status = "失败"
	}
	it.note = fmt.Sprintf("allow: %s | 副作用文件落盘=%v ;; reject: %s | 副作用文件落盘=%v",
		allowNote, allowSideEffect, rejectNote, rejectSideEffect)
	it.conclusion = "裁定口径:allow 路必须收到 request_permission 且命令真实执行(文件落盘);" +
		"reject 路必须收到 request_permission 且命令不得执行(文件不在)。反向请求形状(options[].optionId/kind、" +
		"toolCall 只带 toolCallId)与应答形状(outcome={outcome:\"selected\",optionId})以原始帧为准。"
	it.impact = "生产审批桥(dshapp.ParseRequestPermission/PermissionOutcomeSelected)与 live 实帧逐字段对账的地面真值;" +
		"同时实锤:dsh ACP 下审批只在 sandbox 升级(elevation)路径发射,普通命令不弹卡 —— 手机端审批卡预期按此校准。"
	return it
}

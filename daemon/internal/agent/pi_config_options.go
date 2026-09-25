// pi_config_options.go —— PiProvider 的原生设置只读呈现(NativeConfigOptions)。
//
// 【地面真值】探针 docs/verify/evidence/pi-native-settings-probe.md(0.87.1,隔离
// agent 目录实测):
//   - get_state 应答带 autoCompactionEnabled(bool)与 steeringMode(字符串,
//     one-at-a-time/all);autoRetryEnabled **不在场**(无可读面,不呈现);
//   - set_auto_compaction / set_auto_retry / set_steering_mode 三条写通道经
//     settingsManager 落到**用户桌面全局配置**(<agentDir>/settings.json,生产路径
//     即 ~/.pi)—— 按红线 1(绝不写用户桌面配置),本 Provider **不提供写**,
//     只做只读事实呈现。
//
// 【为什么走 aliveClient 而不是 clientFor】只读呈现不该有"看一眼就拉起一个
// pi 子进程"的副作用:没有活跃会话时如实返回空目录(手机端整组隐藏)。
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/pocketagent/daemon/internal/store"
)

// 编译期钉死:PiProvider 提供原生设置目录(只读)。
var _ NativeConfigOptions = (*PiProvider)(nil)

// piNativeNote 只读行的人话说明(事实陈述,不用"未知/错误"字样)。
const piNativeNote = "由 Pi 桌面端设置管理:原生写通道会把修改持久写入电脑端 Pi 的全局配置,本通道只读呈现当前生效值。"

// ConfigOptionsCatalog 实现 agent.NativeConfigOptions(只读)。
//
// 数据源 = 活跃 pi 会话的 get_state(piNativeState);没有活跃会话时返回空目录
// (nil, nil —— "现在没有可示的原生状态"是事实,不是错误)。
func (p *PiProvider) ConfigOptionsCatalog(ctx context.Context, t *store.Thread) ([]NativeConfigOption, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var tid string
	if t != nil {
		tid = t.ID
	}
	c := p.aliveClient(tid)
	if c == nil {
		return nil, nil
	}
	st, err := c.GetState(ctx)
	if err != nil {
		return nil, fmt.Errorf("get_state: %w", err)
	}
	return piNativeStateRows(st), nil
}

// SetNativeConfigOption 实现 agent.NativeConfigOptions —— pi 一律拒绝写。
//
// 【为什么不是"暂不支持"】探针实证:pi 的 set_auto_compaction/set_auto_retry/
// set_steering_mode 会把修改持久写入用户桌面全局配置(红线 1 明令不写 ~/.pi)。
// 这是有地面真值的边界声明,不是能力缺口。
func (p *PiProvider) SetNativeConfigOption(ctx context.Context, t *store.Thread, optionID, value string) error {
	return errors.New("pi 原生设置只读:写通道会持久修改电脑端 Pi 的全局配置,请在 Pi 桌面端调整")
}

// piNativeStateRows get_state 应答 → 只读行(键名原样做 id;取值原样字符串化;
// 原生未提供候选清单,Options 恒空 —— 绝不从 bundle 猜测候选伪造选项)。
// 纯函数,单测钉住形状与缺键兜底。
func piNativeStateRows(state map[string]any) []NativeConfigOption {
	rows := make([]NativeConfigOption, 0, 2)
	if v, ok := state["autoCompactionEnabled"]; ok {
		rows = append(rows, NativeConfigOption{
			ID:           "autoCompactionEnabled",
			Name:         "自动压缩",
			Type:         "toggle",
			CurrentValue: jsonBoolLiteral(v),
			Writable:     false,
			Note:         piNativeNote,
		})
	}
	if v, ok := state["steeringMode"]; ok {
		rows = append(rows, NativeConfigOption{
			ID:           "steeringMode",
			Name:         "插话队列模式",
			CurrentValue: fmt.Sprint(v),
			Writable:     false,
			Note:         piNativeNote,
		})
	}
	// autoRetryEnabled:0.87.1 get_state 不带此键(探针实证)—— 即便上游哪天加了,
	// 走上面的同款事实呈现;这里不做任何猜测性行。
	return rows
}

// jsonBoolLiteral 把原生 bool 值渲染成 JSON 字面量(true/false);非 bool 原样
// 字符串化(照镜子,不加工)。
func jsonBoolLiteral(v any) string {
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

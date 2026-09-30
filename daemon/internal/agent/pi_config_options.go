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
// 【会话域点亮】get_state 需要活跃会话才有"该会话"的值:有活客户端直接读;
// 线程已绑定落盘会话但本进程没有活客户端(daemon 重启后从未跑轮)时按需重挂载
// 后读(与 Rename/CompactThread 的"低频动作按需起子进程"同口径);线程从未在
// 电脑上落地(无 SessionFile)→ 空目录(UI 已有空态隐藏),绝不为看一眼凭空
// 拉起子进程。
package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pocketagent/daemon/internal/store"
)

// 编译期钉死:PiProvider 提供原生设置目录(只读)。
var _ NativeConfigOptions = (*PiProvider)(nil)

// piNativeNote 只读行的人话说明(事实陈述,不用"未知/错误"字样)。
const piNativeNote = "由 Pi 桌面端设置管理:原生写通道会把修改持久写入电脑端 Pi 的全局配置,本通道只读呈现当前生效值。"

// stateReader 目录读取对 piapp.Client 的窄接口(单测打桩,不依赖子进程;
// piapp.Client 的 GetState 签名天然满足,生产零适配)。
type stateReader interface {
	GetState(ctx context.Context) (map[string]any, error)
}

// ConfigOptionsCatalog 实现 agent.NativeConfigOptions(只读)。
//
// 数据源 = 该线程活跃 pi 会话的 get_state(piNativeState);没有活跃会话且无法
// 建立时返回空目录(nil, nil —— "现在没有可示的原生状态"是事实,不是错误)。
func (p *PiProvider) ConfigOptionsCatalog(ctx context.Context, t *store.Thread) ([]NativeConfigOption, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var tid string
	if t != nil {
		tid = t.ID
	}
	if c := p.aliveClient(tid); c != nil {
		return piCatalogFrom(ctx, c)
	}
	// 没有活客户端:只有"线程已绑定落盘会话"才值得按需重挂载(把绑定会话重新
	// 变成活跃会话);否则空目录。remount 钩子非 nil = 单测注入,绕过真子进程。
	if t == nil || t.ExternalSession.SessionFile == "" {
		return nil, nil
	}
	var src stateReader
	if p.remount != nil {
		got, err := p.remount(t)
		if err != nil {
			return nil, fmt.Errorf("重挂载 Pi 会话失败: %w", err)
		}
		src = got
	} else {
		nc, err := p.ensureMounted(t)
		if err != nil {
			return nil, err
		}
		src = nc
	}
	return piCatalogFrom(ctx, src)
}

// piCatalogFrom 从 get_state 读取器取样成目录行(读取失败如实报错,不回落空
// 目录冒充"没有设置"—— RPC 层把错误落 resp.Error,"读不到 ≠ 没有值")。
func piCatalogFrom(ctx context.Context, src stateReader) ([]NativeConfigOption, error) {
	st, err := src.GetState(ctx)
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
	// sessionName:0.99.1 get_state 新增键(0.87.1 无;pi-0991 探针实证)。会话
	// 显示名 —— daemon 的 Rename(手机端改名)本就双向同步它,这里只读呈现当前
	// 值,与写通道无关。原生 unset 时整个键缺席(pi 不发 null),缺席即无行。
	if v, ok := state["sessionName"]; ok {
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
			rows = append(rows, NativeConfigOption{
				ID:           "sessionName",
				Name:         "会话显示名(Pi 侧)",
				CurrentValue: s,
				Writable:     false,
				Note:         "由手机端重命名或 Pi 桌面端管理,双端自动同步;此处只读呈现当前值。",
			})
		}
	}
	// autoRetryEnabled:0.87.1/0.99.1 get_state 均不带此键(两轮探针实证)——
	// 即便上游哪天加了,走上面的同款事实呈现;这里不做任何猜测性行。
	// followUpMode:0.87.1 起在场上,但属队列内部模式且非本轮新增,维持不呈现
	// (0.87.1 轮的既有取舍,不在本轮擅自变更)。
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

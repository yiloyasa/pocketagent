// native_config_options.go —— 「原生设置目录透出」的可选扩展能力(原生设置透出轮,
// 2026-09-25)。
//
// 【为什么是可选接口而不是 Provider 必选方法】与 Steerer / PrefsAware 同款取舍:
// 各家 agent 的原生设置面形状差异极大 —— dsh 报告带候选目录的 select 列表且可写
// (探针实证),pi 只在 get_state 里给状态键、且其写通道会持久落到用户桌面全局配置
// (红线禁写,只读呈现),codex/zcode 已有各自的设置面(codex 七项偏好 / zcode
// permission.mode 专属分组)。谁有"目录型原生设置"谁就实现,server 层类型断言探测,
// 没有的家不被迫摆空架子。
//
// 【数据纪律(红线 2 的延伸)】
//   - id/name/type/current_value 一律原生原文,daemon 不翻译不改写;
//   - 候选(options)必须来自原生应答,原生没报就不报 —— 绝不伪造选项;
//   - value 是写回 set_config_option 的原样取值(dsh model 的双层 JSON 编码串
//     逐字节保真),任何加工都是污染。
//
// 【写语义(探针实证,dsh-config-persist-probe.md)】dsh 的 session/set_config_option
// 仅会话内生效、不持久(close 后新会话回到原值)。因此:
//   - thread 给定 → 当场写到那个会话(实时生效);
//   - thread 为 nil → 记为"新会话初始值"(新会话创建时由 provider 灌入),
//     手机设置页语义与"更改默认对新建对话生效"一致。
package agent

import (
	"context"

	"github.com/pocketagent/daemon/internal/store"
)

// NativeConfigOptionValue 一个候选叶子(两种实测形状展平后统一到这)。
type NativeConfigOptionValue struct {
	Value       string // 原样(写回通道的取值)
	Name        string // 原生显示名
	Description string // 原生说明(可空)
	Group       string // 分组名(扁平候选为空)
}

// NativeConfigOption 「原生设置」分组里的一行(字段全部原生原文/事实陈述)。
type NativeConfigOption struct {
	ID           string                    // 原生 id(dsh:"model"/"reasoning_effort";pi:get_state 键名)
	Name         string                    // 原生显示名(原生不报时给daemon侧简洁标签,原文进 Note 兜底)
	Category     string                    // 原生分类(dsh 有;原生不报留空)
	Type         string                    // 原生类型原文(select / …;原生不报留空,不猜)
	CurrentValue string                    // 原样字符串化(编码串不加工;bool 按 JSON 字面量)
	Options      []NativeConfigOptionValue // 原样候选;空 = 原生没报(不伪造)
	Writable     bool                      // 本通道是否提供写(探针实证为准)
	Note         string                    // 人话说明:生效范围/为何只读
}

// NativeConfigOptions 「原生设置目录透出」的可选扩展能力。
type NativeConfigOptions interface {
	// ConfigOptionsCatalog 返回该 agent 的原生设置目录。
	// t == nil → "新会话初始值"视角(dsh 用一次性瞬态会话取样,取完即毁;
	// pi 无活跃会话时如实返回空目录,不为看一眼去拉起子进程)。
	// t != nil → 该会话的实时目录(读取失败如实报错,不回落新会话冒充)。
	ConfigOptionsCatalog(ctx context.Context, t *store.Thread) ([]NativeConfigOption, error)

	// SetNativeConfigOption 写一个原生配置项。value 必须是目录候选里的原样取值
	// (provider 侧校验;目录外取值拒绝,不透传不猜)。t == nil 时按"新会话初始值"
	// 语义落(provider 内部记忆);t != nil 时当场生效到该会话。
	SetNativeConfigOption(ctx context.Context, t *store.Thread, optionID, value string) error
}

// configoptions.go —— dsh 原生配置项目录的形状知识(原生设置透出轮,2026-09-25)。
//
// 【地面真值】探针两份(docs/verify/evidence/dsh-p0-probe.md §2 + dsh-config-persist-probe.md):
//   - session/new|resume 应答顶层 = sessionId + configOptions 数组,每项
//     {id,name,category,type,currentValue,options};
//   - options 字段有两种实测形状:扁平候选 [{value,name,description?}]
//     (reasoning_effort)与分组候选 [{group,name,options:[{value,name,description?}]}]
//     (model);value 原样保留 —— model 的 value 是双层 JSON 编码串,任何加工都是污染;
//   - session/set_config_option 的参数名实测是 **configId**(rc.3 zod 校验;
//     旧实现的 optionId 会被 -32602 Invalid params 拒绝),且改动仅会话内生效、
//     不持久( close 后新会话回到原值)。
//
// 【红线】本文件全部只读解析,绝不写用户桌面 dsh 配置;写只经 dsh 自己的
// session/set_config_option 通道(SetConfigOption)。
package dshapp

import "encoding/json"

// OptionLeaf 是 options 字段展平后的一个候选叶子(两种实测形状统一到这)。
type OptionLeaf struct {
	Value       string `json:"value"`                 // 原样(set_config_option 回传的就是它)
	Name        string `json:"name"`                  // 原生显示名
	Description string `json:"description,omitempty"` // 原生一句话说明(可空)
	Group       string `json:"-"`                     // 分组名(实测 model 按 provider 分组;扁平候选为空)
}

// FlattenOptionLeaves 把一个 configOptions[].options 原始字段解析成候选叶子。
// 未知形状按"没有候选"处理(返回空):照镜子不猜,绝不用假候选污染 UI。
// 纯函数,单测钉住两种实测形状 + 脏报文兜底。
func FlattenOptionLeaves(raw json.RawMessage) []OptionLeaf {
	if len(raw) == 0 {
		return nil
	}
	// 形状①:扁平候选数组(实测 reasoning_effort)。判别条件:每个候选都必须
	// 带 value —— 分组候选的 value 在二级叶子上,顶层只有 group/name。
	var flat []struct {
		Value       string `json:"value"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &flat); err == nil && len(flat) > 0 {
		isFlat := true
		for _, f := range flat {
			if f.Value == "" {
				isFlat = false
				break
			}
		}
		if isFlat {
			out := make([]OptionLeaf, 0, len(flat))
			for _, f := range flat {
				out = append(out, OptionLeaf{Value: f.Value, Name: f.Name, Description: f.Description})
			}
			return out
		}
	}
	// 形状②:分组候选数组(实测 model:group=provider 名,叶子在二级 options)。
	var grouped []struct {
		Group       string `json:"group"`
		Name        string `json:"name"`
		Options     []struct {
			Value       string `json:"value"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"options"`
	}
	if err := json.Unmarshal(raw, &grouped); err != nil {
		return nil
	}
	out := make([]OptionLeaf, 0, len(grouped))
	for _, g := range grouped {
		for _, leaf := range g.Options {
			out = append(out, OptionLeaf{
				Value:       leaf.Value,
				Name:        leaf.Name,
				Description: leaf.Description,
				Group:       firstNonEmptyStr(g.Group, g.Name), // 实测分组用 group;name 兜底可读
			})
		}
	}
	return out
}

func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

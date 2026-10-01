// pi_models_store.go —— pi 模型目录文件(models-store.json)的回退源(桶感知)。
//
// 【为什么单独成文件】get_available_models 在 0.99.1 的 --no-session rpc 下永不
// 应答(设计上 8s 快速失败,见 pi.go fetchModelsUncached),模型列表回退到
// ~/.pi/agent/models-store.json。回退管线经 agency 自适应编译(ExtractCandidates
// 零固定 schema),但 agency 的 provider 继承只认「值是 dict 的分桶」——
// 0.99.1 的真实形状(2026-10-01 本机实证)是「models 是**数组**」:
//
//	{ "<provider>": { "models": [ { "id": …, "name": …, … }, … ],
//	                  "checkedAt": …, "lastModified": …, "etag": … } }
//
// 数组走 agency 的 []any 下钻分支,祖先桶键继承不到 → 候选 Provider 全空,
// pi.go 再拼 "provider/id" 就成了 "/<id>" 的坏 ModelID,手机选中即被
// configurePiSelection 以「无效 Pi 模型」拒绝(整轮失败)—— 回退等于没回退。
// 本文件在 pi 侧补一层桶感知提取:顶层逐桶交 ExtractCandidates(条目形状仍
// 自适应),候选 Provider 一律以桶键盖章(桶键即 provider 名,条目内自带
// provider 字段与它同值,双来源取其一即可)。旧形态(「models」为键即 id 的
// dict,agency 会在桶内把 "models" 误当 provider 名)同样被盖章纠正。
package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agency"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// piModelsStoreSource 返回 models-store.json 的 agency 源(桶感知提取)。
// 名字保持 StoreFileSource 的 "store:" 前缀约定(候选 From 字段诊断口径不变)。
func piModelsStoreSource(path string) agency.Source {
	return agency.SourceFunc{
		Name_: "store:" + path,
		ProbeFn: func(ctx context.Context) ([]agency.Candidate, error) {
			return piModelsStoreProbe(path)
		},
	}
}

// piModelsStoreProbe 读文件并产出带 provider 的候选。
//
// 顶层键分两类:
//   - provider 桶(值是 dict 且键名通过 isProviderBucketKey):桶内提取后以桶键
//     盖章 Provider;
//   - 其他(登记元数据标量 / 非桶容器键):跳过 —— 绝不让 "models"/"checkedAt"
//     之类键名被当成 provider 伪造进 ModelID。
func piModelsStoreProbe(path string) ([]agency.Candidate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	var out []agency.Candidate
	for bucket, v := range root {
		if _, ok := v.(map[string]any); !ok {
			continue // 标量/数组顶层键不是 provider 桶
		}
		if !isProviderBucketKey(bucket) {
			continue // 防伪:非 provider 语义的键不认
		}
		for _, c := range agency.ExtractCandidates(v) {
			c.Provider = bucket // 桶键是 provider 的唯一可信来源(无条件覆盖)
			out = append(out, c)
		}
	}
	return out, nil
}

// isProviderBucketKey 判定顶层键是否 provider 语义:登记元数据与容器词不是
// (与 agency 的 metaKeys/容器词表同口径,本地独立声明 —— agency 的表未导出,
// 且本判定只服务 models-store 的顶层形状)。
func isProviderBucketKey(k string) bool {
	for _, bad := range []string{
		"checkedAt", "checked_at", "lastModified", "last_modified", "etag", "revision",
		"models", "catalog", "providers", "items", "data", "list", "settings",
	} {
		if strings.EqualFold(k, bad) {
			return false
		}
	}
	return k != ""
}

// PiModelsStoreFallbackModels 生产回退:models-store.json → 手机 ModelInfo 列表。
// 从 fetchModelsUncached 抽出(探针 cmd/pimodelsprobe 走同一函数本机实跑,
// 保证"探到的就是生产跑的";导出与 ScanPiSessions 同款面 —— 文件侧只读扫描,
// 无子进程副作用)。
func PiModelsStoreFallbackModels() []pb.ModelInfo {
	return piModelsStoreFallbackFrom(piModelsStorePath())
}

// piModelsStoreFallbackFrom 是回退的可注入内核(测试用临时文件,生产传
// piModelsStorePath())。候选缺 Provider 时跳过:拼不出合法的
// "pi/<provider>/<id>" ModelID,与其发一个选中必败的坏候选,不如宁缺毋假。
func piModelsStoreFallbackFrom(path string) []pb.ModelInfo {
	fb := agency.Compile(context.Background(), 10*time.Second,
		piModelsStoreSource(path))
	if len(fb) == 0 {
		return nil
	}
	out := make([]pb.ModelInfo, 0, len(fb))
	for _, c := range fb {
		if c.Provider == "" {
			continue
		}
		display := c.DisplayName
		if display == "" {
			display = c.ID
		}
		supportsImages := false
		for _, in := range c.Input {
			if in == "image" {
				supportsImages = true
				break
			}
		}
		// 直接 append 复合字面量:pb 消息含互斥量,append 已有值会被 vet 拒绝
		// (与 fetchModelsUncached 直连路径同款写法)。
		out = append(out, pb.ModelInfo{
			Id:               c.Provider + "/" + c.ID,
			Provider:         "pi",
			DisplayName:      display,
			Available:        true,
			ContextWindow:    int32(c.ContextWindow),
			SupportsThinking: c.Reasoning,
			SupportsImages:   supportsImages,
		})
	}
	return out
}

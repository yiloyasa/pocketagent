package agent

// codex_askuser_feature_test.go —— 反问卡"运行时能力宣告"的纯函数面回归钉子。
//
// 【背景 / 重要变更】早期 daemon 为解锁 Default 模式的 request_user_input,会在
// thread/start、thread/resume 的 config 覆盖里注入
// features.default_mode_request_user_input=true。该特性在 codex 0.155.1 仍是
// underDevelopment,注入即触发 codex 的"Under-development features enabled"警告
// (真机实测每次会话都弹)。由于抑制警告的唯一官方途径是改写用户 ~/.codex/config.toml
// (本项目红线永久封死),现已**不再注入**该特性。
//
// 本文件现在只验证两件事:
//   1) experimentalFeature/list 应答解析(askUserFeatureAdvertised):宣告判定。
//   2) AskUserInputSupported() 的按 gen 缓存语义(纯能力发现,不写入)。
// 以及 callThreadResume 的"单纯 resume 透传"语义(不再有 features 剥离重试)。
//
// 旧的"注入路径"测试(TestMergeFeatureOverride 等)已随注入逻辑一同删除:
// 注入既已移除,测它只会产生误导。

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAskUserFeatureAdvertised(t *testing.T) {
	// codex 0.155.1 实测形状:default=false enabled=false stage=underDevelopment
	// —— 宣告了(名字在册且阶段可注入),应当注入。
	raw := json.RawMessage(`{"data":[
		{"name":"steer","defaultEnabled":true,"enabled":true,"stage":"beta"},
		{"name":"default_mode_request_user_input","defaultEnabled":false,"enabled":false,"stage":"underDevelopment"}
	]}`)
	if !askUserFeatureAdvertised(raw) {
		t.Fatalf("underDevelopment 阶段的在册特性应视为宣告")
	}

	// 未来 codex 把特性下线:注册表保留墓碑(stage=removed,同实测
	// unavailable_dummy_tools)。对墓碑开开关毫无意义,不算宣告。
	raw = json.RawMessage(`{"data":[
		{"name":"default_mode_request_user_input","defaultEnabled":false,"enabled":false,"stage":"removed"}
	]}`)
	if askUserFeatureAdvertised(raw) {
		t.Fatalf("removed 阶段不应视为宣告")
	}

	// 旧版 codex:注册表里根本没有这条(或整包没有该方法返回别的形状)→ 不宣告。
	raw = json.RawMessage(`{"data":[{"name":"steer","defaultEnabled":true,"enabled":true,"stage":"beta"}]}`)
	if askUserFeatureAdvertised(raw) {
		t.Fatalf("未在册不应视为宣告")
	}
	if askUserFeatureAdvertised(json.RawMessage(`not-json`)) {
		t.Fatalf("坏 JSON 应返回 false")
	}
	if askUserFeatureAdvertised(json.RawMessage(`{}`)) {
		t.Fatalf("缺 data 字段应返回 false")
	}
}

func TestAskUserInputSupportedCachesPerGen(t *testing.T) {
	// 零值 provider(client=nil):探针调用不可达,应返回 false 且不 panic ——
	// 这是"codex 未起/测试环境"的降级路径:能力按"不可用"判定(不再注入)。
	p := &CodexProvider{}
	if p.AskUserInputSupported() {
		t.Fatalf("client==nil 时应返回 false(能力按不可用判定)")
	}
	// 结果按 gen 记账:伪造一条"上一代曾宣告"的缓存,gen 未变时不得复用
	// (client==nil 的重算恒为 false 并覆盖记账);再验证同代二次调用走缓存。
	p.askFeatGen, p.askFeatVal = 99, true
	p.gen.Store(1)
	if p.AskUserInputSupported() {
		t.Fatalf("gen 变化后应重算(client=nil 恒 false)")
	}
	if p.askFeatGen != 1 || p.askFeatVal {
		t.Fatalf("重算后应按当前 gen 记账 false, got gen=%d val=%v", p.askFeatGen, p.askFeatVal)
	}
	p.askFeatVal = true // 伪造缓存污染:同代二次调用应直接吃缓存并"复现"污染值
	if !p.AskUserInputSupported() {
		t.Fatalf("同 gen 应走缓存,不再触发重算")
	}
}

// ---- thread/resume 降级护栏回归钉子 ----
//
// 【背景】原简历点只在 thread/start 加了"带 features 覆盖失败 → 摘除 features 重试"的
// 护栏;thread/resume 没有。一旦某版 codex 宣告了该特性却不认 config.features 键,
// resume 直接失败 → 调用方走 CodexThreadID="" 改开新线程 → 用户老会话历史全丢,且只
// 表现为"AI 失忆"。callThreadResume 抽出了这层护栏,以下用例覆盖三条分支。
//
// 不依赖真实子进程:把 p.client.Call 替换成一个脚本化 fake caller(call 形参注入)。

// fakeResumeCaller 记录每次 thread/resume 调用,并据 script 返回错误(用尽后一律成功)。
// 这样无需真实 codex 子进程即可驱动 callThreadResume 的两条分支。
type fakeResumeCaller struct {
	script []error
	idx    int
	calls  []resumeCallRecord
}

type resumeCallRecord struct {
	hasConfig bool     // params 里是否带 config 键
	cfgEmpty  bool     // config(若带)是否为空 map
	cfgKeys   []string // config 里的键(验证"原样透传、无 features 注入/剥离")
}

func (f *fakeResumeCaller) call(method string, params interface{}) (json.RawMessage, error) {
	rec := resumeCallRecord{}
	if m, ok := params.(map[string]interface{}); ok {
		if cfg, ok := m["config"].(map[string]interface{}); ok {
			rec.hasConfig = true
			if len(cfg) == 0 {
				rec.cfgEmpty = true
			}
			for k := range cfg {
				rec.cfgKeys = append(rec.cfgKeys, k)
			}
		}
	}
	f.calls = append(f.calls, rec)
	var err error
	if f.idx < len(f.script) {
		err = f.script[f.idx]
	}
	f.idx++
	return json.RawMessage(`{}`), err
}

// ---- callThreadResume 新语义:单纯 resume 透传,无 features 剥离重试 ----
//
// 旧版的"带 features 覆盖失败 → 摘除 features 重试"护栏是专为注入 under-development
// 特性设的逃生舱。现已不再注入任何特性(见 codex.go 的 AskUserInputSupported 注释),
// callThreadResume 退化为"带 config 透传、失败即透传错误、绝不重试"。以下用例覆盖
// 三条关键不变量。

func TestCallThreadResumeForwardsConfig(t *testing.T) {
	// 带 config 覆盖且第一次就成功:单次调用,且 config 原样透传。
	cfg := map[string]interface{}{"model_verbosity": "low"}
	f := &fakeResumeCaller{script: []error{}} // 第一次即成功
	err := (&CodexProvider{}).callThreadResume("tid-1", cfg, f.call)
	if err != nil {
		t.Fatalf("第一次成功应返回 nil, got %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("成功路径仅调用一次, 实际 %d 次", len(f.calls))
	}
	if !f.calls[0].hasConfig {
		t.Fatalf("应带 config 覆盖, got %+v", f.calls[0])
	}
	if len(f.calls[0].cfgKeys) != 1 || f.calls[0].cfgKeys[0] != "model_verbosity" {
		t.Fatalf("config 应原样透传, got keys=%v", f.calls[0].cfgKeys)
	}
}

func TestCallThreadResumeOmitsConfigWhenEmpty(t *testing.T) {
	// 空 config 覆盖:不得带 config 键,避免给 codex 传空 config 触发未知拒绝。
	f := &fakeResumeCaller{script: []error{}}
	err := (&CodexProvider{}).callThreadResume("tid-2", map[string]interface{}{}, f.call)
	if err != nil {
		t.Fatalf("空 config 成功应返回 nil, got %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("应调用一次, 实际 %d 次", len(f.calls))
	}
	if f.calls[0].hasConfig {
		t.Fatalf("空 config 不得带 config 键, got %+v", f.calls[0])
	}
}

func TestCallThreadResumeReturnsErrorNoRetry(t *testing.T) {
	// 失败:直接透传真错误,绝不重试(旧版的 features 剥离重试已整段删除)。
	f := &fakeResumeCaller{script: []error{errors.New("boom")}}
	err := (&CodexProvider{}).callThreadResume("tid-3", map[string]interface{}{"model_verbosity": "low"}, f.call)
	if err == nil {
		t.Fatalf("失败应透传真错误")
	}
	if len(f.calls) != 1 {
		t.Fatalf("失败不得重试, 实际 %d 次", len(f.calls))
	}
}

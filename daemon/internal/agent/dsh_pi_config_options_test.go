// dsh_pi_config_options_test.go —— 原生设置目录透出的行为契约(接口 fake,红灯先行):
//   - dsh:探针夹具目录 → 展示行(value 逐字节原样);写校验(目录外/裸模型名拒绝);
//     "新会话初始值"记录 + ensureSession 建会话时灌入(探针实证仅会话内生效);
//     线程绑定 → session/resume 实时目录;
//   - pi:get_state 键 → 只读行(探针实证写通道落桌面全局配置 → 一律拒绝写)。
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/store"
)

// 编码串夹具:探针第 2 项 model 候选叶子 value 的解析后形态(双层 JSON 编码串)。
const wantEncodedFlash = `["deepseek-official","deepseek-flash"]`

// ---- dsh:目录透出 ----

// TestDshCatalogRowsFromProbeFixture 探针夹具目录 → 展示行:id/name/type 原样,
// model 候选是双层编码串逐字节,effort 候选 4 档;行级 Writable=true + 生效范围 note。
func TestDshCatalogRowsFromProbeFixture(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-catalog", ConfigOptions: probeCatalogOpts(t)},
	}
	p := newTestDshProvider(t, fake)

	rows, err := p.ConfigOptionsCatalog(context.Background(), nil)
	if err != nil {
		t.Fatalf("目录读取应成功: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("探针夹具应 2 行(model/effort),得 %d: %+v", len(rows), rows)
	}
	model := rows[0]
	if model.ID != "model" || model.Name != "Model" || model.Type != "select" || model.Category != "model" {
		t.Fatalf("model 行字段应原样: %+v", model)
	}
	if !model.Writable || model.Note == "" {
		t.Fatalf("dsh 行应可写且带生效范围说明: %+v", model)
	}
	if len(model.Options) != 4 {
		t.Fatalf("model 应 4 候选叶子,得 %d", len(model.Options))
	}
	// 双层编码串逐字节(红线:任何加工都是污染)。
	if got := model.Options[0].Value; got != wantEncodedFlash {
		t.Fatalf("编码串应逐字节原样: got=%q want=%q", got, wantEncodedFlash)
	}
	if model.Options[0].Group != "deepseek-official" {
		t.Fatalf("分组名应记进叶子: %+v", model.Options[0])
	}
	effort := rows[1]
	if effort.ID != "reasoning_effort" || len(effort.Options) != 4 || effort.Options[3].Value != "max" {
		t.Fatalf("effort 行应 4 档(off/low/high/max): %+v", effort)
	}
	// 目录快照已登记(写校验的依据)。
	if len(p.rawCatalogSnapshot()) != 2 {
		t.Fatalf("取样后原生目录应入缓存: %d", len(p.rawCatalogSnapshot()))
	}
}

// TestDshSetNativeDefaultRecordsAndPushesOnNewSession 无线程写入 → 记"新会话初始值";
// ensureSession 建新会话时灌入(参数 configId 由 dshapp 层钉,这里断言 optionID/value)。
func TestDshSetNativeDefaultRecordsAndPushesOnNewSession(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-new", ConfigOptions: probeCatalogOpts(t)},
	}
	p := newTestDshProvider(t, fake)
	// 先取样建目录(校验依据),再无线程写初始值。
	if _, err := p.ConfigOptionsCatalog(context.Background(), nil); err != nil {
		t.Fatalf("目录读取应成功: %v", err)
	}
	if err := p.SetNativeConfigOption(context.Background(), nil, "reasoning_effort", "low"); err != nil {
		t.Fatalf("目录内取值应被接受: %v", err)
	}
	if got := p.nativeDefaultsSnapshot()["reasoning_effort"]; got != "low" {
		t.Fatalf("初始值应记录: %+v", p.nativeDefaultsSnapshot())
	}

	// 新会话:ensureSession 建会话后应灌入初始值。
	p.gen.Store(1)
	thread := &store.Thread{ID: "t1", Workspace: `C:\ws`}
	sid, err := p.ensureSession(context.Background(), TurnRequest{Thread: thread, Workspace: `C:\ws`})
	if err != nil {
		t.Fatalf("ensureSession 应成功: %v", err)
	}
	if sid != "s-new" {
		t.Fatalf("会话 id 不符: %q", sid)
	}
	calls := fake.snapshot().setCalls
	found := false
	for _, c := range calls {
		if c.sessionID == "s-new" && c.optionID == "reasoning_effort" && c.value == "low" {
			found = true
		}
	}
	if !found {
		t.Fatalf("新会话应灌入初始值 set_config_option(reasoning_effort,low),实际调用: %+v", calls)
	}
}

// TestDshSetNativeRejectsOutsideCatalog 写校验:裸模型名(红线)与目录外 optionID 一律拒绝。
func TestDshSetNativeRejectsOutsideCatalog(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-catalog", ConfigOptions: probeCatalogOpts(t)},
	}
	p := newTestDshProvider(t, fake)
	if _, err := p.ConfigOptionsCatalog(context.Background(), nil); err != nil {
		t.Fatalf("目录读取应成功: %v", err)
	}

	if err := p.SetNativeConfigOption(context.Background(), nil, "model", "deepseek-v4-pro"); err == nil {
		t.Fatal("裸模型名必须被拒绝(红线:禁止裸模型名)")
	}
	if err := p.SetNativeConfigOption(context.Background(), nil, "nope", "low"); err == nil {
		t.Fatal("目录外 optionID 必须被拒绝")
	}
	if calls := fake.snapshot().setCalls; len(calls) != 0 {
		t.Fatalf("被拒的写不得下发给原生通道: %+v", calls)
	}
}

// TestDshCatalogThreadBound 线程绑定 → session/resume 实时目录(探针修正②:
// resume 应答带 configOptions),并顺手刷新目录缓存。
func TestDshCatalogThreadBound(t *testing.T) {
	opts := probeCatalogOpts(t)
	opts[1].CurrentValue = "low" // 桌面端已改档的实时形态
	fake := &fakeDshClient{
		resumeBy: map[string]*dshapp.SessionState{
			"s-live": {SessionID: "s-live", ConfigOptions: opts},
		},
	}
	p := newTestDshProvider(t, fake)
	p.gen.Store(1)
	thread := &store.Thread{ID: "t1", AgentID: "dsh", ExternalSession: store.ExternalSessionRef{SessionID: "s-live"}}

	rows, err := p.ConfigOptionsCatalog(context.Background(), thread)
	if err != nil {
		t.Fatalf("线程绑定目录读取应成功: %v", err)
	}
	if hits := fake.snapshot().resumeHits; len(hits) != 1 || hits[0] != "s-live" {
		t.Fatalf("应 resume 该会话一次: %+v", hits)
	}
	if rows[1].CurrentValue != "low" {
		t.Fatalf("实时 currentValue 应透传: %+v", rows[1])
	}
	if snap := p.rawCatalogSnapshot(); len(snap) != 2 || snap[1].CurrentValue != "low" {
		t.Fatalf("resume 目录应刷新缓存: %+v", snap)
	}
}

// TestDshOverlayCurrent 覆盖语义:已确认灌入的项覆盖 currentValue,其余原样。
func TestDshOverlayCurrent(t *testing.T) {
	opts := probeCatalogOpts(t)
	out := dshOverlayCurrent(opts, map[string]string{"reasoning_effort": "low"})
	if out[1].CurrentValue != "low" {
		t.Fatalf("已灌入项应覆盖: %+v", out[1])
	}
	if out[0].CurrentValue != opts[0].CurrentValue {
		t.Fatalf("未灌入项应原样: %+v", out[0])
	}
	if len(dshOverlayCurrent(nil, map[string]string{"a": "b"})) != 0 {
		t.Fatal("空目录应原样返回")
	}
}

// ---- pi:只读呈现 ----

// TestPiNativeStateRows get_state 键 → 只读行(探针实证形状);缺键不摆行;
// 候选恒空(原生未报可选清单,绝不伪造选项)。
func TestPiNativeStateRows(t *testing.T) {
	rows := piNativeStateRows(map[string]any{
		"autoCompactionEnabled": false,
		"steeringMode":          "one-at-a-time",
		"model":                 map[string]any{"id": "x"}, // 非设置键不呈现
	})
	if len(rows) != 2 {
		t.Fatalf("应 2 行,得 %d: %+v", len(rows), rows)
	}
	if rows[0].ID != "autoCompactionEnabled" || rows[0].CurrentValue != "false" || rows[0].Writable {
		t.Fatalf("自动压缩行应只读且值原样: %+v", rows[0])
	}
	if rows[1].ID != "steeringMode" || rows[1].CurrentValue != "one-at-a-time" || rows[1].Writable {
		t.Fatalf("插话队列行应只读且值原样: %+v", rows[1])
	}
	for _, r := range rows {
		if r.Note == "" || !strings.Contains(r.Note, "只读") {
			t.Fatalf("只读行必须带事实说明: %+v", r)
		}
		if len(r.Options) != 0 {
			t.Fatalf("pi 行候选必须为空(不伪造): %+v", r)
		}
	}

	// 缺键兜底:空状态 → 零行。
	if rows := piNativeStateRows(map[string]any{}); len(rows) != 0 {
		t.Fatalf("空状态应零行: %+v", rows)
	}
}

// TestPiSetNativeRefuses 红线钉子:pi 原生设置一律拒绝写(写通道持久落桌面全局配置)。
func TestPiSetNativeRefuses(t *testing.T) {
	p := &PiProvider{}
	if err := p.SetNativeConfigOption(context.Background(), nil, "autoCompactionEnabled", "false"); err == nil {
		t.Fatal("pi 原生设置必须拒绝写(红线:绝不写用户桌面配置)")
	}
}

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type selectionFake struct {
	calls []string
	level string
	fail  string
}

func (f *selectionFake) CallCtx(_ context.Context, method string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	if method == f.fail {
		return nil, errors.New("rejected")
	}
	if method == "get_available_thinking_levels" {
		return json.RawMessage(`{"levels":["low","high","max"]}`), nil
	}
	if method == "get_state" {
		return json.RawMessage(`{"model":{"id":"glm-5.3","provider":"zai"},"thinkingLevel":"` + f.level + `"}`), nil
	}
	return json.RawMessage(`{}`), nil
}

func TestPiSelectionRejectsClampedAndFailedSettings(t *testing.T) {
	for _, tc := range []struct{ effort, actual, fail string }{
		{"minimal", "high", ""},
		{"low", "high", ""},
		{"low", "low", "set_model"},
		{"low", "low", "set_thinking_level"},
	} {
		f := &selectionFake{level: tc.actual, fail: tc.fail}
		_, _, _, err := configurePiSelection(context.Background(), f, "pi/zai/glm-5.3", tc.effort)
		if err == nil {
			t.Fatalf("silently accepted %+v; calls=%v", tc, f.calls)
		}
		if tc.effort == "minimal" && strings.Contains(strings.Join(f.calls, ","), "set_thinking_level") {
			t.Fatal("must reject unsupported level before Pi clamps it")
		}
	}
}

func TestPiSelectionConfirmsRequestedLow(t *testing.T) {
	f := &selectionFake{level: "low"}
	model, effort, _, err := configurePiSelection(context.Background(), f, "pi/zai/glm-5.3", "low")
	if err != nil || model != "pi/zai/glm-5.3" || effort != "low" {
		t.Fatalf("model=%s effort=%s error=%v", model, effort, err)
	}
}

// ---- 1.0.2 对齐(2026-10-07):Model not found 回退 = 跳过 set_model ----

// piNotFoundFake 模拟 1.0.2 的 set_model 精确匹配落空(bundle:handler 只按
// find(provider===…&&id===…) 定位,无「跟随默认」特例 → 恒 "Model not found"),
// get_state 回读真值。
type piNotFoundFake struct {
	calls []string
}

func (f *piNotFoundFake) CallCtx(_ context.Context, method string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	switch method {
	case "set_model":
		return nil, errors.New("Model not found: zai-coding-cn/glm-5.3")
	case "get_state":
		return json.RawMessage(`{"model":{"id":"glm-5.3","provider":"zai-coding-cn"},"thinkingLevel":"high"}`), nil
	}
	return json.RawMessage(`{}`), nil
}

// 回退语义:原模型失效时不再发任何补救 set_model(旧候选 {"modelId":"default"}
// 与 {"provider":"","modelId":""} 在 1.0.2 下双双必败,白打 RPC),只做 get_state
// 回读,并把真值与 warn 一起上抛。
func TestPiSelectionFallbackSkipsFutileSetModel(t *testing.T) {
	f := &piNotFoundFake{}
	actual, effort, warn, err := configurePiSelection(context.Background(), f, "pi/zai-coding-cn/glm-5.3", "")
	if err != nil {
		t.Fatalf("回退路径不得整轮失败: %v", err)
	}
	if actual != "pi/zai-coding-cn/glm-5.3" || effort != "high" {
		t.Fatalf("回读真值应上抛: actual=%s effort=%s", actual, effort)
	}
	if warn == "" || !strings.Contains(warn, "已从 Pi 目录移除") {
		t.Fatalf("warn 缺失或文案漂移: %q", warn)
	}
	if strings.Join(f.calls, ",") != "set_model,get_state" {
		t.Fatalf("必须恰好一次 set_model(主路径)+一次 get_state 回读,不许补救 set_model: %v", f.calls)
	}
}

// piReadbackFailFake 在 set_model 必败之外连 get_state 也失败:回读不可得。
type piReadbackFailFake struct{ calls []string }

func (f *piReadbackFailFake) CallCtx(_ context.Context, method string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	if method == "set_model" {
		return nil, errors.New("Model not found: zai-coding-cn/glm-5.3")
	}
	return nil, errors.New("readback unavailable")
}

// 回读也失败时软着陆:warn 仍在、不发明真值、不失败(与既有"真值不可得"分支
// 一致,同样不得出现补救 set_model)。
func TestPiSelectionFallbackReadbackFailureStaysSoft(t *testing.T) {
	f := &piReadbackFailFake{}
	actual, _, warn, err := configurePiSelection(context.Background(), f, "pi/zai-coding-cn/glm-5.3", "")
	if err != nil || actual != "" || warn == "" {
		t.Fatalf("回读失败应软着陆: actual=%q warn=%q err=%v", actual, warn, err)
	}
	if strings.Join(f.calls, ",") != "set_model,get_state" {
		t.Fatalf("调用序列漂移(不许补救 set_model): %v", f.calls)
	}
}

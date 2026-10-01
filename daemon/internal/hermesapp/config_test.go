package hermesapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setHermesHomeForTest 把 hermes 家指到临时目录,返回还原函数。
func setHermesHomeForTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HERMES_HOME", dir)
	return dir
}

func TestSetReasoningEffortCreatesAgentSection(t *testing.T) {
	home := setHermesHomeForTest(t)
	if err := SetReasoningEffort("high"); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(home, "config.yaml"))
	got := string(raw)
	if !strings.Contains(got, "agent:") || !strings.Contains(got, "reasoning_effort: high") {
		t.Fatalf("应写入 agent.reasoning_effort= high,得到:\n%s", got)
	}
}

func TestSetReasoningEffortReplacesExistingKey(t *testing.T) {
	home := setHermesHomeForTest(t)
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("model:\n  default: deepseek-flash\nagent:\n  reasoning_effort: low\n"), 0o644)
	if err := SetReasoningEffort("xhigh"); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(cfg)
	got := string(raw)
	if strings.Contains(got, "reasoning_effort: low") {
		t.Fatalf("旧值 low 应被替换:\n%s", got)
	}
	if !strings.Contains(got, "reasoning_effort: xhigh") || !strings.Contains(got, "default: deepseek-flash") {
		t.Fatalf("新值 xhigh 与 model 段应共存:\n%s", got)
	}
}

func TestSetReasoningEffortEmptyRemovesKey(t *testing.T) {
	home := setHermesHomeForTest(t)
	cfg := filepath.Join(home, "config.yaml")
	os.WriteFile(cfg, []byte("agent:\n  reasoning_effort: high\n"), 0o644)
	if err := SetReasoningEffort(""); err != nil {
		t.Fatalf("set: %v", err)
	}
	raw, _ := os.ReadFile(cfg)
	if strings.Contains(string(raw), "reasoning_effort") {
		t.Fatalf("空 effort 应删键回默认,得到:\n%s", string(raw))
	}
}

func TestIsValidReasoningEffort(t *testing.T) {
	for _, ok := range []string{"minimal", "low", "medium", "high", "xhigh", "max", "ultra", "HIGH"} {
		if !IsValidReasoningEffort(ok) {
			t.Errorf("IsValidReasoningEffort(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "hgih", "extreme"} {
		if IsValidReasoningEffort(bad) {
			t.Errorf("IsValidReasoningEffort(%q) = true, want false", bad)
		}
	}
}

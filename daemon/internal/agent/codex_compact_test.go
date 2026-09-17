package agent

import "testing"

// usageSum 仅用于测试:本 turn 上下文占用 token(桌面端同口径:input + output)。
func usageSum(in, out, cr, cw, rs int64) int64 {
	return in + out
}

func TestCompactDecideArmedFiresAboveThreshold(t *testing.T) {
	const window = 272_000
	// 85% 正好等于阈值 → 触发
	total := int64(float64(window) * 0.85)
	ns, fire := compactDecide(compactArmed, total, window, autoCompactThreshold, autoCompactHysteresis)
	if !fire {
		t.Fatalf("占用率=阈值应触发压缩, got fire=%v", fire)
	}
	if ns != compactFired {
		t.Fatalf("触发后状态应为 compactFired, got %v", ns)
	}
	// 略低于阈值(84%)→ 不触发
	total = int64(float64(window) * 0.84)
	if _, fire := compactDecide(compactArmed, total, window, autoCompactThreshold, autoCompactHysteresis); fire {
		t.Fatalf("占用率<阈值不应触发, got fire=true")
	}
}

func TestCompactDecideFiredStaysArmedUntilDrop(t *testing.T) {
	const window = 272_000
	// 已发起压缩,占用率仍高(0.90)→ 不重复触发,保持待命
	total := int64(float64(window) * 0.90)
	ns, fire := compactDecide(compactFired, total, window, autoCompactThreshold, autoCompactHysteresis)
	if fire {
		t.Fatalf("已发起后不应再次触发, got fire=true")
	}
	if ns != compactFired {
		t.Fatalf("占用率未回落应维持 compactFired, got %v", ns)
	}
	// 占用率回落到阈值-滞回以下(0.70 < 0.85-0.10)→ 重新武装(armed),但仍不触发
	total = int64(float64(window) * 0.70)
	ns, fire = compactDecide(compactFired, total, window, autoCompactThreshold, autoCompactHysteresis)
	if fire {
		t.Fatalf("回落过程不应触发, got fire=true")
	}
	if ns != compactArmed {
		t.Fatalf("回落到阈值-滞回以下应重新武装, got %v", ns)
	}
	// 重新武装后再升到 0.90 → 再次触发
	total = int64(float64(window) * 0.90)
	ns, fire = compactDecide(compactArmed, total, window, autoCompactThreshold, autoCompactHysteresis)
	if !fire || ns != compactFired {
		t.Fatalf("重新武装后越阈应再次触发, got fire=%v state=%v", fire, ns)
	}
}

func TestCompactDecideUnknownWindowNeverFires(t *testing.T) {
	// 窗口未知(window<=0):无论占用率多高都不触发,状态保持不变。
	ns, fire := compactDecide(compactArmed, 999_999, 0, autoCompactThreshold, autoCompactHysteresis)
	if fire {
		t.Fatalf("窗口未知时绝不应触发压缩, got fire=true")
	}
	if ns != compactArmed {
		t.Fatalf("窗口未知时状态不应改变, got %v", ns)
	}
	// 已经 fired 但窗口未知:也不应因窗口未知而误判;保持原状态
	ns, fire = compactDecide(compactFired, 999_999, 0, autoCompactThreshold, autoCompactHysteresis)
	if fire || ns != compactFired {
		t.Fatalf("窗口未知不应改变 fired 状态, got fire=%v state=%v", fire, ns)
	}
}

func TestUsageSumHelper(t *testing.T) {
	// 上下文占用口径 = input + output(缓存读/推理不重复计入,与桌面端一致)。
	if got := usageSum(100, 50, 30, 10, 5); got != 150 {
		t.Fatalf("usageSum = %d, want 150", got)
	}
}

package agent

// zcode_threadmap_test.go —— P1-2 回归钉子:threadMap 并发读写必须全走 threadMu。
//
// 【背景】写方 ensureSession(轮次 goroutine,持 turnMu)与读方
// loadedZcodeSessionID(被 SetGoal 在 server RPC goroutine 调用)之间没有共同
// 锁 —— 修复前裸 map 并发读写会触发 Go runtime fatal("concurrent map read and
// map write"),整个 daemon 崩、所有会话全断。本测试并发轰炸读写两路,
// 锁缺失时 `-race` 必报;并用"不存在的 node"短路 SetGoal 的子进程拉起,离线可跑。

import (
	"fmt"
	"sync"
	"testing"
)

func TestZcodeThreadMapConcurrentReadWrite(t *testing.T) {
	// LocateZCode 尊重 ZCODE_NODE/ZCODE_CJS:指向不存在的文件 = 模拟"没装",
	// SetGoal 走 withClient 快速失败 → 暂存(不真起 ZCode 子进程)。
	t.Setenv("ZCODE_NODE", t.TempDir()+`\missing-node.exe`)
	t.Setenv("ZCODE_CJS", t.TempDir()+`\missing-zcode.cjs`)
	p := NewZcodeProvider()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		id := fmt.Sprintf("th-%d", g)
		// 写方:ensureSession 命中查询与记账走的同一对入口。
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				p.setZcodeThreadRef(id, threadRef{id: fmt.Sprintf("sess-%d", j), gen: j})
				p.zcodeThreadRefFor(id)
			}
		}()
		// 读方:SetGoal 的读路径(loadedZcodeSessionID,修复前无锁裸读)。
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = p.loadedZcodeSessionID(id)
				_ = p.SetGoal(id, "目标") // 子进程拉不起 → 暂存,读路径已覆盖
			}
		}()
	}
	wg.Wait()
}

// TestZcodeThreadRefRoundTrip 钉死加锁读写入口的语义(与裸 map 一致):
// 写入后可读出、覆盖以最后一次为准、未写的线程返回未命中。
func TestZcodeThreadRefRoundTrip(t *testing.T) {
	p := NewZcodeProvider()
	if _, hit := p.zcodeThreadRefFor("th-x"); hit {
		t.Fatal("未写的线程不应命中")
	}
	p.setZcodeThreadRef("th-x", threadRef{id: "sess-1", gen: 3})
	ref, hit := p.zcodeThreadRefFor("th-x")
	if !hit || ref.id != "sess-1" || ref.gen != 3 {
		t.Fatalf("读写往返不一致: %+v %v", ref, hit)
	}
	p.setZcodeThreadRef("th-x", threadRef{id: "sess-2", gen: 4})
	if ref, _ := p.zcodeThreadRefFor("th-x"); ref.id != "sess-2" {
		t.Fatalf("重复写入应以最新为准: %+v", ref)
	}
	// gen 校验语义:loadedZcodeSessionID 只认当前 gen 的记录(子进程重启后旧 id 无效)
	p.setZcodeThreadRef("th-y", threadRef{id: "sess-old", gen: 0}) // 当前 gen=0
	if got := p.loadedZcodeSessionID("th-y"); got != "sess-old" {
		t.Fatalf("gen 命中应返回 id,得到 %q", got)
	}
	p.gen.Add(1) // 子进程重启(gen 前进)
	if got := p.loadedZcodeSessionID("th-y"); got != "" {
		t.Fatalf("gen 过期应返回空串,得到 %q", got)
	}
}

// TestZcodeSetGoalStashesWhenNoClient SetGoal 的失败语义(与修复前一致):
// 子进程拉不起来 → 暂存 + 返回错误;线程未挂载 → 暂存 + 不算失败。
func TestZcodeSetGoalStashesWhenNoClient(t *testing.T) {
	t.Setenv("ZCODE_NODE", t.TempDir()+`\missing-node.exe`)
	t.Setenv("ZCODE_CJS", t.TempDir()+`\missing-zcode.cjs`)
	p := NewZcodeProvider()

	// 拉不起子进程:暂存 + 报错(调用方落库不受影响)
	if err := p.SetGoal("th-1", "目标A"); err == nil {
		t.Fatal("子进程拉不起应返回错误")
	}
	if g, ok := p.takePendingGoal("th-1"); !ok || g != "目标A" {
		t.Fatalf("应暂存目标等补发: %q %v", g, ok)
	}

	// 手工伪造一个"已挂载"状态(绕开真子进程):SetGoal 仍会因拉不起而报错暂存
	p.setZcodeThreadRef("th-2", threadRef{id: "sess-2", gen: 0})
	if err := p.SetGoal("th-2", "目标B"); err == nil {
		t.Fatal("无可用子进程时下发应报错")
	}
	if g, ok := p.takePendingGoal("th-2"); !ok || g != "目标B" {
		t.Fatalf("失败后应放回暂存: %q %v", g, ok)
	}
}

// abandoned_turn_test.go —— CallTurnCtx 放弃后迟到 stopReason 一次性认领的契约
// (审查 open 项 9,2026-10-04;形态学 codexapp abandonedProbe 先例):
//  1. CallTurnCtx(ctx 取消)除名 pending 之外把 id 登记进 abandoned;
//  2. CallCtx(短控制)放弃**不**登记,迟到应答语义不变(仍走处置口静默丢);
//  3. readLoop 对迟到应答先查 abandoned:命中一次性认领(留痕含 stopReason 摘要)
//     并除名,轮次不复活;
//  4. TTL 过期清理(登记时同步扫描);容量(32)满时淘汰 id 最小(最早发起)。
//
// 【时序纪律(门禁裁定:绝不能睡真实 60s 时钟,722s 事故复盘)】
//   - TTL 是包级 var(client.go):abandonedClient 统一收紧到 50ms、收尾还原 ——
//     过期清扫**零真实等待**驱动(扫描本就是登记时同步做,无 ticker/后台协程);
//   - 取消驱动用**预取消 ctx**(cancel() 先于调用):call() 的 select 里唯一
//     就绪臂就是 ctx.Done(turn 无兜底 timer、无应答帧),确定性返回,无 sleep 协程;
//   - 文件内所有等待一律有界:轮询步长 2-5ms、看门狗 2s —— 任何环境性卡死都在
//     2s 内以带名字的失败收场,不再陪葬整包超时。
// 全部 io.Pipe 直控假连接(client_test.go pipeClient 同纪律,不拉真子进程)。
package hermesapp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// testAbandonedTTL 是本文件全部用例的收紧 TTL:50ms —— 比"自然老化"断言的
// 有界等待(≤TTL+20ms≈70ms)小一档,又远大于调度抖动,毫秒级驱动零歧义。
const testAbandonedTTL = 50 * time.Millisecond

// abandonedWatchdog 是单次有界等待的上限:Cleanup 收尸/迟到应答认领若 2s 内
// 不落场,立即带名字失败(绝不拖满整包超时)。
const abandonedWatchdog = 2 * time.Second

// abandonedClient 造一条 io.Pipe 直控的假连接:cmd 置非零零值以过 call() 的
// "未启动"防御,但绝不拉真子进程;abandoned 集**显式初始化**(测试直接下账,
// 不依赖 giveUpTurn 的懒建,杜绝 nil map 赋值 panic)。stdin 侧挂常驻丢弃读端
// (io.Pipe 同步语义:没人读则写帧阻塞,call() 会卡在写帧而非按期走到放弃分支)。
// TTL 在此统一收紧、Cleanup 统一还原(包内无 t.Parallel,包级 var 改写安全)。
func abandonedClient(t *testing.T) (*Client, *io.PipeWriter) {
	t.Helper()
	oldTTL := abandonedTurnTTL
	abandonedTurnTTL = testAbandonedTTL
	t.Cleanup(func() { abandonedTurnTTL = oldTTL })

	sOutR, sOutW := io.Pipe()
	sInR, sInW := io.Pipe()
	done := make(chan struct{})
	go func() { // 假子进程的 stdin 消费方
		_, _ = io.Copy(io.Discard, sInR)
		close(done)
	}()
	c := &Client{cmd: &exec.Cmd{}, stdout: bufio.NewReader(sOutR), stdin: sInW,
		pending:   map[int64]chan rpcResp{},
		abandoned: map[int64]abandonedTurn{},
		exited:    make(chan struct{})}
	go c.readLoop()
	t.Cleanup(func() {
		_ = sOutW.Close() // 先断数据源让 readLoop 退出,reap 才能收尸
		select {
		case <-c.exited:
		case <-time.After(abandonedWatchdog):
			t.Fatalf("readLoop 未在 %v 内退出", abandonedWatchdog)
		}
		_ = sInW.Close()
		select {
		case <-done:
		case <-time.After(abandonedWatchdog):
			t.Fatalf("stdin 丢弃读端未在 %v 内收场", abandonedWatchdog)
		}
	})
	return c, sOutW
}

// waitAbandonedGone 有界轮询等一个 id 从 abandoned 集除名(非破坏性窥视:
// 绝不能用 takeAbandoned 轮询 —— 那会替 readLoop 抢先把认领做掉)。
func waitAbandonedGone(t *testing.T, c *Client, id int64) {
	t.Helper()
	deadline := time.Now().Add(abandonedWatchdog)
	for {
		c.mu.Lock()
		_, still := c.abandoned[id]
		c.mu.Unlock()
		if !still {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("abandoned 记账 id=%d 未被除名(等待上限 %v)", id, abandonedWatchdog)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestCallTurnCtxAbandonRegistersAbandoned ctx 取消除名之外登记 abandoned。
// 【零等待】预取消 ctx:select 唯一就绪臂就是 ctx.Done(turn 无兜底 timer、
// 无应答帧),确定性走放弃分支,不睡任何真实时钟。
func TestCallTurnCtxAbandonRegistersAbandoned(t *testing.T) {
	c, _ := abandonedClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预取消:放弃路径确定性触发(写帧在 select 之前照常完成)
	_, err := c.CallTurnCtx(ctx, "session/prompt", map[string]any{"sessionId": "fake-sess-1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应随 ctx 取消返回,得 %v", err)
	}
	// giveUpTurn 在 call 返回前完成登记:fresh Client 首调用 id 恒为 1
	if _, ok := c.takeAbandoned(1); !ok {
		t.Fatal("CallTurnCtx 放弃应把 id 登记进 abandoned")
	}
	if _, ok := c.takeAbandoned(1); ok {
		t.Fatal("abandoned 认领应一次性:第二次查询应未命中")
	}
}

// TestCallCtxGiveUpLeavesAbandonedUntouched 短控制调用的放弃语义不变。
func TestCallCtxGiveUpLeavesAbandonedUntouched(t *testing.T) {
	c, _ := abandonedClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预取消,同上零等待
	_, err := c.CallCtx(ctx, "session/list", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应随 ctx 取消返回,得 %v", err)
	}
	if _, ok := c.takeAbandoned(1); ok {
		t.Fatal("CallCtx(短控制)放弃不应登记 abandoned(迟到应答语义保持静默丢)")
	}
}

// TestReadLoopClaimsAbandonedTurnResponse 迟到应答被一次性认领并留痕 stopReason。
func TestReadLoopClaimsAbandonedTurnResponse(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	c, sOut := abandonedClient(t)
	c.giveUpTurn(7, "session/prompt")
	frame := "{\"jsonrpc\":\"2.0\",\"id\":7,\"result\":{\"stopReason\":\"cancelled\"}}\n"
	if _, err := io.WriteString(sOut, frame); err != nil {
		t.Fatalf("喂迟到应答帧失败: %v", err)
	}
	waitAbandonedGone(t, c, 7) // 有界轮询:readLoop 认领即除名
	// 断数据源等 readLoop 退出:日志写入(Close 前)→ close(exited) → 本 goroutine
	// 收到,这条 happens-before 链之后读日志缓冲才无数据竞争(-race 口径)
	_ = sOut.Close()
	select {
	case <-c.exited:
	case <-time.After(abandonedWatchdog):
		t.Fatalf("readLoop 未在 %v 内退出", abandonedWatchdog)
	}
	if !strings.Contains(buf.String(), "stopReason=cancelled") {
		t.Errorf("认领日志应含 stopReason 摘要,得: %s", buf.String())
	}
}

// TestAbandonedTurnTTLSweep 过期记账被登记时的同步扫描清除,零真实等待:
//  (a) 直接下一条"盖章即过期"的账(at 倒扣 2×TTL),下一次登记必扫掉它 ——
//      证明扫描以"存龄 vs 当前 TTL"为准,且随登记同步发生(无后台协程);
//  (b) 先登记一条新账,有界等它**自然老过** 50ms TTL(≤70ms,2ms 步进),
//      再登记触发扫描 —— 证明扫的不是"存在即删",而是真按存龄过期;
//      未过账的 3 号不被误扫(同一次扫描的负向断言)。
func TestAbandonedTurnTTLSweep(t *testing.T) {
	c, _ := abandonedClient(t)
	// (a) 盖章即过期:零等待
	c.mu.Lock()
	c.abandoned[1] = abandonedTurn{method: "session/prompt", at: time.Now().Add(-2 * testAbandonedTTL)}
	c.mu.Unlock()
	c.giveUpTurn(2, "session/prompt") // 登记动作本身触发扫描
	c.mu.Lock()
	_, live := c.abandoned[1]
	c.mu.Unlock()
	if live {
		t.Fatal("过期记账应被 TTL 扫描清除")
	}
	if _, ok := c.takeAbandoned(2); !ok {
		t.Fatal("新登记的记账应健在")
	}
	// (b) 自然老化:有界等 2 号存龄超过 TTL,再登记触发扫描
	// (2 号在 (a) 已被 takeAbandoned 消费,重新登记即全新样本)
	regAt := time.Now()
	c.giveUpTurn(2, "session/prompt") // 重新登记 2 号作为老化样本
	for time.Since(regAt) <= testAbandonedTTL {
		time.Sleep(2 * time.Millisecond) // 有界:最多 ~70ms
	}
	c.giveUpTurn(3, "session/prompt") // 本次登记应扫掉老化的 2 号
	c.mu.Lock()
	_, aged := c.abandoned[2]
	_, fresh := c.abandoned[3]
	c.mu.Unlock()
	if aged {
		t.Fatal("存龄超过 TTL 的记账应被扫描清除")
	}
	if !fresh {
		t.Fatal("未过 TTL 的新记账不应被误扫")
	}
}

// TestAbandonedTurnCapacityEviction 满载登记淘汰 id 最小(最早发起)的记账。
// 纯同步逻辑(登记即淘汰),零等待;容量常量不收紧 —— 32 次登记微秒级。
func TestAbandonedTurnCapacityEviction(t *testing.T) {
	c, _ := abandonedClient(t)
	for i := 1; i <= abandonedTurnCap; i++ {
		c.giveUpTurn(int64(i), "session/prompt")
	}
	c.giveUpTurn(int64(abandonedTurnCap+1), "session/prompt") // 满载再登记:挤掉最旧
	if _, ok := c.takeAbandoned(1); ok {
		t.Fatal("满载应淘汰 id 最小(最早发起)的记账")
	}
	if _, ok := c.takeAbandoned(2); !ok {
		t.Fatal("次早的记账应健在")
	}
	if _, ok := c.takeAbandoned(int64(abandonedTurnCap + 1)); !ok {
		t.Fatal("最新登记的记账应健在")
	}
}

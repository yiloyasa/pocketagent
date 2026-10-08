package eventstore

// eventstore_buffer_test.go —— 2026-10-09 写路径重排的回归钉子。
//
// 【背景】Append 从"每行一次 syscall 直写"收成 32KB 行缓冲;回放从"持 s.mu
// 全量扫盘"改为"锁内快照路径 + 锁外扫描"。这两处各自引入一条必须钉死的语义:
//  1. 回放必须看到全部已 Append 的帧(回放前冲缓冲);
//  2. 轮转前必须冲缓冲(轮出去的代次不能缺尾行,重启 maxSeq 恢复不能回退);
//  3. Append 与回放并发时锁序 mu → scanMu 无死锁、seq 无重复(交给 -race 与
//     并发用例共同覆盖)。

import (
	"sync"
	"testing"
)

func TestBufferedAppendsVisibleToReplay(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	defer s.Close()
	const n = 40 // 远小于 32KB 缓冲:不冲缓冲的话,回放将一无所见
	for i := 0; i < n; i++ {
		s.Append("th-buf", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"x"}`))
	}
	got := s.ReplayTurnFrames("th-buf", 0)
	if len(got) != n {
		t.Fatalf("回放应看到全部已 Append 帧(缓冲必须先冲),got %d want %d", len(got), n)
	}
	for i, e := range got {
		if e.Seq != int64(i+1) {
			t.Fatalf("seq 断档: 第 %d 帧_seq = %d", i, e.Seq)
		}
	}
}

func TestRotationFlushesBufferedTail(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	// 阈值 400B(单行约 144B):前 2 条整体落在 32KB 用户态缓冲里、磁盘未变;
	// 第 3 条触发唯一一次轮转。若轮转前不冲缓冲,这 2 行会随旧句柄关闭而丢失。
	// 【阈值别再调小】单行尺寸 ≈144B,阈值若 < 2 行会退化成"每次追加都轮转",
	// 二次轮转按既有语义覆盖上一代(旧直写实现同样如此),与本用例要钉的
	// "冲刷后轮转"无关。
	s.SetMaxBytes(400)
	for i := 0; i < 3; i++ {
		s.Append("th-rot", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"line"}`))
	}
	_ = s.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重开: %v", err)
	}
	defer s2.Close()
	if s2.MaxSeq() != 3 {
		t.Fatalf("轮转冲刷失败:重启后 MaxSeq = %d, want 3(缓冲尾行丢了)", s2.MaxSeq())
	}
	total := len(s2.ReplayTurnFrames("th-rot", 0))
	if total != 3 {
		t.Fatalf("两代合计应回放 3 帧,got %d", total)
	}
}

// TestAppendAndReplayConcurrent Append(热路径)与回放并发:无死锁、seq 连续、
// 重启后 maxSeq 不回退。跑在 -race 门禁里,同时压 scanMu 的互斥路径。
func TestAppendAndReplayConcurrent(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir)
	const workers, per = 8, 50
	// 【结构说明】回放方不能与 Append 方共用同一个 WaitGroup:回放循环要等
	// replayStop 关闭,而 replayStop 要等 Append 全部结束才关 —— 同组互相等待
	// 是测试自身的死锁。Append 用 wg,回放方用独立的 done 通道收尾。
	var wg sync.WaitGroup
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				s.Append("th-conc", "turn-1", TypePrefixTurn+"text_delta", []byte(`{"delta":"x"}`))
			}
		}()
	}
	// 回放方:与 Append 赛跑,轮次结束前反复触发"冲缓冲 + 扫两代文件"路径。
	replayStop := make(chan struct{})
	replayDone := make(chan struct{})
	go func() {
		defer close(replayDone)
		for {
			select {
			case <-replayStop:
				return
			default:
				_ = s.ReplayTurnFrames("th-conc", 0)
			}
		}
	}()
	wg.Wait()
	close(replayStop)
	<-replayDone

	want := int64(workers * per)
	if s.MaxSeq() != want {
		t.Fatalf("MaxSeq = %d, want %d(seq 必须无重复无断档)", s.MaxSeq(), want)
	}
	// 全量回放(先冲缓冲)必须看到每一帧。
	if got := len(s.ReplayTurnFrames("th-conc", 0)); int64(got) != want {
		t.Fatalf("终态回放帧数 = %d, want %d", got, want)
	}
	_ = s.Close()
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("重开: %v", err)
	}
	defer s2.Close()
	if s2.MaxSeq() != want {
		t.Fatalf("重启后 MaxSeq = %d, want %d", s2.MaxSeq(), want)
	}
}

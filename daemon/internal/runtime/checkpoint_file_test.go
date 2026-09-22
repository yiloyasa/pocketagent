package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// 本文件只测文件版 CheckpointManager（checkpoint.go）的**契约语义**。
// 内存版由 contract_test.go 的契约套件覆盖；这里额外用 ContractSuite 跑一遍文件
// 后端，证明"升级"不是分叉 —— 同一份契约基线换实现依然成立（一行断言都没放宽）。

func fileCheckpointKey(name string) CheckpointKey {
	return CheckpointKey{AgentID: "echo", ThreadID: "th-file", Name: name}
}

// TestFileCheckpointSaveLoadRoundTrip：写入后能逐字节读回，且覆盖写是整体替换。
func TestFileCheckpointSaveLoadRoundTrip(t *testing.T) {
	b := NewFileBackend(t.TempDir())
	cm := b.Checkpoints()
	ctx := context.Background()
	key := fileCheckpointKey("conversation")

	payload := []byte{0x00, 0x01, 0xff, 0xfe, 'x', '\n', 0xe6, 0xb5, 0x8b}
	if err := cm.Save(ctx, key, payload); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	got, found, err := cm.Load(ctx, key)
	if err != nil || !found {
		t.Fatalf("Save 后 Load 应 found=true 且无错，实得 found=%v err=%v", found, err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("内容应逐字节一致：\n want=%x\n got =%x", payload, got)
	}

	// 覆盖写：旧内容必须被整体替换，不得拼出"旧+新"的半截内容。
	if err := cm.Save(ctx, key, []byte("v2")); err != nil {
		t.Fatalf("覆盖 Save 失败: %v", err)
	}
	got2, found2, err2 := cm.Load(ctx, key)
	if err2 != nil || !found2 {
		t.Fatalf("覆盖后 Load 应 found=true 且无错，实得 found=%v err=%v", found2, err2)
	}
	if string(got2) != "v2" {
		t.Fatalf("覆盖写应整体替换，实得 %q", got2)
	}
}

// TestFileCheckpointSurvivesNewBackendOnSameDir：同一 dataDir 上新建 Backend
// 实例能读回 —— 这是"重启后恢复"的最小可执行证明。
func TestFileCheckpointSurvivesNewBackendOnSameDir(t *testing.T) {
	dir := t.TempDir()
	key := fileCheckpointKey("restart")
	payload := []byte(`{"cursor":42}`)
	ctx := context.Background()

	if err := NewFileBackend(dir).Checkpoints().Save(ctx, key, payload); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	// 关键：这是一个**全新的 Backend**，它的进程内状态是空的，只有 dataDir 共享。
	got, found, err := NewFileBackend(dir).Checkpoints().Load(ctx, key)
	if err != nil || !found {
		t.Fatalf("同目录新实例应能读回，实得 found=%v err=%v", found, err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("跨实例读回内容应一致：want=%q got=%q", payload, got)
	}

	// 反证：换一个 dataDir 必须读不到 —— 否则"读回"可能只是某个共享内存的假象。
	if _, wrongFound, wrongErr := NewFileBackend(t.TempDir()).Checkpoints().Load(ctx, key); wrongErr != nil || wrongFound {
		t.Fatalf("不同 dataDir 不该读到 checkpoint（说明落盘路径没被真正使用），实得 found=%v err=%v", wrongFound, wrongErr)
	}
}

// TestFileCheckpointMissingIsNotFoundNotError：文件缺失是明确的"无 checkpoint"，
// 不是错误；对不存在的 key 调 Drop 也必须幂等成功。
func TestFileCheckpointMissingIsNotFoundNotError(t *testing.T) {
	// 目录都还不存在——这条同时覆盖"首次使用前 Load"的路径。
	b := NewFileBackend(t.TempDir())
	cm := b.Checkpoints()
	ctx := context.Background()
	key := fileCheckpointKey("never-written")

	got, found, err := cm.Load(ctx, key)
	if err != nil {
		t.Fatalf("缺文件不得报错，实得 %v", err)
	}
	if found {
		t.Fatal("缺文件时 found 必须为 false，不得假装读到")
	}
	if got != nil {
		t.Fatalf("缺文件时内容应为 nil，实得 %x", got)
	}
	if err := cm.Drop(ctx, key); err != nil {
		t.Fatalf("Drop 不存在的 checkpoint 应幂等成功，实得 %v", err)
	}
	if err := cm.Drop(ctx, key); err != nil {
		t.Fatalf("重复 Drop 仍应幂等成功，实得 %v", err)
	}
	// key 维度不全必须报错（与内存实现同一条语义）。
	if err := cm.Save(ctx, CheckpointKey{AgentID: "echo"}, []byte("x")); !errors.Is(err, ErrCheckpointKeyIncomplete) {
		t.Fatalf("维度不全的 key 应返回 ErrCheckpointKeyIncomplete，实得 %v", err)
	}
}

// TestFileCheckpointCorruptFailsClosedAndPreservesFile：损坏文件必须 fail closed，
// 且**不得**被顺手改写（覆盖 = 把可能可修复的损坏变成永久丢失）。
func TestFileCheckpointCorruptFailsClosedAndPreservesFile(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		corrupt func(orig []byte) []byte
	}{
		{
			name:    "截断成半截",
			corrupt: func(orig []byte) []byte { return orig[:len(orig)/2] },
		},
		{
			name: "正文被改一个字节（摘要不符）",
			corrupt: func(orig []byte) []byte {
				out := append([]byte(nil), orig...)
				out[len(out)-1] ^= 0xff
				return out
			},
		},
		{
			name: "magic 不符（别人的文件）",
			corrupt: func(orig []byte) []byte {
				out := append([]byte(nil), orig...)
				copy(out, "NOTMINE!")
				return out
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			b := NewFileBackend(dir)
			cm := b.Checkpoints()
			key := fileCheckpointKey("corrupt-me")
			if err := cm.Save(ctx, key, []byte(`{"ok":true}`)); err != nil {
				t.Fatalf("Save 失败: %v", err)
			}
			path := onlyCheckpointFile(t, checkpointsDir(dir))
			orig, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读落盘文件失败: %v", err)
			}
			broken := tc.corrupt(orig)
			if err := os.WriteFile(path, broken, 0o600); err != nil {
				t.Fatalf("造损坏文件失败: %v", err)
			}

			got, found, err := cm.Load(ctx, key)
			if err == nil {
				t.Fatal("损坏文件必须报错，不得静默当作'无 checkpoint'或'读到了内容'")
			}
			if !errors.Is(err, ErrCheckpointCorrupt) {
				t.Fatalf("错误应可用 errors.Is 判定为 ErrCheckpointCorrupt，实得 %v", err)
			}
			if found {
				t.Fatal("损坏时 found 必须为 false")
			}
			if got != nil {
				t.Fatalf("损坏时内容应为 nil，实得 %x", got)
			}
			// 关键：fail closed ≠ 顺手清空/重建。
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("损坏文件不该消失: %v", err)
			}
			if !bytes.Equal(after, broken) {
				t.Fatalf("损坏文件被改写了：\n before=%x\n after =%x", broken, after)
			}
		})
	}
}

// TestFileCheckpointAtomicWriteLeavesNoTemp：正常路径下不留 .tmp 残留，
// 且重复写同一 key 只产生一个文件（临时文件是随写随清，不是每写一份留一份）。
func TestFileCheckpointAtomicWriteLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	b := NewFileBackend(dir)
	cm := b.Checkpoints()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := cm.Save(ctx, fileCheckpointKey("stable"), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatalf("第 %d 次 Save 失败: %v", i, err)
		}
		if err := cm.Save(ctx, fileCheckpointKey(fmt.Sprintf("other-%d", i)), []byte("x")); err != nil {
			t.Fatalf("Save 失败: %v", err)
		}
	}
	if err := cm.Drop(ctx, fileCheckpointKey("other-1")); err != nil {
		t.Fatalf("Drop 失败: %v", err)
	}

	entries, err := os.ReadDir(checkpointsDir(dir))
	if err != nil {
		t.Fatalf("读 checkpoint 目录失败: %v", err)
	}
	ckpt, tmp := 0, 0
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e.Name(), ".tmp"):
			tmp++
		case strings.HasSuffix(e.Name(), ".ckpt"):
			ckpt++
		default:
			t.Fatalf("目录里出现意外文件: %s", e.Name())
		}
	}
	if tmp != 0 {
		t.Fatalf("原子写不该留下 .tmp 残留，实得 %d 个", tmp)
	}
	// other-0 / other-2 / stable 三份；other-1 已删；stable 写 3 次仍只占 1 个文件。
	if ckpt != 3 {
		t.Fatalf("应有 3 个 checkpoint 文件（3 次覆盖写 stable 只占 1 个），实得 %d", ckpt)
	}
}

// TestFileCheckpointConcurrentAccessIsRaceFree：不同 key 并发写读互不干扰；
// 同一 key 上并发覆盖写时，读者只能读到"某一次完整写入"的内容，绝不报错或读到半截。
// 本用例在 -race 下运行（见交付验证命令）。
func TestFileCheckpointConcurrentAccessIsRaceFree(t *testing.T) {
	b := NewFileBackend(t.TempDir())
	cm := b.Checkpoints()
	ctx := context.Background()

	var wg sync.WaitGroup
	// 各自独立的 key：证明临时文件唯一 + rename 原子，不会互相覆盖。
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := CheckpointKey{AgentID: "echo", ThreadID: fmt.Sprintf("th-%d", i), Name: "probe"}
			want := []byte(fmt.Sprintf("writer-%d", i))
			for j := 0; j < 20; j++ {
				if err := cm.Save(ctx, key, want); err != nil {
					t.Errorf("goroutine %d Save 失败: %v", i, err)
					return
				}
				got, found, err := cm.Load(ctx, key)
				if err != nil || !found {
					t.Errorf("goroutine %d Load 失败: found=%v err=%v", i, found, err)
					return
				}
				if !bytes.Equal(got, want) {
					t.Errorf("goroutine %d 读回内容不一致: want=%q got=%q", i, want, got)
					return
				}
			}
		}(i)
	}

	// 同一个 key 上并发覆盖写：读者不得报错，且拿到的必须是某一次完整写入。
	shared := fileCheckpointKey("shared")
	if err := cm.Save(ctx, shared, []byte("seed")); err != nil {
		t.Fatalf("seed Save 失败: %v", err)
	}
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if err := cm.Save(ctx, shared, []byte(fmt.Sprintf("w%d-j%d", w, j))); err != nil {
					t.Errorf("writer %d Save 失败: %v", w, err)
					return
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			got, found, err := cm.Load(ctx, shared)
			if err != nil {
				t.Errorf("共享 key 并发读报错: %v", err)
				return
			}
			if !found {
				t.Error("共享 key 已写入，读不到即为丢失")
				return
			}
			if len(got) == 0 {
				t.Error("并发覆盖写期间读到了空内容（半截写入）")
				return
			}
		}
	}()
	wg.Wait()
}

// TestContractSuitePassesOnFileBackend：文件实现必须通过**同一套** ContractSuite。
//
// 【为什么这条最重要】它不是新增断言，而是拿既有契约基线去卡新实现 —— 一行都没
// 放宽。若文件实现只在自定义用例里绿、在契约套件里红，那说明"升级"引入了行为分叉。
func TestContractSuitePassesOnFileBackend(t *testing.T) {
	desc := compliantDescriptor("stub-file")
	b := NewFileBackend(t.TempDir())
	th := &store.Thread{ID: "th-file-suite"}
	rt, err := (&stubProvider{desc: desc, backend: b}).OpenRuntime(context.Background(), th)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	ContractSuite{T: t, AgentID: "stub-file", RT: rt, Backend: b, Thread: th}.RunAll()
}

// onlyCheckpointFile 返回目录里唯一的 .ckpt 文件路径（不唯一就 Fatal）。
func onlyCheckpointFile(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 checkpoint 目录失败(%s): %v", dir, err)
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ckpt") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) != 1 {
		t.Fatalf("应恰好有 1 个 checkpoint 文件，实得 %d: %v", len(files), files)
	}
	return files[0]
}

// thread_section_clear_test.go —— T-17 回归钉子:DeleteThreadSection 的
// section-clear 落库失败必须经 RPC 上报,不再吞错仅日志(原体检 P2-4)。
//
// 【为什么钉这条】分组已在 codex 侧删除后,本地清单清理失败意味着手机上的
// 会话仍挂在已删除的分组下(错乱且重启 daemon 前好不了)。这是有现成错误通道
// 的 RPC 收尾,升级为返回 errInternal + 人话文案;title-sync 等无回传通道的
// 埋点不受影响(它们的口径由 store_observability_test.go 钉住)。
//
// 【离线可测】DeleteThreadSection 前置的 codex 下发(codexProvider 具体类型,
// 无法用桩替换)会真拉子进程,单测不走;故把"清本地清单"抽为
// clearSectionLocalCache 接缝,用真实 FileStore 注入磁盘故障(os.RemoveAll)
// 覆盖失败/成功两条路径 —— 与 store_observability_test.go 同款注入手法。
package server

import (
	"os"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestClearSectionLocalCacheSuccess 健康存储:目标分组的会话全部摘出
// (SectionID 置空),其他分组的会话不受牵连,返回 nil。
func TestClearSectionLocalCacheSuccess(t *testing.T) {
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := &Server{cfg: Config{Store: fs}}
	in1 := &store.Thread{ID: "th-in-1", SectionID: "sec-a"}
	in2 := &store.Thread{ID: "th-in-2", SectionID: "sec-a"}
	other := &store.Thread{ID: "th-other", SectionID: "sec-b"}
	free := &store.Thread{ID: "th-free"} // 本来就没分组
	for _, th := range []*store.Thread{in1, in2, other, free} {
		if err := fs.Create(th); err != nil {
			t.Fatalf("建线程 %s: %v", th.ID, err)
		}
	}

	if err := s.clearSectionLocalCache("sec-a"); err != nil {
		t.Fatalf("健康存储不应报错: %v", err)
	}
	for _, id := range []string{"th-in-1", "th-in-2"} {
		if got, _ := fs.Get(id); got.SectionID != "" {
			t.Fatalf("%s 应已摘出分组,得到 %q", id, got.SectionID)
		}
	}
	if got, _ := fs.Get("th-other"); got.SectionID != "sec-b" {
		t.Fatalf("其他分组的会话不得受牵连,得到 %q", got.SectionID)
	}
	if got, _ := fs.Get("th-free"); got.SectionID != "" {
		t.Fatalf("未分组会话应保持未分组,得到 %q", got.SectionID)
	}
}

// TestClearSectionLocalCacheFailureReturnsHumanInternalError 落库失败
// (注入真实磁盘故障):返回 codes.Internal + 人话文案;底层原因只进日志
// (op=section-clear 埋点),不透给 UI。
func TestClearSectionLocalCacheFailureReturnsHumanInternalError(t *testing.T) {
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := &Server{cfg: Config{Store: fs}}
	th := &store.Thread{ID: "th-stuck", SectionID: "sec-gone"}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("注入磁盘故障失败: %v", err)
	}

	buf := captureStdlog(t)

	err = s.clearSectionLocalCache("sec-gone")
	if err == nil {
		t.Fatal("落库失败必须返回错误,不得吞掉")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("应为 codes.Internal,得到 %v", status.Code(err))
	}
	for _, want := range []string{"删除分组成功", "会话清单同步失败", "请重试或重启 daemon"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("文案缺人话片段 %q,得到 %q", want, err.Error())
		}
	}
	if strings.Contains(err.Error(), "op=section-clear") || strings.Contains(err.Error(), dir) {
		t.Fatalf("底层原因不得透给 UI(只进日志),得到 %q", err.Error())
	}
	// 排障埋点仍在:失败那条留 op=section-clear + thread ID。
	out := buf.String()
	if !strings.Contains(out, "op=section-clear") || !strings.Contains(out, "thread=th-stuck") {
		t.Fatalf("失败应留埋点日志,得到:\n%s", out)
	}
}

// TestDeleteThreadSectionUnavailableWithoutCodex 既有闸门不变:没有 codex
// 时仍报 Unavailable"电脑上没有可用的 codex"(升级只动 section-clear 一处)。
func TestDeleteThreadSectionUnavailableWithoutCodex(t *testing.T) {
	srv := New(Config{Store: mustStore(t), Providers: map[string]agent.Provider{}})
	_, err := srv.DeleteThreadSection(ctxBG(), &pb.DeleteThreadSectionRequest{SectionId: "sec-x"})
	if err == nil || status.Code(err) != codes.Unavailable || !strings.Contains(err.Error(), "电脑上没有可用的 codex") {
		t.Fatalf("无 codex 应 Unavailable + 人话,得到 %v", err)
	}
}

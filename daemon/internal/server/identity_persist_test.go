// identity_persist_test.go —— W5 身份持久化：daemon 重启后主机身份必须不变。
//
// 【为什么这是硬要求】host_id 出现在 GetHostInfo 与 mDNS 宣告里，是手机判断
// 「还是同一台电脑」的依据。进程启动时随机生成，等于每次重启手机都要重新认识
// 这台机器 —— 配对与信任链在用户不知情的情况下被打断（规格 §3 待改造项、
// §4.1 明确要求 installation_id 与 host_id 都必须持久化）。
package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/store"
)

func newServerWithDataDir(t *testing.T, dataDir string) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := New(Config{
		Store:     fs,
		Providers: map[string]agent.Provider{"echo": agent.NewEcho()},
		Workspace: t.TempDir(),
		DataDir:   dataDir,
	})
	// eventstore 会持有 <data>/events.jsonl 的句柄；不关它，t.TempDir() 的清理
	// 会在 Windows 上报「file is being used by another process」（与扇出 flaky
	// 同族：后台资源未释放 + 目录清理相撞）。t.Cleanup 是 LIFO，故关闭先执行。
	t.Cleanup(func() {
		if s.events != nil {
			_ = s.events.Close()
		}
	})
	return s
}

// TestHostIDStableAcrossRestarts：同一数据目录下重启两次，host id 必须相同。
// （RED：修复前两次各自 randomID("host")，必然不等。）
func TestHostIDStableAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	first := newServerWithDataDir(t, dir).HostID()
	if first == "" {
		t.Fatal("host id 不应为空")
	}
	second := newServerWithDataDir(t, dir).HostID()
	if first != second {
		t.Fatalf("重启后 host id 变了: %q → %q（手机会认为换了机器）", first, second)
	}
}

// TestInstallationFileWrittenAndReused 钉住落盘事实与复用：
// installation.json 真的写了，且第二次启动读回同一 installation_id / host_id。
func TestInstallationFileWrittenAndReused(t *testing.T) {
	dir := t.TempDir()
	srv := newServerWithDataDir(t, dir)
	path := filepath.Join(dir, "installation.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("installation.json 未落盘: %v", err)
	}
	var got struct {
		InstallationID string `json:"installation_id"`
		HostID         string `json:"host_id"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("installation.json 不是合法 JSON: %v\n%s", err, raw)
	}
	if got.InstallationID == "" {
		t.Fatal("installation_id 必须被持久化（规格 §4.1）")
	}
	if got.HostID != srv.HostID() {
		t.Fatalf("文件里的 host_id(%q) 与运行期 HostID()(%q) 不一致", got.HostID, srv.HostID())
	}

	// 第二次启动：读回同一份，不得重写身份。
	second := newServerWithDataDir(t, dir)
	raw2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("二次启动后 installation.json 丢失: %v", err)
	}
	var got2 struct {
		InstallationID string `json:"installation_id"`
		HostID         string `json:"host_id"`
	}
	if err := json.Unmarshal(raw2, &got2); err != nil {
		t.Fatalf("二次启动后 JSON 损坏: %v", err)
	}
	if got2.InstallationID != got.InstallationID || got2.HostID != got.HostID {
		t.Fatalf("二次启动改写了身份: %+v → %+v", got, got2)
	}
	if second.HostID() != got.HostID {
		t.Fatalf("二次启动的 HostID 与文件不符: %q ≠ %q", second.HostID(), got.HostID)
	}
}

// TestInstallationIDIsSeparateFromHostID 钉住两个 id 不是同一个值：
// installation 代表「一次安装」，host 代表「这台主机」——规格 §4.1 要求分开，
// 因为卸载/重置配对/升级三种行为要能分别定义。
func TestInstallationIDIsSeparateFromHostID(t *testing.T) {
	dir := t.TempDir()
	newServerWithDataDir(t, dir)
	raw, err := os.ReadFile(filepath.Join(dir, "installation.json"))
	if err != nil {
		t.Fatalf("installation.json 未落盘: %v", err)
	}
	var got struct {
		InstallationID string `json:"installation_id"`
		HostID         string `json:"host_id"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if got.InstallationID == got.HostID {
		t.Fatalf("installation_id 与 host_id 不应是同一个值: %q", got.InstallationID)
	}
}

// TestHostIDWithoutDataDirStaysRandom 钉住行为零分叉：无数据目录（测试/内存模式）
// 时不做持久化，仍给随机身份 —— 既有大量测试依赖它，且不该因本波改动而变红。
func TestHostIDWithoutDataDirStaysRandom(t *testing.T) {
	a := newServerWithDataDir(t, "").HostID()
	b := newServerWithDataDir(t, "").HostID()
	if a == "" || b == "" {
		t.Fatal("内存模式也应给非空 host id")
	}
	if a == b {
		t.Fatalf("内存模式两次 New 应各自随机（否则测试间会互相污染）: %q", a)
	}
}

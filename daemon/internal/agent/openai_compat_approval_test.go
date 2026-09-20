package agent

// openai_compat_approval_test.go —— P0-1 回归钉子:审批通道缺失(approve==nil,
// 如 TestModel 诊断 RPC 路径 server/model.go 传 nil)时,危险工具(write_file /
// run_command,Dangerous:true)必须拒绝执行,绝不允许绕过审批直接跑;
// 非危险工具不受影响,照常执行(钉死"不破坏正常功能")。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func oaNoopSink(*pb.TurnServerMessage) {}

// TestExecWithApprovalNilBlocksWriteFile approve=nil + write_file:不执行,回人话错误。
// 真实临时目录探针:若被执行必然落盘,断言文件不存在 = 真没执行。
func TestExecWithApprovalNilBlocksWriteFile(t *testing.T) {
	ws := t.TempDir()
	tools := StdTools()
	call := oaToolCall{ID: "call-1", Type: "function"}
	call.Function.Name = "write_file"
	call.Function.Arguments = `{"path":"probe.txt","content":"should never be written"}`
	got := execWithApproval(context.Background(), tools, call, ws, oaNoopSink, nil)
	if !strings.Contains(got, "审批通道不可用") {
		t.Fatalf("审批通道缺失时危险工具应拒绝执行,得到 %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws, "probe.txt")); !os.IsNotExist(err) {
		t.Fatal("write_file 不该被执行:探针文件被写出了(审批被绕过)")
	}
}

// TestExecWithApprovalNilBlocksRunCommand 同上,run_command 用输出探针:
// 若被执行,echo 的标记串必然出现在结果里。
func TestExecWithApprovalNilBlocksRunCommand(t *testing.T) {
	ws := t.TempDir()
	tools := StdTools()
	call := oaToolCall{ID: "call-2", Type: "function"}
	call.Function.Name = "run_command"
	call.Function.Arguments = `{"command":"echo T08A-PROBE-SHOULD-NOT-RUN"}`
	got := execWithApproval(context.Background(), tools, call, ws, oaNoopSink, nil)
	if !strings.Contains(got, "审批通道不可用") {
		t.Fatalf("审批通道缺失时危险工具应拒绝执行,得到 %q", got)
	}
	if strings.Contains(got, "T08A-PROBE-SHOULD-NOT-RUN") {
		t.Fatal("run_command 不该被执行:命令输出出现在结果里(审批被绕过)")
	}
}

// TestExecWithApprovalNilAllowsSafeTool approve=nil + 非危险工具:照常执行,
// 钉死"不破坏正常功能"(TestModel 诊断轮的读文件/列目录仍然能用)。
func TestExecWithApprovalNilAllowsSafeTool(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "hello.txt"), []byte("ok-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := StdTools()
	call := oaToolCall{ID: "call-3", Type: "function"}
	call.Function.Name = "read_file"
	call.Function.Arguments = `{"path":"hello.txt"}`
	got := execWithApproval(context.Background(), tools, call, ws, oaNoopSink, nil)
	if !strings.Contains(got, "ok-content") {
		t.Fatalf("非危险工具在 approve=nil 时应照常执行,得到 %q", got)
	}
}

// TestExecWithApprovalHookStillWorks approve!=nil 的既有语义不被本次改动影响:
// 用户拒绝 → 结果文本带"用户拒绝",工具不执行;同意 → 工具真的执行。
func TestExecWithApprovalHookStillWorks(t *testing.T) {
	ws := t.TempDir()
	tools := StdTools()

	denyCall := oaToolCall{ID: "call-4", Type: "function"}
	denyCall.Function.Name = "write_file"
	denyCall.Function.Arguments = `{"path":"deny.txt","content":"x"}`
	got := execWithApproval(context.Background(), tools, denyCall, ws, oaNoopSink,
		func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return false, "太危险", nil
		})
	if !strings.Contains(got, "用户拒绝") {
		t.Fatalf("approve 拒绝路径应保持既有语义,得到 %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws, "deny.txt")); !os.IsNotExist(err) {
		t.Fatal("用户拒绝后 write_file 不该执行")
	}

	allowCall := oaToolCall{ID: "call-5", Type: "function"}
	allowCall.Function.Name = "write_file"
	allowCall.Function.Arguments = `{"path":"allow.txt","content":"x"}`
	got = execWithApproval(context.Background(), tools, allowCall, ws, oaNoopSink,
		func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
			return true, "", nil
		})
	if !strings.Contains(got, "已写入") {
		t.Fatalf("approve 同意路径应真的执行工具,得到 %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws, "allow.txt")); err != nil {
		t.Fatalf("approve 同意后 write_file 应落盘: %v", err)
	}
}

package piapp

// hidewin_assert_test.go —— P1-3 回归钉子:Version 的构造路径必须走
// hidewin.Apply,不许出现裸 exec 链式调用。
//
// 【为什么是 AST 断言而不是行为测试】hidewin 的效果(CREATE_NO_WINDOW 是否
// 生效、桌面有没有闪黑窗)在单测进程里无法观测——测试进程自己就带控制台,
// 子窗口可见性取决于父进程形态。能钉住的是"构造形状":cmd 落变量 → Apply →
// 再 Output,与 Start(生产同文件)及 codexapp/client.go:118 的写法一致。
// 断言对象是源码结构,不是字符串 grep(对格式化/注释不敏感),任何把
// exec.CommandContext(...).Output() 链式写法改回来的提交都会在这里失败。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestVersionPathHidesWindow 两条断言:
//  1. Version 函数体内必须出现 hidewin.Apply 调用;
//  2. client.go 全文件不得出现 exec.Command*(...).Run/Output/Start 等
//     链式写法 —— cmd 被埋进表达式就没有 Apply 的安放点,一旦出现即回归
//     (P1-3:全核心链路唯一漏加 hidewin 的正是这种形状)。
func TestVersionPathHidesWindow(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "client.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 client.go 失败: %v", err)
	}

	var versionFn *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "Version" {
			versionFn = fn
			return false
		}
		return true
	})
	if versionFn == nil {
		t.Fatal("找不到 Version 函数(被改名/移走时请同步更新本断言)")
	}

	applied := false
	ast.Inspect(versionFn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "hidewin" && sel.Sel.Name == "Apply" {
				applied = true
			}
		}
		return true
	})
	if !applied {
		t.Fatal("Version 必须对 exec 出的 cmd 调 hidewin.Apply(P1-3:GetHostInfo 周期探测会在用户桌面闪黑窗)")
	}

	// 裸 exec 链式调用:exec.Command*(...).X(X ∈ Run/Output/...),cmd 未落变量。
	bareSuffixes := []string{
		"Run", "RunContext", "Output", "OutputContext", "CombinedOutput",
		"Start", "StartContext", "Wait",
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		isExecCmd := false
		if isel, ok := inner.Fun.(*ast.SelectorExpr); ok {
			if id, ok := isel.X.(*ast.Ident); ok && id.Name == "exec" &&
				(isel.Sel.Name == "Command" || isel.Sel.Name == "CommandContext") {
				isExecCmd = true
			}
		}
		if !isExecCmd {
			return true
		}
		for _, name := range bareSuffixes {
			if sel.Sel.Name == name {
				t.Fatalf("client.go 出现裸 exec 链式调用 exec.Command*().%s(cmd 无法 hidewin.Apply,会在用户桌面闪黑窗)", name)
			}
		}
		return true
	})
}

package codexapp

// client_debug_ast_test.go —— T-24 回归钉子:readLoop 的 PA_CODEX_DEBUG 调试
// 帧必须走 log 包落 daemon.log,不得直写 os.Stderr。
//
// 【为什么是 AST 断言而不是行为测试】(仿 piapp hidewin_assert_test 思路)
// 缺陷场景本身无法在单测进程里复现:生产 daemon 是 windowsgui 后台进程,
// 无控制台,os.Stderr 无人接收 —— 直写的调试帧"在本机跑测试时看得见、
// 上生产全丢"。而 readLoop 依赖真实 codex 子进程的 stdout 管道,难以直接
// 单测。能钉住的是"构造形状":调试帧一律经 log 输出;任何把 os.Stderr 直写
// 改回来的提交都会在这里失败。断言对象是源码结构,不是字符串 grep
// (对格式化/注释不敏感)。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestReadLoopDebugOutputGoesToLog 两条断言:
//  1. readLoop 函数体内不得出现 os.Stderr 直写(后台 daemon 无控制台,直写
//     = PA_CODEX_DEBUG 开了也一个调试帧都收不到);
//  2. readLoop 必须保留 PA_CODEX_DEBUG env 开关(调试输出仅在开关非空时
//     产生 —— 本回归防的是"输出通道"回退,不许顺手把开关语义也弄丢)。
func TestReadLoopDebugOutputGoesToLog(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "client.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 client.go 失败: %v", err)
	}

	var readLoop *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "readLoop" {
			readLoop = fn
			return false
		}
		return true
	})
	if readLoop == nil {
		t.Fatal("找不到 readLoop 函数(被改名/移走时请同步更新本断言)")
	}

	debugGate := false
	ast.Inspect(readLoop, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" && sel.Sel.Name == "Stderr" {
				t.Errorf("readLoop 出现 os.Stderr 直写(%s):后台 daemon 无控制台,调试帧必须走 log 包(T-24)", fset.Position(n.Pos()))
			}
		}
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"PA_CODEX_DEBUG"` {
			debugGate = true
		}
		return true
	})
	if !debugGate {
		t.Error("readLoop 丢失 PA_CODEX_DEBUG env 开关:调试输出应仅在开关非空时产生,不许静默删除或常开")
	}
}

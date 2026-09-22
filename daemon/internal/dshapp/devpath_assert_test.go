// devpath_assert_test.go —— 发布边界回归钉子：本包不得把开发机专有路径写进
// 会被编译进二进制的位置。
//
// 【为什么需要它】Go 的 -trimpath 只剥离源码路径、不剥离字符串常量：写在常量里的
// C:\Users\<开发机用户名>\... 与厂商自选安装目录会原样进 pocketagentd.exe，
// 随发布包分发出去（规格 §2 明令发布包不得包含开发者电脑路径）。
//
// 这类缺陷**行为测试抓不到** —— 在开发机上那些路径确实存在、功能照常工作，
// 只有换一台机器或拆包看字节才会现形。所以这里直接审源码。
//
// 【判据走 AST 而不是文本】只有字符串字面量会随二进制分发，注释会被编译器丢弃
// （README §12 坑 1 同款事实）。文本匹配会把解释性注释判成泄漏 —— 那些注释恰恰
// 在说明"旧实现曾写死某台开发机的目录"，删掉它们只会让后人不明就里。
//
// 与 piapp/devpath_assert_test.go 同款：两包各自审自己的源码（要被审的文件在
// 两个不同的包里），关键字表与 AST 扫描器则共用 hostenv/goscan，避免两处漂移。
package dshapp

import (
	"os"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/hostenv/goscan"
)

// TestSourceHasNoDeveloperPaths 扫描本包全部**非测试** Go 源码的字符串字面量，
// 断言不含开发机专有痕迹。
func TestSourceHasNoDeveloperPaths(t *testing.T) {
	files, err := goscan.PackageGoFiles(".")
	if err != nil {
		t.Fatalf("枚举包内 Go 文件失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("未扫到任何源文件 —— 闸门静默空转就是假绿")
	}
	var offenders []string
	for _, f := range files {
		hits, err := goscan.DeveloperTracesInFile(f)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", f, err)
		}
		if len(hits) > 0 {
			offenders = append(offenders, f+" → "+strings.Join(hits, ", "))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("源码字符串常量含开发机专有路径（会被编译进发布包，规格 §2 禁止）:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestLocateDshReturnsOnlyExistingPaths：未设 DSH_BIN / DSH_NODE 时，LocateDsh
// 只能返回空串或**本机真实存在**的路径。
//
// 【为什么不在这里断言"不含痕迹"】返回值可能由 %LOCALAPPDATA% 派生，在开发机上
// 展开后自然含当前用户名 —— 那是正确的派生结果，不是泄漏。「是否写死了开发机
// 路径」由上面的 AST 检查判定；这里守的是：绝不返回本机不存在的路径（旧实现会
// 把写死的路径当默认值，让"未找到 dsh"的报错念出开发机目录结构）。
func TestLocateDshReturnsOnlyExistingPaths(t *testing.T) {
	t.Setenv("DSH_BIN", "")
	t.Setenv("DSH_NODE", "")
	node, bin, _ := LocateDsh()
	for _, p := range []string{node, bin} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("LocateDsh 返回了本机不存在的路径 %q（把写死的路径当默认值）", p)
		}
	}
}

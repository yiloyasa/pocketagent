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
// （README §12 坑 1 同款事实）。用文本匹配会把解释性注释判成泄漏 —— 那些注释
// 恰恰在说明"旧实现曾写死某台开发机的目录"，把它们删掉只会让后人不明就里。
//
// 【先例】包内 hidewin_assert_test.go 已用 AST 断言钉住「构造路径不得出现裸 exec」，
// 本条同款思路：把纪律固化成会失败的测试，而不是写在注释里的君子协定。
package piapp

import (
	"os"
	"testing"

	"github.com/pocketagent/daemon/internal/testkit"
)

// TestSourceHasNoDeveloperPaths 扫描本包全部**非测试** Go 源码的字符串字面量,
// 断言不含开发机专属路径(实现收编到 testkit,与 piapp/dshapp 同款一份)。
func TestSourceHasNoDeveloperPaths(t *testing.T) {
	testkit.AssertNoDeveloperPaths(t)
}

// TestLocateCLIReportsMissingInsteadOfGuessing：未设 PI_CLI 时，LocateCLI 只能
// 返回空串或**本机真实存在**的路径。
//
// 【为什么不在这里断言"不含痕迹"】返回值由 %LOCALAPPDATA% / %APPDATA% 派生，
// 在开发机上展开后必然含当前用户名 —— 那是正确的派生结果，不是泄漏。
// 「是否写死了开发机路径」由上面的 AST 检查判定；这里守的是另一条不变量：
// 绝不把一条本机不存在的路径当默认值返回（旧实现正是**无条件**返回写死路径、
// 连 Stat 都不做，于是"未找到 pi"的报错会念出开发机的目录结构）。
func TestLocateCLIReportsMissingInsteadOfGuessing(t *testing.T) {
	t.Setenv("PI_CLI", "")
	got := LocateCLI()
	if got == "" {
		return
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("LocateCLI 返回了本机不存在的路径 %q（无条件返回默认值的旧行为）", got)
	}
}

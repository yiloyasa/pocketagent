// cmd_devpath_test.go —— daemon/cmd 探针层的发布边界回归钉子:cmd 下全部探针
// 的**字符串字面量**里不得出现开发机专有路径。
//
// 【为什么钉 cmd 层】internal/piapp、internal/dshapp 各自的 devpath_assert_test.go
// 只审自己包内的源码;daemon/cmd 下几十个探针是彼此独立的 main 包,先例闸门够不着
// —— 而探针层恰恰是历史上硬编码个人路径的重灾区(定位 node/cli/数据目录/报告落点
// 时图省事写死开发机绝对路径)。Go 的 -trimpath 只剥源码路径、不剥字符串常量,
// 这些常量会随各探针自己的二进制分发(规格 §2 明令禁止)。
//
// 【为什么审字符串字面量而不是文本】与 dshapp/devpath_assert_test.go 同款判据:
// 只有字符串字面量会进二进制,注释会被编译器丢弃 —— 注释里"旧实现曾写死某开发机
// 路径"的解释性说明(以及 herm*probe 注释里的红线说明)不应判成泄漏。扫描器与
// 关键字表共用本包 + hostenv,单一实现,防两处漂移。
//
// 【路径推导(为什么是 ../../../cmd)】go test 的工作目录是**被测包的源码目录**,
// 即本文件所在目录:
//
//	<repo>/daemon/internal/hostenv/goscan
//
// 目标目录 daemon/cmd 相对它:
//
//	goscan --(..)--> hostenv          第 1 层
//	hostenv --(..)--> internal        第 2 层
//	internal --(..)--> daemon         第 3 层
//	daemon 下的 cmd 子目录            → "../../../cmd"
//
// 故相对路径为 "../../../cmd"(3 个 ".." + cmd)。若目录层级变动,枚举会直接
// 报错或扫到 0 个文件,下面的空转防御会立刻红,不会静默假绿。
package goscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdProbesHaveNoDeveloperPaths 遍历 daemon/cmd 的每个探针包目录,
// 逐个非测试 Go 文件跑 AST 字符串字面量扫描,断言零开发机痕迹。
func TestCmdProbesHaveNoDeveloperPaths(t *testing.T) {
	cmdRoot := filepath.Join("..", "..", "..", "cmd")
	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		t.Fatalf("枚举 %s 失败(go test 工作目录应为 goscan 包目录): %v", cmdRoot, err)
	}
	var offenders []string
	scanned := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(cmdRoot, e.Name())
		files, err := PackageGoFiles(dir) // 只取非测试 .go(与发布二进制对齐)
		if err != nil {
			t.Fatalf("枚举 %s 失败: %v", dir, err)
		}
		for _, f := range files {
			hits, err := DeveloperTracesInFile(f)
			if err != nil {
				t.Fatalf("解析 %s 失败: %v", f, err)
			}
			scanned++
			if len(hits) > 0 {
				rel, _ := filepath.Rel(cmdRoot, f)
				offenders = append(offenders, rel+" → "+strings.Join(hits, ", "))
			}
		}
	}
	if scanned == 0 {
		t.Fatal("未扫到任何源文件 —— 闸门静默空转就是假绿(检查 ../../../cmd 相对路径推导)")
	}
	if len(offenders) > 0 {
		t.Fatalf("daemon/cmd 探针源码字符串常量含开发机专有路径(会随探针二进制分发,规格 §2 禁止):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

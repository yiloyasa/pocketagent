package testkit

import (
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/hostenv/goscan"
)

// AssertNoDeveloperPaths 扫描**本包**(测试进程 cwd = 被测包源码目录)全部非测试
// Go 源码的字符串字面量,断言不含开发机专属路径。
//
// 【为什么必须是本包 cwd】包级回归抓的是"有人把已确认的专属路径写进字符串";每个
// 包的测试二进制由 go test 在自己的包目录下启动,所以这里传 "." 就等价于原先写在
// 各包 devpath_assert_test.go 里的那一份(语句与文案逐字同款,仅多一层 t.Helper()
// 让失败行号指回调用点)。
//
// 【为什么要 AST 而不是文本】文本匹配会被注释/字符串拼接骗过;goscan 是 AST 扫描
// (与 hostenv/goscan 同源),语义不变。命中即 Fatalf —— 这是 AUDIT §2 的硬门,不弱化。
func AssertNoDeveloperPaths(t *testing.T) {
	t.Helper()
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

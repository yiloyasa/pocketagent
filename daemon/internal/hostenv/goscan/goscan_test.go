package goscan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDeveloperTracesInFilePositiveAndNegative 是闸门的对照实验。
//
// 【为什么闸门自己也要测】一条永远返回"无命中"的扫描器看起来和"清洁的代码库"
// 一模一样 —— 这正是本项目定义的静音失效（假绿）。所以这里同时给正例与反例：
// 反例（注释里的痕迹）必须不报，正例（字符串字面量里的痕迹）必须报。
// 只测其中一边，都测不出闸门是活的还是死的。
func TestDeveloperTracesInFilePositiveAndNegative(t *testing.T) {
	dir := t.TempDir()

	// 反例：痕迹只出现在注释与标识符里 —— Go 编译器丢弃注释，不进二进制。
	commentOnly := filepath.Join(dir, "comment_only.go")
	commentSrc := "package a\n\n" +
		"// 旧实现曾写死 C:\\Users\\yanzi\\node.exe 与某厂商的自选目录，已改为环境派生。\n" +
		"const Answer = \"ok\"\n"
	if err := os.WriteFile(commentOnly, []byte(commentSrc), 0o644); err != nil {
		t.Fatalf("写反例文件失败: %v", err)
	}
	hits, err := DeveloperTracesInFile(commentOnly)
	if err != nil {
		t.Fatalf("解析反例失败: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("注释里的说明性文字不该算命中（否则纪律会逼着注释写含糊）: %v", hits)
	}

	// 正例：痕迹出现在字符串常量里 —— 这才是会随二进制分发的东西。
	withLiteral := filepath.Join(dir, "with_literal.go")
	literalSrc := "package b\n\n" +
		"const Node = `C:\\Users\\yanzi\\node.exe`\n"
	if err := os.WriteFile(withLiteral, []byte(literalSrc), 0o644); err != nil {
		t.Fatalf("写正例文件失败: %v", err)
	}
	hits, err = DeveloperTracesInFile(withLiteral)
	if err != nil {
		t.Fatalf("解析正例失败: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("字符串字面量里的开发机路径必须被检出 —— 闸门失灵（假绿）")
	}
}

// TestPackageGoFilesSkipsTests 钉住"只审非测试文件"这条口径：
// 测试文件里的绝对路径不会进发布二进制，若也扫，钉子会在开发机上永远红。
func TestPackageGoFilesSkipsTests(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"real.go", "real_test.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package c\n"), 0o644); err != nil {
			t.Fatalf("造文件失败: %v", err)
		}
	}
	files, err := PackageGoFiles(dir)
	if err != nil {
		t.Fatalf("枚举失败: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "real.go" {
		t.Fatalf("应只返回非测试文件，实得 %v", files)
	}
}

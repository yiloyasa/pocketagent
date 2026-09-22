// Package goscan 用 AST 提取 Go 源码里「会进二进制」的字符串字面量，
// 供发布边界的源码级回归钉子使用（注释与标识符一律不算）。
//
// 【为什么必须走 AST 而不是文本匹配】Go 编译器会丢弃注释：注释里出现
// "Claude Code"、"yanzi" 这类词往往是解释性的（正是本项目要求的"注释写为什么"），
// 它们不会进二进制。朴素的文本扫描会把这类说明判成泄漏，逼着后来的人把注释
// 写含糊 —— 那是拿纪律换噪音，而且判据与物理事实不符。
//
// 判据必须与物理事实对齐：**只有字符串字面量随二进制分发**。反引号原始串
// （本项目写绝对路径的常用形式）与双引号解释串都要覆盖 —— 只扫其中一种，
// 换个写法就能绕过去。
//
// 【为什么单独一个包】本包依赖 go/parser，只被 _test.go 引用；放在被生产代码
// 引用的 hostenv 里会把整个 go/parser 拖进 daemon 二进制。分包后链接器直接丢弃它。
package goscan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pocketagent/daemon/internal/hostenv"
)

// StringLiterals 返回文件里全部字符串字面量的解码后内容（去引号、解转义）。
func StringLiterals(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			// 原始串理论上都能 Unquote；容错时退化为去引号，避免因一条
			// 解析不了的字面量整文件跳过（跳过 = 漏检）。
			s = strings.Trim(lit.Value, "`\"")
		}
		out = append(out, s)
		return true
	})
	return out, nil
}

// DeveloperTracesInFile 报告文件里「会进二进制」的开发机痕迹（去重）。
func DeveloperTracesInFile(path string) ([]string, error) {
	lits, err := StringLiterals(path)
	if err != nil {
		return nil, err
	}
	var hits []string
	seen := make(map[string]bool)
	for _, s := range lits {
		for _, h := range hostenv.DeveloperTracesIn([]byte(s)) {
			if !seen[h] {
				seen[h] = true
				hits = append(hits, h)
			}
		}
	}
	return hits, nil
}

// PackageGoFiles 返回目录下**非测试**的 .go 文件（跳过 _test.go 与子目录）。
//
// 非测试文件才是会被编译进发布二进制的部分；测试文件里的绝对路径不进 exe，
// 若也扫，这条钉子在开发机上会永远红。
func PackageGoFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out, nil
}

package hidewin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestProductionCodeExecutesAreHidden 是「电脑端绝不弹前台窗口」的守护测试。
//
// 【为什么做成静态扫描而不是单测某个点】窗口弹现是"漏掉一个调用点就复发"
// 的回归:历史实锤是 GetHostInfo 探活每 8 秒闪一次黑窗。逐点写测试永远追不上
// 新增的 exec.Command,所以这里钉的是**全仓纪律**——生产链(internal/ 与
// paguard)里每个 exec.Command 所在文件必须出现 hidewin 调用。
//
// 【豁免范围】cmd/ 下的探针工具(*probe*)是开发机手动运行的诊断工具,不在
// daemon 生产链上;term/manager.go 的 ConPTY 会话走 CreationFlags 直传
// (go-pty 不读 HideWindow 字段),不引 hidewin 包,单独豁免并注释。
func TestProductionCodeExecutesAreHidden(t *testing.T) {
	root := "../.."
	execRe := regexp.MustCompile(`exec\.Command(Context)?\(`)
	hideRe := regexp.MustCompile(`hidewin\.Apply|hidewin\.Enabled|CREATE_NO_WINDOW`)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "gen" || name == ".git" || strings.Contains(path, "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(path)
		// 生产链 = internal/** 与 cmd/paguard(看护进程)。探针工具豁免。
		if !strings.HasPrefix(rel, "internal/") && rel != "cmd/paguard/main.go" {
			return nil
		}
		if strings.Contains(rel, "term/") {
			// ConPTY 会话:见 manager.go 内注释,走 CreationFlags 直传豁免。
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if execRe.Match(src) && !hideRe.Match(src) {
			t.Errorf("%s: 生产代码调用了 exec.Command 但未见 hidewin 调用 —— 控制台子进程会在用户桌面弹前台窗口(历史实锤:探活每 8 秒闪黑窗)。用 hidewin.Apply(cmd) 隐藏;若属于 term/ 之外的正当豁免,先在这里登记理由再改。", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
}

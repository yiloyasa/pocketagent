// cmd/pimodelsprobe —— pi models-store.json 模型回退候选实证探针(零子进程、
// 零网络、只读)。
//
// 【目的(2026-10-01,G1 修复实证)】pi 0.99.1 的 get_available_models 走 8s
// 快速失败后,模型列表回退到 ~/.pi/agent/models-store.json。本探针对**真实文件**
// 并排跑两条管线,验证回退候选质量:
//
//	prod   —— 生产路径 agent.PiModelsStoreFallbackModels()(桶感知提取,G1 后)
//	legacy —— 修复前路径 agency.Compile(StoreFileSource 整树提取,provider 继承
//	          对「models 数组」形状失效,候选 Provider 全空 → ModelID 拼成 "/<id>")
//
// 期望:prod 出全量候选且 Provider/ID/DisplayName/ContextWindow/Thinking/Images
// 完整;legacy 在 0.99.1 真实形状下暴露 provider 缺失(留作回归对照)。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pocketagent/daemon/internal/agency"
	"github.com/pocketagent/daemon/internal/agent"
)

func main() {
	path := flag.String("path", "", "models-store.json 路径(默认 ~/.pi/agent/models-store.json)")
	flag.Parse()
	if *path == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			fmt.Println("无法定位用户目录:", err)
			os.Exit(1)
		}
		*path = filepath.Join(home, ".pi", "agent", "models-store.json")
	}
	fmt.Println("== pi models-store 回退候选实证 ==")
	fmt.Println("文件:", *path)
	if _, err := os.Stat(*path); err != nil {
		fmt.Println("文件不可读:", err)
		os.Exit(1)
	}

	// ---- 生产路径(桶感知)----
	fmt.Println("\n[prod] agent.PiModelsStoreFallbackModels()(桶感知提取,G1 后生产路径)")
	mis := agent.PiModelsStoreFallbackModels()
	if len(mis) == 0 {
		fmt.Println("  (空 —— 生产将只剩 pi/default)")
	}
	for i := range mis {
		mi := &mis[i]
		fmt.Printf("  %-42s display=%-24q window=%d thinking=%-5v images=%v\n",
			mi.Id, mi.DisplayName, mi.ContextWindow, mi.SupportsThinking, mi.SupportsImages)
		if !validModelID(mi.Id) {
			fmt.Printf("  !! 坏 ModelID %q(缺 provider 段,手机选中必败)\n", mi.Id)
		}
	}

	// ---- 修复前路径(整树提取,对照)----
	fmt.Println("\n[legacy] agency.Compile + StoreFileSource(修复前路径,对照)")
	cs := agency.Compile(context.Background(), 10*time.Second,
		agency.StoreFileSource{Path: *path})
	if len(cs) == 0 {
		fmt.Println("  (空)")
	}
	for _, c := range cs {
		fmt.Printf("  provider=%-14q id=%-24q display=%-24q window=%d reasoning=%v\n",
			c.Provider, c.ID, c.DisplayName, c.ContextWindow, c.Reasoning)
		if c.Provider == "" {
			fmt.Println("  !! Provider 为空:0.99.1 真实形状(models 数组)下 provider 继承失效(G1 修复点)")
		}
	}
	fmt.Printf("\n汇总: prod=%d 条 ModelInfo;legacy=%d 条候选\n", len(mis), len(cs))
}

// validModelID ModelInfo.Id 的合法形状:恰好 "provider/id" 一段斜杠,两侧非空
// (Thread.ModelID 由 server 端再加 "pi/" 前缀合成 "pi/<provider>/<id>",
// configurePiSelection 按 "/" 切出 provider+modelId,缺 provider 段即"无效 Pi 模型")。
func validModelID(id string) bool {
	prov, mid, ok := strings.Cut(id, "/")
	return ok && prov != "" && mid != "" && !strings.Contains(mid, "/")
}

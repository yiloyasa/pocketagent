package main

// models 探测模式(2026-10-01"无法切换模型"专项):实证 pi 0.99.1 下
// get_available_models 的真实行为 —— daemon fetchModelsUncached 起同样参数的
// 一次性子进程,这里复现它的调用并原样记录应答/错误,作为修复依据。

import (
	"fmt"
	"time"

	"github.com/pocketagent/daemon/internal/piapp"
)

func modelsProbe() {
	c, err := piapp.Start(piapp.StartOptions{ExtraArgs: []string{"--no-session"}})
	if err != nil {
		fmt.Println("START_ERR:", err)
		return
	}
	defer c.Close()
	raw, err := c.Call("get_available_models", nil, 30*time.Second)
	if err != nil {
		fmt.Println("CALL_ERR:", err)
		return
	}
	out := string(raw)
	if len(out) > 4000 {
		out = out[:4000] + "...(截断)"
	}
	fmt.Println("RAW_OK len=", len(raw))
	fmt.Println(out)
}

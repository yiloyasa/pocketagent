# pi 协作 MCP 端到端证据(T-L2pi-prod 审核记录摘录,T-FU 补录)

- 摘录来源: T-L2pi-prod 审核员代跑记录(2026-09-21);本文件为摘录,完整原始输出以该审核会话记录为准
- 测试对象: `daemon/internal/piapp/mcpsetup_e2e_test.go` 的 `TestE2ECollabAgentDirRealPi`(真实端到端,门控 `PA_PI_E2E=1` 才运行)
- 运行命令: `PA_PI_E2E=1 go test ./internal/piapp/ -run TestE2ECollabAgentDirRealPi -v`(审核员代跑)
- 结果: **PASS**,总耗时 **14.02s**
- 运行时间戳: 2026-09-21(审核记录)

## 五要素(与测试断言一一对应,全部实证)

1. **真实网络安装**: `pi install npm:pi-mcp-adapter --no-approve` 真跑(SetupCollabAgentDir 生产路径),首次初始化含安装耗时 **6.5s**;
2. **安装痕迹落盘**: `<agentDir>/settings.json` 的 packages 列表含 `npm:pi-mcp-adapter`,`AdapterInstalled` 复核为 true;
3. **幂等短路**: 二次调用 `SetupCollabAgentDir` 把 `PI_CLI` 指向不存在的路径仍成功 —— 证明"已装则跳过"真实发生,未重跑网络安装;
4. **mcp.json 注册**: `WriteCollabMcpConfig` 把 MCP 服务器条目落盘 `<agentDir>/mcp.json`(原子写);
5. **真实 pi 会话 + 注入生效**: 经 `piapp.Start` 生产路径拉起真实 `pi --mode rpc` 子进程(`PA_PI_COLLAB=1` → `PI_CODING_AGENT_DIR` 注入),`get_state` 应答不报错 = adapter 就绪;session 文件落在注入的 agent 目录内(注入生效的直接证据),日志无 pi-mcp-adapter 报错。

## 隔离纪律(测试设计)

- `USERPROFILE` 重定向到 `t.TempDir()`:全程不触碰真实 `~/.pi` 与真实 `~/.pocketagentd/pi-agent`;
- 不复制任何凭据(CopyUserCredentials 设计上不自动调用)。

## 补录说明(T-FU,2026-09-21)

- 补录时 git HEAD: `ceebeb43113f16d498daa5bdde05f6021223443b`,go1.27.1 windows/amd64;
- 本文件只摘录审核记录的结论性事实(命令/结果/耗时/五要素),未重跑、未改写数值;常规回归(`go test ./...`)下该 E2E 整体跳过,属预期设计(单测层已钉住全部门控逻辑)。

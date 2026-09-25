# L2 真机回归 · 2026-09-25 17:40–18:15

## 环境与产物自证（L0）
- 真机：8e0e7bbe（USB），APK v0.2.2（cleanapk 重建，sha256 99ed667ef76c6cef…，18:45 装机 lastUpdateTime 17:45:35）
- daemon：热部署 `_pa_rebuild_daemon.py`，新 exe 29082624 bytes（17:40:31），PID 72172 监听 7246
- dex 自证：新增中文字面量 3/3 命中（控制中心/消息透析/降级文案）；`GetControlPlane` 类在包
- **D8 迁移真实生效**：2463 文件 / 98,141,407 字节 `~/.pocketagentd` → `%LOCALAPPDATA%\PocketAgent\data`，migration.json 带 manifest_sha256（09d36432…），旧目录保留可回滚
- daemon 日志：预算/协同策略从**新目录**装载、启动恢复报告 status=ok、providers pi 0.87.1 / dsh 0.1.5-rc.3 探测成功

## 连接故障与修复
- 手机直连失败根因：Windows 防火墙无 7246 入站规则（此前仅 demo exe 规则）。按 install.bat 同口径 UAC 提权补规则名 "PocketAgent"（TCP 7246 入站允许）后，手机 **Tailscale 通道连接成功**（01-home-connected.png）

## L2 验证结果（证据 l2-20260925/*.png）
| 项 | 结果 | 证据 |
|---|---|---|
| 手机控制中心入口+总览头（WHOISLOVE · v0.2.2 · 已运行 N 分钟） | ✅ | 03-control-expanded.png |
| **控制面八域真机渲染**（预算/协同/Agent接入/记忆/未知事件/审批/连接/资源，全带人话 summary） | ✅ | 06-control-plane-full.png |
| 协同域就地切换（委派子任务 关→开） | ✅ daemon 落盘 revision 29→30 双向确认（collab-policy.json + UI 回读一致） | 07-collab-domain.png |
| 消息透析弹窗：未知事件人话翻译+字段透析表+原文+五处置键真机渲染 | ✅（ZCode 资源采样事件被 W8 词表正确翻译） | 05-dialysis-sheet.png |
| 聊天新 UI（玻璃编辑条/模型选择器/发送链路：hello 上屏） | ✅ 上屏成功 | 10-chat-turn.png |
| dsh 旧会话 resume（rc.3 对旧 sessionId 报 -32602） | ⚠️ 如实报错原文+重试键，不伪造（见"发现的适配债"） | 08-chat-retry.png |

## 发现的适配债（如实记录，不阻塞本轮）
1. **dsh rc.3 旧会话不可 resume**：升级后旧 sessionId 在 rc.3 的 session/list 中不存在，remountSession 如实拒绝（设计正确——不静默新开）。但 UI 层"重试"重放同一路径必然再失败，用户无出路。建议：resume 失败时提示"该会话因 agent 升级无法恢复，删除后重建"或自动降级为新会话继续（需产品决策）。
2. pi 子进程 RPC 模式拉起即退（exit status 1）——与本轮 UI 无关，daemon 日志可见；待专项排查（0.87.1 rpc 模式启动参数或环境差异）。
3. mDNS 5353 被浏览器占用（日志既有警告，Tailscale 通道不受影响）。

## 结论
部署链（热部署/迁移/L0/装机/连接）全通过；控制面与透析两大新功能在真机**功能与视觉双验收通过**；协同开关就地切换经 daemon 落盘双写确认。发现的 2 项适配债已挂牌（dsh resume 升级兼容、pi rpc 启动），下轮处理。

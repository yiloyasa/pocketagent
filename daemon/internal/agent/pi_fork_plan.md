# pi 分叉(ForkThread)接通实施清单 —— 只报告不实施(2026-09-30,P1)

**结论:可行。** pi 0.99.1 原生有完整的 fork 能力面(`fork`/`clone`/`get_tree`/`get_fork_messages`,
m99 探针 12/12 通过,活体证据 `docs/verify/evidence/2026-09-30/pi-0991-m99.md` §12 与附录
`piprobe-18/19/20/21`),server 侧只是硬路由 codex。接通 = server 两处路由/位改造 + PiProvider
一个方法 + 一份新接口断言,**proto 零改、手机端零改**。预计 ~200 行(含测试)。

---

## 0. 语义对齐(为什么 pi 的对等物是 `clone` 而不是 `fork`)

| | codex `thread/fork`(现状,codex_threads.go:308) | pi `fork {entryId}` | pi `clone` |
|---|---|---|---|
| 效果 | **新线程**复制源历史,"fork through inclusive" | 按用户消息 entryId **切换当前进程**到新分支会话文件(createBranchedSession) | 在**当前叶**做 fork:新分支会话文件复制活跃分支全量,进程切过去 |
| 原会话 | 不动 | 文件不动,但**进程已离开** | 同左 |
| 对"分叉出一条新会话,原会话不动"的 server 语义(thread_ext.go:273 注释) | 直接对等 | 半对等(需再切回来) | **直接对等** |

pi 侧机制(bundle 静态证据,实施时以这些为准):
- `clone` RPC handler:`dist/modes/rpc/rpc-mode.js:488-498` —— 取当前 `leafId` →
  `runtimeHost.fork(leafId, {position:"at"})` → 成功后 `rebindSession()`;应答
  `data={cancelled}`(**不回新会话 id**,新 sessionFile/sessionId 靠后续 `get_state` 取)。
- `fork` 实现:`dist/core/agent-session-runtime.js:197-247` —— persisted 会话走
  `SessionManager.open(currentSessionFile)` + `createBranchedSession(targetLeafId)`
  (`dist/core/session-manager.js:1205`),随后 `teardownCurrent("fork",…)` 并把进程
  `apply` 到新 runtime(`session_start` 事件 `reason:"fork"`)。
- 前置闸门:`agent-session-runtime.js:217` `existsSync(currentSessionFile)` 不满足即抛
  `"This session has not been saved yet. Send a message before cloning or forking it."`
  (m99 实测复现:fork 刚分出的新会话文件尚未落盘时,紧接 clone 失败 —— 证据
  pi-0991-m99.md 附录 `piprobe-21`)。
- 取消面:`session_before_fork` 扩展事件可取消(rpc-commands.md:612/640)。**daemon 的
  守护扩展只挂 `tool_call`**(daemon/internal/piapp/guard.go:63),无此钩子,不会被取消。
- `get_fork_messages`(rpc-commands.md:666-687)→ `data.messages[]={entryId,text}`
  (仅**用户消息**,m99 实测 3 条全为 prompt 原文)。
- `get_tree`(rpc-commands.md:719-744)→ `data={tree[],leafId}`,leafId=null 表示空会话。

---

## 1. server/thread_ext.go 路由改造点

`ForkThread` 现体:`daemon/internal/server/thread_ext.go:276-322`。

| 行 | 现状 | 改造 |
|---|---|---|
| 276 | `func (s *Server) ForkThread(ctx, req) (*pb.Thread, error)` | 不动 |
| 277 | `src, ok := s.cfg.Store.Get(req.ThreadId)` | 不动 |
| 281 | `p := s.codexProvider()`(model.go:221-226,硬路由 codex) | **在其前插 pi 分支**,仿 `CompactThread` 的 pi 路径(server/thread.go:600-611):`if threadAgentID(src) == "pi" { pc, ok := s.cfg.Providers["pi"].(piShapedForker); if !ok { return nil, errUnavailable("电脑上没有可用的 pi") }; return s.forkPiThread(src, pc) }`。`threadAgentID` 已存在(thread.go:1526-1531,空串视作 codex,老库零感知) |
| 285 | `if src.CodexThreadID == ""` → FailedPrecondition | pi 分支内同款前置:`src.ExternalSession.SessionFile == ""` → 同文案的 FailedPrecondition(与 thread.go:607 zcode 分支同式) |
| 288 | `p.ForkThread(src.CodexThreadID, req.LastTurnId)` | pi 分支改调 `pc.ForkThread(src)`(返回新 ExternalSessionRef,见 §3) |
| 292-308 | 组装 `store.Thread{… CodexThreadID: newCodexID …}` | pi 分支组装:`AgentID:"pi"`、`ExternalSession: 新ref`、**不填 CodexThreadID**;`Messages` 用 `forkSeedMessages("", src, nil)` 的兜底路径(327-343:`newCodexID==""` 时复制 src.Messages,天然复用) |
| 309 | `Store.Create(t)` | 不动(共用) |
| 315-317 | `p.SetCodexThreadName` 标题写回 | pi 分支换成 `Rename(t, t.Title)`(titlePusher,thread.go:566-568;生产实现 pi.go:1699 → set_session_name),失败同样只记日志 |
| 318 | codex 日志 | pi 分支平行一条 `[fork] …(pi <srcFile8> → <newFile8>)` |

实现载体建议:新增私有方法 `forkPiThread(src *store.Thread, pc piShapedForker) (*pb.Thread, error)`
放在 ForkThread 之后、forkSeedMessages 之前,避免在原函数里再叠分支;codex 路径(281-322)一行不动。

**新接口声明落点**:`server/thread.go` 的最小接口族(543-568 一带,piThreadCompactor 同款
注释风格),不在 host.go —— host.go 的 type 块(host.go:31-58)本轮纪律归"位声明"用途,
接口与路由同文件(thread.go)与 piThreadCompactor 先例一致。

```go
// piShapedForker pi 侧分叉能力(pi 0.99.1 clone;生产由 *agent.PiProvider 实现)。
// 参数 *store.Thread,与 codexShapedForker 的 string 形状天然互斥 —— 一家只命中
// 一个(host.go:34-36 compactor 同款纪律),能力位断言才不会误亮。
type piShapedForker interface {
	ForkThread(t *store.Thread) (store.ExternalSessionRef, error)
}
```

> 注意:**不要**让 PiProvider 实现与 codexShapedForker 同形签名,否则 host.go:151 的断言
> 会给 pi 误亮 `fork:pi` 位而路由仍走 codex 形状 —— 形状互斥是这套位声明体系的根基。

## 2. server/host.go 位断言改造点

| 行 | 现状 | 改造 |
|---|---|---|
| 45-49 | `codexShapedForker` 定义,注释"pi/zcode/dsh 无对等能力" | 注释改为"zcode/dsh 无对等能力;pi 走 piShapedForker(thread.go),同 compact 三形状一位先例"。接口本体不动 |
| 151-153 | `if _, ok := provider.(codexShapedForker); ok { caps = append(caps, "fork:"+id) }` | 其后并排加 `if _, ok := provider.(piShapedForker); ok { caps = append(caps, "fork:"+id) }`(与 compact 三连断言 host.go:141-149 完全同式;同 id 同位,不会重复追加不同串) |
| host_capability_bits_test.go:15 | "fork/rollback/review:<id> = codex 专属" | **必须同步改**:fork 从"codex 专属"名单挪出,与 compact 同列"多形状一位";rollback/review 维持 codex 专属 |

## 3. PiProvider 需实现的方法与 pi RPC 调用序列

落点:`daemon/internal/agent/pi.go`(CompactThread §1563 与 ExportHTML §1598 之间或其后,
同为"线程级控制面"家族)。

```go
// ForkThread 把当前 pi 会话在"当前叶"分叉成一个新会话(pi 0.99.1 clone RPC),
// 返回新会话的挂载引用;原会话与其子进程不受影响(子进程在 clone 后已切到新
// 会话,这里选择关闭回收,两条线程下次 Run 各自冷启动重挂载 —— 见步骤 6)。
// 未落地(ExternalSession.SessionFile 空)/轮次运行中/会话未落盘,一律结构化报错。
func (p *PiProvider) ForkThread(t *store.Thread) (store.ExternalSessionRef, error)
```

调用序列(每步的依据见 §0):

1. **闸门(本地,不发 RPC)**
   - `t.ExternalSession.SessionFile == ""` → `&piError{code:"PI_NOT_MOUNTED", …}`(server 侧
     也有同款前置,双保险;piError 形状见 pi.go 现有用法,CompactThread 同款闸门风格)。
   - `p.running[t.ID]` 为真 → `&piError{code:"PI_BUSY", msg:"该 Pi 会话已有一轮在执行"}`
     (与 acquireRunning(pi.go:705)同码;fork 改变会话树,运行中一律拒)。
2. **挂载**:`c, err := p.ensureMounted(t)`(pi.go:1543 —— clientFor + mount 握手,
   CompactThread 同款入口;不直接用 clientFor,它不保证已挂载)。
3. **前置校验(可选但建议)**:`c.GetState(ctx)`(piapp/client.go:715)取 sessionFile,
   `os.Stat` 存在性检查 —— 对齐 bundle 闸门(agent-session-runtime.js:217),把 pi 的
   英文报错提前变成结构化中文错误("先发一条消息,落盘后才能分叉")。
   `get_tree` 的 `leafId`(rpc-commands.md:719-744)此时一并可取:null 即空会话,同样
   结构化拒绝(clone 在空会话上会被 rpc-mode.js:490-492 以 "no current entry selected" 拒)。
4. **`clone`**:`c.CallCtx(ctx, "clone", nil, piCallBudget)` → 成功且 `data.cancelled==false`
   继续;`cancelled==true`(被扩展取消)→ 结构化错误。**`get_fork_messages` 不在 v1 序列里**:
   它服务"按历史某条用户消息分叉"(fork {entryId}),而 daemon 的 LastTurnId 没有可靠的
   entryId 映射(见 §5 范围裁决),v1 固定"从最新状态分叉",与 codex lastTurnID 为空的
   主路径(codex_threads.go:310 注释)语义对齐。
5. **取新引用**:`c.GetState(ctx)` 第二次 —— clone 后进程已切到新分支会话,GetState 的
   sessionFile/sessionId(piapp/client.go:724-728)经 updateSession(246)刷新缓存;
   组装 `store.ExternalSessionRef{SessionID: c.SessionID(), SessionFile: c.SessionFile(), Cwd: t.Workspace}`
   (与 mount 回填 pi.go:470-474 同式)。
6. **子进程归属裁决(推荐:关闭回收)**:clone 后这个子进程已属于"新会话",但新线程 id
   由 server 生成,provider 拿不到。两个变体:
   - **v1 采纳(推荐)**:`c.Close()` + `delete(p.clients, t.ID)` + 清 cursors(DropThread
     pi.go:421-432 的内存回收三连)。源线程下次 Run:clientFor→mount 见 SessionFile 不符
     →switch_session 切回源文件(pi.go:451-462);新线程首次 Run:mount 走 switch_session
     挂上分支文件。代价 = 两条线程各多一次冷启动 spawn;收益 = 零事件错账(GlobalHandler
     按 threadID 闭包记账,pi.go:396-407,若把"挂在新会话上的子进程"留给任一线程都会把
     对方的事件记错账)。
   - 变体 B(记录备查):server 建完新线程后回调 `pc.AdoptClient(srcID, newID)` 重键
     p.clients/cursors。省两次 spawn,但多一个接口方法 + GlobalHandler 需按新 threadID
     重闭包(piapp.Client 无换 handler 的公开口子,还要加),复杂度不值,v1 不做。
7. **落库由 server 做**(forkPiThread,§1):provider 只回 ref,不碰 store —— 与 mount
   "回填由 server 落库"的分工(pi.go:464 注释)一致。

**LastTurnId(v2 展望,v1 明确忽略)**:`req.LastTurnId` 是 codex 回合 id 语义;
pi 侧对等物是 get_fork_messages 的 `entryId`。daemon 的 store.StoredMessage 不带 pi
entryId,按 text 反查会把重复 prompt 映错(m99 实测一轮里就有两条一模一样的 prompt)。
v2 若要做:Run 落库路径给 StoredMessage 增配 `pi_entry_id`(只增 JSON 字段,老库兼容),
fork 时按 id 直查。**手机端 v1 照发 last_turn_id,daemon 忽略之 —— 降级为"从最新分叉",
不破坏。**

## 4. 手机端无需改动的论证

1. **协议面零改**:`ForkThreadRequest{thread_id, last_turn_id}` / 响应 `pb.Thread` 已存在
   (daemon/internal/gen/pocketagent/v1/thread.pb.go:1644 起),字段全复用,proto 不动。
2. **入口点亮走能力位**:手机端按 `fork:<agent>` 位显隐分叉入口(host.go:150-153 上方
   注释块 132-140:缺位=入口隐藏,不摆假按钮)。现网 APK 已认识 `fork:` 前缀(codex 在用);
   daemon 给 pi 补位后,**旧 APK 无需升级即自动点亮 pi 的分叉入口** —— 与 compact 三形状
   一位的上线方式完全相同(host.go:113-150 注释自证"旧 daemon 不带这些位,新 APK 按位点亮,
   旧 APK 多收几个未知串无副作用")。
3. **返回物是普通线程**:forkPiThread 走 `s.threadToPB(t)`(thread_ext.go:321 同源),
   手机端把它当普通 pi 会话渲染;历史来自本地 store(GetThreadHistory 只读本地库,
   thread_ext.go:299-303 注释自证),v1 已用 src.Messages 复制播种,时间线不空白。
4. **错误呈现零新增**:全部走既有 gRPC 码族(errUnavailable/errFailedPrecondition/errInternal,
   codex fork 的"先发一条消息"提示同款,手机端已有该文案渲染路径)。
5. **last_turn_id 语义降级可接受**(§3 v2):字段可选,不发的 APK 与 v1 语义完全一致。

## 5. 测试清单(实施时)

- server 侧:`thread_ext` fork 路由单测(仿 CompactThread 的分家测试)——pi 线程走
  piShapedForker(注 fake)、codex 线程仍走 codexProvider、dsh/zcode 线程显式
  FailedPrecondition、能力位 `fork:pi` 出现(host_capability_bits_test 对表)。
- agent 侧:`PiProvider.ForkThread` 闸门单测 —— 未落地/运行中 → piError 码;
  clone cancelled=true → 错误;用 piMountClient(pi.go:439-445)打桩钉"clone→get_state
  回填 ref→Close 回收"的序列(生产零适配保证:`var _ piMountClient = (*piapp.Client)(nil)`,
  pi.go:448 同款编译期断言)。

## 6. 实施前必须活体复核的两件事(隔离 PI_CODING_AGENT_DIR,红线不碰 ~/.pi)

1. **分支会话文件落盘时机**:m99 实测 fork 刚产出的新会话文件在 clone 视角
   `existsSync` 为假(pi-0991-m99.md 附录 piprobe-21)。须验证:正常已保存会话上
   `clone` 成功后,新 sessionFile 是否**立即可被 switch_session 打开**(决定 §3 步骤 6
   关闭回收后的重挂载是否稳);若落盘是惰性的,关闭前需补一次强制 flush(如
   set_session_name 触发持久化)或改采变体 B。
2. **clone 后 get_state 的字段完备性**:确认新 sessionId/sessionFile 均非空(m99 只验到
   `data={cancelled}`,新会话身份字段是从 get_state 推断的,rpc-mode.js:495
   rebindSession 的语义须实测钉死)。

## 7. 证据索引

- 活体:`docs/verify/evidence/2026-09-30/pi-0991-m99.md`(A12 §12;附录 piprobe-18/19/20/21;
  fork 成功帧 431、clone 失败帧 433)
- 静态:pi 随包 `docs/rpc-commands.md:610-687`(fork/clone/get_fork_messages)、
  `docs/json.md`(get_tree/leafId);bundle `dist/modes/rpc/rpc-mode.js:488-502`、
  `dist/core/agent-session-runtime.js:197-247`(闸门 217)、
  `dist/core/session-manager.js:1205`(createBranchedSession)、
  `dist/modes/rpc/rpc-types.d.ts:112/325-340`
- 路由先例:server/thread.go:592-640(CompactThread 四家分家)、thread.go:1526-1531
  (threadAgentID)、host.go:141-153(compact/fork 位断言)

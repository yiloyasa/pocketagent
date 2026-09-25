# Pi 0.87.1 followUp 原生投递线证据(prompt streamingBehavior / queue_update)

> 生成:2026-09-25 工具:**静态 bundle 检索**(node_modules 内 0.87.1 发行物逐行核对,
> 未拉起任何子进程——本记录零探针进程、零写副作用)
> 核对物:`@earendil-works/pi-coding-agent@0.87.1`(package.json 实证版本号),
> `dist/core/agent-session.js`、`dist/modes/rpc/rpc-mode.js`、`dist/modes/json-event.js`、
> `docs/rpc-commands.md`(随包文档)、`docs/json.md`(随包文档)。
> 活体报文交叉印证:docs/verify/evidence/pi-s-probe.md(2026-09-19 S6 项,真实
> `follow_up` 命令 + 3 次 `queue_update` 原始帧)。

## 1. `prompt` 命令带 `streamingBehavior`(投递通道形状)

随包 docs/rpc-commands.md "Prompting/prompt":

```json
{"type": "prompt", "message": "New instruction", "streamingBehavior": "steer"}
```

- `"steer"`:当前助手轮工具调用结束后、下一次 LLM 调用前注入;
- `"followUp"`:**等 agent 停止后才投递**;
- 流式期间不带 `streamingBehavior` → 命令报错(success:false)。

RPC 层透传(dist/modes/rpc/rpc-mode.js,`case "prompt"`):

```js
void session.prompt(command.message, {
    images: command.images,
    streamingBehavior: command.streamingBehavior,
    source: "rpc", ...
```

会话层语义(dist/core/agent-session.js,`async prompt(text, options)`):

```js
if (this.isStreaming) {
    if (!options?.streamingBehavior) { throw ...("Specify streamingBehavior ('steer' or 'followUp')..."); }
    if (options.streamingBehavior === "followUp") { await this._queueFollowUp(expandedText, currentImages); }
    else                                          { await this._queueSteer(expandedText, currentImages); }
    return;
}
```

【关键】`streamingBehavior` **只在流式期间生效**;pi 空闲时该参数被忽略、prompt
立即执行。daemon 侧因此必须先判"有运行中轮次"再投(投递守卫,见 §4)。

## 2. `queue_update` 事件(回执链映射的数据源)

随包 docs/json.md 事件表:

| 事件 | 字段 | 含义 |
|---|---|---|
| `queue_update` | `steering`, `followUp` | 排队队列变化;**两字段都是当前完整队列** |

发射点(dist/core/agent-session.js):

```js
_emitQueueUpdate() {
    this._emit({ type: "queue_update",
                 steering: [...this._steeringMessages],
                 followUp: [...this._followUpMessages] });
}
async _queueFollowUp(text, images) {
    this._followUpMessages.push(text);
    this._emitQueueUpdate();          // 入队必发
    ...
}
clearQueue() { ...清空两队列...; this._emitQueueUpdate(); return { steering, followUp }; }
```

RPC 模式经 `session.subscribe((event) => output(toJsonEvent(event)))` 原样外发
(toJsonEvent 只改写 message_update,其余事件透传)。活体帧样例(pi-s-probe.md S6):

```
{"type":"queue_update","steering":[],"followUp":["只回复四个字:排队收到。"]}
...
{"type":"queue_update","steering":[],"followUp":[]}
```

【消费含义】对比相邻两帧 followUp 数组长度的增减,即可无损推断"入队了几条 /
出队了几条"——daemon 据此映射 Queued(+1)/QueueFlushed(−1) 回执帧,手机排队
计数继续正确(手机端口径:Queued position>0 时 +1,QueueFlushed 时 −1,
android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt)。

## 3. settled 前自动续跑;abort 不续跑但不清队列(取消语义的依据)

`_shouldContinue`(dist/core/agent-session.js,轮末判断是否继续 drain 队列):

```js
return !this._agentRunAbortRequested && this.agent.hasQueuedMessages();
```

- 正常轮末:队列非空 → 继续跑(followUp 在**同一 settled 周期内**续跑,活体证据
  pi-s-probe.md S6 注记"排队后在本 settled 周期内续跑")——daemon 的 Run 一次
  prompt 就能覆盖"当前轮 + 全部 followUp",无需 daemon 队列接力;
- `abort()` 置 `_agentRunAbortRequested` → **不再续跑**,但队列条目**留在内存队列**
  (只有 clearQueue 清):下一条 prompt 的运行周期会把它带出来执行。因此 daemon 的
  "终止任务"路径在 abort 后必须补一次 `clear_queue`,对齐"取消 = 排队的也不跑"。

## 4. 对 PocketAgent 接线的约束(本轮实现据此落码)

| 约束 | 依据 |
|---|---|
| 投递前必须确认"有运行中轮次且未 settled",否则 prompt 立即执行成新轮次 | §1 空闲忽略 streamingBehavior |
| 入队/出队回执以 queue_update 事件为准(不自发造 Queued 位次) | §2 两帧对比即真相 |
| 终局判定不用改:followUp 在 settled 前跑完,Run 一次覆盖 | §3 _shouldContinue |
| 用户取消 = abort + clear_queue(撤销已预投递条目) | §3 abort 留队列 |
| 只做投递与只读,不碰 set_auto_*(上轮探针实证落盘 ~/.pi 全局) | pi-native-settings-probe.md 红线判定 |

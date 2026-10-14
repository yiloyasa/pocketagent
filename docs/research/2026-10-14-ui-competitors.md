# PocketAgent 竞品审美与交互调研

> 调研时间：2026-10-14 · 范围：AI 编程工具（Cursor、GitHub Mobile、Windsurf、Replit、CodeSandbox、Aider、Cody/AMP、Continuum、Claude Code 移动端）+ 主流 AI 对话 App（Claude、ChatGPT、Gemini、Perplexity）
> 用途：Flutter 重构 UI/交互设计依据（与国内审美调研、Flutter 可行性报告并列）

## 一、各工具/App 关键 UI 模式

### A 类：AI 编程工具

**Cursor（IDE + iOS，2026.6 公测）**
- IDE chat 400–520px 侧栏，composer 贴底向上生长；Agent 模式展示"运行中的编辑与命令列表"，终端命令和文件读取各自可展开；`@` 提及以 pill 胶囊渲染；模型推理藏 disclosure 折叠区；完成后显示 token 数与成本
- Review flow：一次接受全部编辑 / 逐文件浏览；checkpoint 版本回滚
- iOS：可启动云端 always-on agent，或 **Remote Control** 遥控本机 agent；语音输入、slash 命令、任意前沿模型；**Live Activities** 锁屏显示进度；完成/需输入/待 review 时推送；云端 agent 产出 demo/截图/日志可 App 内 inspect diff、留言、合并 PR；截图作视觉上下文

**GitHub Mobile**
- 通知为中心；PR review mobile-optimized code view；社区反馈 split diff 手机上不可用需切 unified；Copilot coding agent 把 session 日志写进 PR timeline（审批与审计合一）；为百万行级 PR 重建 diff 渲染面（虚拟化+内联评论）

**Windsurf（Cascade）**
- Cascade 面板：先 plan、跨文件编辑、调用工具，**diff staging + 逐步审批**，内置 checkpoint 回退；多 pane 并行；无原生移动 App——第三方 CodeAgent Mobile 通过 Open VSX 扩展配对（QR/6 位码），流式响应/diff/工具调用/推理过程回传手机，y/n 交互式提问可手机点选

**Replit Agent**：动手前先展示可调整 plan；Agent 4 新增 design canvas；App 在路上

**Aider**：终端 chat 驱动编辑、git 自动 commit、unified diff 编辑格式（文档称显著降低模型偷懒率）

**Continuum（最直接竞品）**
- iPhone App 是完整 Code workbench：会话详情六 tab——**Chat / Plan / Diff / PR / Terminal / Files**；tab 仅在有内容时才出现；**手机记住每个 session 停在哪个 tab**
- **Plan mode 是真正的 gate**：agent 提议 → 手机读步骤 → Approve 之前不动 repo
- Diff 按 file/hunk 渲染；PR tab 带 GitHub 实时状态与 Merge；Terminal 是真 PTY，键盘 accessory row 补手机键盘缺失的键
- **Command outbox**：所有写操作立即入队、composer 即刻清空；退避 1s→4s→15s→60s→5min→30min；耗尽后停在 outbox 面板可滑动重试/取消；每个 envelope 带 **idempotency key**，宿主重放回执而非重跑，断网不会双发 prompt 或双 merge
- Widgets（用量 gauge）与 **Live Activity**；语音系统键盘；Mac/Web/iPhone 对等端，同一 session 同分支同 transcript；休眠和网络切换自动重连

**Claude Code 移动端（官方）**
- 不做独立 App：Claude App 内 **Code tab**，容纳 cloud sessions / Projects / **Remote Control**（QR 配对驱动本机会话）/ Dispatch；照片附件直接可见、其他文件下载为 `@` 引用；长任务完成或需决策时推送，可说 "notify me when tests finish"；权限模式 Accept edits / Plan / Auto，**移动端刻意不提供 Bypass**；休眠后自动重连

### B 类：主流 AI 对话 App

**Claude**：全宽消息、不用 SMS 气泡；**thinking 默认折叠+一行摘要**；Artifacts 侧栏（Preview/Code 双 tab，复制作用于代码块）；流式 caret 是小实心方块；每条回复标注模型名；regenerate 用 carousel 轮播保留历史版本

**ChatGPT**：代码块一键复制（边打字边显示复制按钮是已知反模式）；Canvas 侧编辑器带快捷键菜单；reasoning 可折叠摘要；Stop generating 流式中断；错误按类型区分

**Gemini**：Material 3、渐变与动效建立信任感；曾有多草稿 "Show drafts"（用户要求回归）

**Perplexity**：**引用作编号脚注**链接信源（信任模式行业基准）；Focus 选择器；Related follow-up chips 密集；自动滚动处理正确——用户上滚即锁定位置并出现 jump-to-latest；内容 max-width ~720px

**Cursor chat**：`Cmd+Enter` 发送 / `Shift+Enter` 换行；完成后显示 token 数与成本；消息操作含领域动作 **Apply to file**；`@filename` 引用；inline diff accept/reject

## 二、编程 agent 通用设计模式

**双层时间线**：对话层（消息流）之下叠**任务层（步骤账本）**：pending=空心圆、running=动画、done=对勾、failed=叉；每步默认折叠为一行，展开才显示工具调用；running step 固定可见；**失败步骤保留原位并标记**。未知长度任务禁用百分比进度条（"90% 卡三分钟"是 2026 年已上市反模式），改显示"第 X 步 · 已耗时 Y"。（setproduct patterns 6/7/15）

**工具调用卡三级折叠**：
- L0 账本行：步骤名+状态图标+耗时（默认态）
- L1 卡头：工具名+输入摘要+耗时+状态+权限徽章（**移动端默认折叠到这一级**）
- L2 输出体：截断、可展开；连续相同调用合并为一张带计数
- 推理/思考独立成区：默认折叠+一行摘要。判别原则："改变了世界的是工具调用，改变了想法的是推理"。**绝不把需要用户行动的信息放进折叠区**。

**审批卡危险度分级**：
- 读：灰色，不提示
- 写：蓝色，每 session 每 scope 问一次，提供 "always allow"
- 破坏性：红色，**每次都问、无 always-allow**，且**布局要不同**（不同形状打破肌肉记忆，不止换色）；徽章色盲安全（形状/图标差异）
- 批量审批：同层级动作合并为可滚动清单（默认全选、一个 Approve），破坏性项单独呈现；每项显示后果（`auth.ts +40 −12`）

**流式中断与恢复**：
- Stop 应是 **Pause**：暂停而非杀死，账本冻结、已完成步骤保留，出现"要改什么？"输入框，agent 修订剩余计划并显示 diff 后恢复；按钮文案写 Pause，因为用户怕按 Stop 丢工作
- 停止后保留部分输出，提供 Continue 和 Regenerate 两个明确出口
- 错误必须分类（网络掉线/限流/内容过滤/上下文超限）+ 对应恢复动作，禁止笼统 "Something went wrong"
- 断流恢复：从最后 token 有序续传、不重复；重连后加载离线期间完整状态（需要 streamId + offset 或 durable session）
- 首个 token <800ms；caret 是最低成本"活着"信号；用户上滚即锁定滚动并显示 Jump to latest

**离线/断连**：
- **命令 outbox**：写操作立即入队、composer 即刻清空；退避重试；耗尽停在 outbox 面板滑动重试/取消；idempotency key 防双发
- 顶部琥珀色断连横幅
- 导航 pill 区分"运行中"与"等你输入"——后者变琥珀+问号图标；两者混为一谈会损失数小时
- 休眠/网络切换后自动重连；锁屏 Live Activity + 推送，推送要"说出发生了什么"（agent、repo、耗时、摘要），而非"一个会话结束了"

## 三、针对 PocketAgent 的 10 条落地建议

1. **双层时间线**：消息流下叠步骤账本（状态图标+步骤名+耗时），账本行可展开出工具调用卡。移动端列表场景账本优先
2. **工具调用卡默认折叠到 L1 卡头**：工具名+关键参数+耗时+状态徽章；输出体折叠；连续相同调用合并计数（"读文件 40 次"压缩成一张卡）
3. **三级审批视觉语言**：读=灰无提示；写=蓝、可"本 session 始终允许"；破坏性=红、每次必问且不同卡片布局（实线粗边框+⚠+反向按钮序），审批卡固定底部拇指区
4. **批量审批 + 按风险排序 diff 摘要**：同层文件变更合并清单，每项 `文件 +增 −删`，删除行优先、外部写入次之、本地编辑最后
5. **Pause 而非 Stop**：中断按钮暂停任务、保留已完成步骤与部分输出，提供 Continue / Redirect / Regenerate
6. **命令 outbox + 幂等键**：写操作入队即清空 composer；退避 1s/4s/15s/60s/5min/30min；耗尽停在 outbox 面板；服务端按 idempotency key 去重
7. **断连三件套**：顶部琥珀色断连横幅；重连拉取离线期间全部事件（不重跑推理）；锁屏 Live Activity + 推送含 agent 名/repo/耗时/首条摘要
8. **思考卡折叠纪律**：默认折叠+一行摘要；需要用户决策的内容必须出现在账本里展开显示
9. **移动端 composer 规范**：固定底部（非浮动覆盖消息）、随键盘升起；键盘打开时只显示最近 2–3 条消息并隐藏非必要 header；发送/停止按钮 ≥44pt；流式期间上滚即锁定并出现 Jump to latest
10. **每条 assistant 消息标注模型+思考强度**；任务结束附 **Verified / Assumed / Could not do** 三段式脚注，空段显示"无"

## 四、配色与材质

- 深色优先：近黑底非纯黑（`#0A0A0B` / `#111113`），Surface 分 2–3 层，1px hairline 边框而非厚重阴影；Linear/VS Code 式扁平低饱和是开发者工具审美共识；Light mode 必须做但默认深色
- 等宽字体仅限代码区域（Roboto Mono / JetBrains Mono）；正文系统无衬线
- 语法高亮取舍：流式期间代码块先按纯文本渲染（token 常到达在 fence 中途），闭合后二次渲染高亮，避免闪烁；高亮主题必须保证 WCAG AA 4.5:1
- 状态色：运行中=品牌色/白，pending=灰，done=绿✓，failed=红✗，警告/断连=琥珀；审批三级灰/蓝/红，破坏性叠加形状差异
- 拒绝玻璃拟态和 SMS 圆角气泡；消息全宽+微妙底色区分用户/AI；动效仅保留 caret 脉动与状态过渡，尊重 prefers-reduced-motion
- 密度：长文 max-width 65–72 字符/行、行高 ~1.6；移动端 16px 侧边距

**核心结论**：直接对标 Continuum 移动 workbench（六 tab、outbox、plan gate）与 Cursor iOS 的通知/Live Activity 层；审美基准是 Claude/ChatGPT 的全宽消息+折叠思考；交互红线是 setproduct 六大反模式（无限 spinner、审批疲劳、自信的错误结果、聊天记录当审计日志、隐藏的自主行为、假进度）。最高优先级落地四项：工具调用卡默认折叠到卡头级、审批灰/蓝/红三级且破坏性换布局、Stop 改 Pause、命令 outbox 幂等入队。

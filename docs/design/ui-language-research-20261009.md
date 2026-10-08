# PocketAgent UI 设计语言外部调研（2026-10-09）

> **性质**：纯调研文档，为"下一轮 UI 大改"提供提案依据。不改任何代码、不改既有设计令牌。
> **触发**：用户要求 App UI 参考网易云音乐等主流 App 重新提升；上轮侦察结论"视觉身份弱、缺品牌记忆点、信息密度高但层级平"。
> **方法**：WebSearch + 文章抓取（来源逐条标注；未检索到一手文章的部分标注"设计知识综述"）。搜索服务间歇限流，个别条目转引自搜索摘要，均已注明。

---

## 〇、本 App 设计本底速览（对比基准）

来源：`android/app/src/main/java/dev/pocketagent/app/ui/theme/PaEditorial.kt`、`ui/anim/PaMotion.kt`、`ui/theme/PaGlass.kt`（读取于 2026-10-09）。

| 维度 | 现状 |
|---|---|
| 色彩 | 编辑式三盘同构（LIGHT 冷白画布 / DARK 深蓝灰 / HACKER GitHub Dark+电光青 #22D3EE）。主色蓝 #386BE7，纪律：**主色只用于"当前操作"**；语义色五件套（RUNNING 绿 / ATTENTION 琥珀 / NEUTRAL / DENIED 珊瑚 / GREY）必须带状态词，不单靠颜色 |
| 字体 | Type 八档：Display 34 Poppins Bold / Title 25 Poppins Bold / Heading 16 / Body 14 / BodySmall 13 / Caption 12 / Micro 11 / State 10；等宽 JetBrains Mono 管代码/终端/数字 |
| 圆角 | 六档已机械收敛：Terminal 6 / Chip 10 / IconButton 13 / Card 18 / Sheet 23 / Pill 100（全 app 260 处裸写已归档，**新增圆角值违反既有纪律**） |
| 动效 | PaMotion 七档 spring（press / standard / emphasized / momentum / sheet / nav / instant）+ 循环令牌 pulse/sweep；可中断性铁律；减弱动效退化 |
| 材质 | PaGlass 玻璃（半透 + 顶缘发丝高光）+ paPushBack 推背模糊（浮层开合时被覆盖内容模糊+缩小） |
| 屏幕 | 工作台(Home) / 会话列表(Threads) / 会话(Chat) / 设置 / 任务中心 / 协同 / 文件 / Git / 终端 / 浏览器 + PaSheet 统一模态家族 |
| 已落地 | 语法高亮、duotone 图形、Poppins/JetBrains Mono、机械收敛（圆角/弹簧/循环） |
| 短板 | 视觉身份弱（看一眼记不住）、缺品牌记忆点、信息密度高但层级平 |

---

## 一、各对标拆解

### 1.1 网易云音乐（用户钦点）

**色彩策略**
- 红/白/黑三色体系，比同类音乐产品更简洁直观；演进方向是**从"大面积红"退到"大面积白"**，红色收缩为图标、主按钮、播放态等点缀位，黑色承担文字——层次感反而更分明。（来源：[网易云UI设计细节剖析，即时设计](https://js.design/special/article/netease-cloud-ui-design.html)）
- 品牌红 ≈ #C20C0C（"网易红"），与黑胶 VIP（Vinyl Member）订阅、黑胶播放器共同构成"红=品牌、黑胶=产品人格"的双记忆点。（#C20C0C 与黑胶绑定关系为设计知识综述；黑胶 VIP 见 [Wikipedia](https://en.wikipedia.org/wiki/NetEase_Cloud_Music)）
- 深色/炫酷黑皮肤：大面积深色 + **更低对比度**换取视觉舒适，配合封面内容成为主角。（深色方法论转引自站酷 B 站暗黑模式改版案例的搜索摘要；皮肤设置路径为设计知识综述）
- **封面取色**：播放页配色可由专辑封面主色提取生成，环境色随内容变化——"沉浸感"的一半来自这个。（转引自 [CSDN 沉浸式播放界面定制](https://blog.csdn.net/gitblog_00134/article/details/159783695)对官方行为的描述）

**导航**
- 移动端底部四 Tab（首页/社区/播放…），结构多年稳定；顶部 Tab 与搜索同区排布保持对称。（搜索框与 Tab 同区、按使用频次排布顶部功能，来源同上 js.design 文章的 PC 端分析，移动端底部导航为设计知识综述）
- 播放页是"**第二主场**"：从任意页面以底部 mini 播条 → 上滑全屏的方式到达，形成"列表页 ↔ 播放页"的双层世界结构。（设计知识综述）

**卡片/信息流**
- 首页近九成版面给推荐内容：Banner 大卡 + 歌单横向卡片流 + 场景推荐，卡片圆角大、封面图即视觉主体、文字退居辅助；菜单文字用灰色与图片区统一，"整个页面更有呼吸感"。（来源：js.design 文章）

**播放页沉浸式设计**
- 黑胶唱片机是全 App 的人格图腾：旋转唱片 + 唱针，"播放中"这个状态被具象成一件可以盯着的物体。（设计知识综述；第三方模仿佐证见 [Obsidian Vinyl Life 插件](https://community.obsidian.md/plugins/vinyl-life)）
- 触碰唱片手势：上滑回主页、下滑看评论、左右滑切歌——沉浸页自身承载一套完整手势词汇。（来源：js.design 文章）
- 2024 年"动态封面 Live Cover"：播放页全屏动态封面，由音乐人制作或 AI 生成，"用动起来方式呈现歌曲气质"。（来源：[知乎官方通稿转载](https://zhuanlan.zhihu.com/p/680945921)、[优设](https://www.uisdc.com/hunter/0221555420.html)、[新浪](https://news.sina.cn/sx/2024-02-06/detail-inahaeum9838614.d.html)）
- "神光模式"等播放器皮肤：模拟现场环境光、大屏歌词、烟雾镭射——沉浸是可购买/可切换的**图层**，不是固定皮肤。（搜索摘要，知乎专栏）

**动效**
- 封面旋转随播放启停、歌词平滑滚动、mini 条与全屏播放页之间的承接展开；学术研究亦以其"干净、用户友好"的界面与搜索/收藏流程为样本。（转场承接展开为设计知识综述；学术来源：[A Study of NetEase Cloud Music, MobileHCI '21](https://jc926.github.io/Jie_Cai/paper/mobilehci21adjunct-7.pdf)）

**字体层级**
- 界面以系统无衬线为主；**歌词页/播放页才有大字**（歌词大号、时间码大号等宽数字）——"大字只出现在情绪场景"是它的隐形纪律。（设计知识综述；js.design 文章未展开字体层级）

### 1.2 ChatGPT

**布局/会话**：居中输入框 + 左侧按时间分组的会话历史（今天/昨天/过去 7 天）；会话历史被当作**功能与记忆**而非导航元素，营造"产品记得你"。（来源：[ChatGPT Design Breakdown, 925 Studios](https://www.925studios.co/blog/chatgpt-interface-design-breakdown)）

**流式输出**：逐 token 流式不是性能优化而是**信任机制**——0.3 秒内首字上屏，降低感知延迟、暗示"在思考"；如今无流式的 AI 界面反而显得过时。（同上；架构层参考 [Streaming UI](https://medium.com/@agusetyawan/streaming-ui-the-architecture-behind-chatgpt-style-interfaces-33afd798f7d4)）

**输入栏/模型切换**：空状态即邀请（空白输入框 + 少量建议 chips，不塞新手教程）；模型选择器**按结果而非机制命名**（Auto / Fast / Thinking 而非型号名）；阅读行宽固定 ~65 字符/行。（来源同上；移动端逐屏拆解见 [screensdesign: ChatGPT Mobile App Design](https://screensdesign.com/articles/chatgpt-mobile-app-design/)）

**视觉**：激进克制（radical simplicity）——空白输入框、流式文本、极简 chrome 是默认态；官方生态设计系统 `@openai/apps-sdk-ui` 延续同一语言。（来源：925 Studios 文章 + [OpenAI UI Guidelines](https://developers.openai.com/plugins/concepts/ui-guidelines)）

**动效**：流式光标、发送/停止按钮状态切换、气泡淡入——动效全部服务于"模型正在工作"的感知。（设计知识综述 + 上述来源综合）

### 1.3 Claude

**色彩**：暖奶油画布（Pampas #F4F3EE）+ 陶土橙强调（Crail #C15F3C）+ 暖灰（Cloudy #B1ADA1），刻意反"冷科技风"。（来源：[Mobbin – Claude Brand Colors](https://mobbin.com/colors/brand/claude)）

**风格定位**："文学沙龙式"编辑版式——衬线大标题、暖色底、大量留白、克制 UI；与 PocketAgent 的编辑式（冷白画布 + 石墨重点面）**同属一个家族，只是冷暖之别**。（来源：[OpenDesign – Claude design system](https://open-design.ai/plugins/design-system-claude)、[The quiet genius of Claude's branding, Medium](https://medium.com/design-bootcamp/the-quiet-genius-of-claudes-branding-less-hype-more-humanity-f4f5567051cc)；"编辑式"类比为本报告判断）

**对本 App 的意义**：Claude 证明了"编辑式底子 + 一处暖/强记忆点"可以在 AI 产品里成立——这是方向 B/C 可行性的最硬外部佐证。

### 1.4 Gemini

**2025 大改版**：Material 3 Expressive 语系（fluid animations、vibrant colors、新字体、触感反馈）；2026 I/O 进一步发布 "Neural Expressive" 重设计，明确**抛弃"一堵文字墙"式的回答**，向卡片化、可交互的生成式 UI 走。（来源：[Google 官方](https://blog.google/innovation-and-ai/products/gemini-app/next-evolution-gemini-app)、[Dezeen](https://www.dezeen.com/2026/05/19/google-rolls-out-neural-expressive-redesign-of-gemini-ai-tool)、[Fast Company](https://www.fastcompany.com/91544692/google-gemini-ai-redesign)、[Android Authority 上手](https://www.androidauthority.com/gemini-neural-expressive-android-app-hands-on-3668985)）

**生成式 UI**：回答内按需生成富可视组件（卡片/图表/交互件），不再是纯 markdown。（来源：[Google Research: Generative UI](https://research.google/blog/generative-ui-a-rich-custom-visual-interactive-user-experience-for-any-prompt)）

**设计语言底座**：M3 Expressive 强调"彩色、情绪化、个性化"，同族组件共用弹簧参数——与 PaMotion 的分层弹簧纪律同构。（来源：[Material 3 Expressive 概述, Medium](https://medium.com/uxdworld/material-3-expressive-googles-new-direction-in-ui-design-286dc8517ef5)）

### 1.5 豆包 / Kimi

- 豆包：比竞品**信息更密、色彩更艳**的多模态界面（文本/语音/视频入口并存），工具栏丰富；反映"超级 App"路线——功能入口优先于留白。（来源：[WIRED](https://www.wired.com/story/bytedance-doubao-chatbot-popularity)、[ToolCenter 评测](https://www.toolcenter.ai/en/articles/doubao-review-2026)；均转述界面观感）
- Kimi：会话极简 + 长上下文/文件分析为卖点，界面语言接近 ChatGPT 一档。（来源：[kimi.com](https://www.kimi.com/en)；UI 细节一手文章未检索到，标注为限制）

> 两家 UI 细节的一手中文章节因搜索限流未取到，此处仅定调性；**豆包=密艳、Kimi=极简**，本 App 属工具属性，宜取 Kimi 一侧而非豆包。

### 1.6 效率/工具类标杆（可选项）

**Things 3**
- "Calm"哲学：多年视觉几乎不变的一致性；**不用红色制造焦虑**（逾期任务不变红）；FAB "+" 按钮向新建页的 morph 变形转场是行业经典。（来源：[Why I Use Things 3, Medium](https://medium.com/@woojingo/why-i-use-things-3-as-my-main-task-management-app-3295578b11c)；Calm 哲学与 morph 转场转引自 MacStories/60fps.design 的搜索摘要）

**滴答清单 TickTick**
- 反面参照：功能全、可定制强，但"design is functional, not beautiful"，卡片密、视觉语言弱——**高信息密度若不做层级，就会滑向 TickTick 观感**。（来源：[Reddit r/ticktick 设计讨论](https://www.reddit.com/r/ticktick/comments/1do0l03/tickticks_design)；"functional, not beautiful"出自 muratesmer.com 评测搜索摘要）

**Termius**（搜索限流未获一手文章，以下为设计知识综述）
- 深色优先、纯黑-石墨面阶 + 单一强调色；主机列表用强圆形主机头像做记忆锚；终端场景等宽字 + 高对比语法色。对本 App 终屏/Git 屏的启示：**深色面 + 头像锚点 + 等宽纪律**即可，不需要更多装饰。

---

## 二、可迁移模式清单

### 2.1 网易云音乐 → 本 App 可抄/改

| # | 网易云做法 | 迁移方式 | 落点（本 App 资产） |
|---|---|---|---|
| N1 | **双层世界**：列表页 ↔ 全屏沉浸播放页，mini 条常驻 | 会话页做成"播放页"：Threads/Home 的运行中会话以底部 mini 条常驻，上滑进入沉浸会话屏 | Chat 屏 + 新增 mini 会话条组件；转场用 PaMotion.sheet/emphasized 承接展开 |
| N2 | **沉浸式头部**：全幅封面 + 取色渐隐 + 信息压图 | 会话/任务详情顶部：agent/host 身份区做全幅石墨面头部（duotone 图形当"封面"），标题用 Type.Display/Title 大字 | Graphite/GraphiteDrawer 面 + duotone 图形 + Poppins Bold——**全部已有，零新增令牌** |
| N3 | **强调色收缩策略**：红从大面退到点缀位（主按钮/播放态/进度） | 不引入全局品牌红；把"运行态能量色"（Running/Primary）定为品牌记忆点，只出现在：运行中呼吸点、进度、当前会话标记、转场瞬间 | EditorialTone.RUNNING/PRIMARY 既有语义，扩大"出现场合"而非改色值 |
| N4 | **播放状态具象图腾**（旋转黑胶） | 给"agent 正在干活"一个可盯着看的具象件：duotone 图形 + 呼吸/旋转律动（循环节拍用 PaMotion.pulse/sweep，减弱动效退化） | HomeScreen 呼吸点已有雏形，升级为统一"机魂"组件 |
| N5 | **卡片圆角大台阶 + 封面即视觉主体** | Home 信息流重构：运行中会话/待审批任务升为**大卡**（Radius.Card 18，内部留白加大、图片/图形占比拉高），次级信息降为小卡/列表行；文字灰阶退后（Muted/Subtle） | Radius 六档不动，只改"谁用哪档"的分配 |
| N6 | **Tab/顶栏毛玻璃** | 底部导航与沉浸页顶栏用 paGlass(hairline=true)，浮层开合继续 paPushBack | PaGlass 既有，可能需给"贴边玻璃"微调 tint alpha |
| N7 | **Live Cover 动态封面** | 任务/会话卡在"运行中"状态启用微动效背景（duotone 图形律动），完成即静止——状态可感知 | PaMotion.pulse/sweep + 循环动效纪律 |
| N8 | **沉浸页自带手势词汇**（上下左右各有所指） | 沉浸会话页：下滑收起回列表、左右切会话；手势一律跟 PaMotion.momentum | 交互新增，动效令牌现成 |
| N9 | 封面取色环境化 | **慎抄**：按 host/agent 给 duotone 图形换色相可以做，但不得侵入语义色五件套与主色纪律（见 §3 冲突 3） | 可做实验性局部 |

### 2.2 AI 助手会话类标配（缺了显得过时的部分）

| # | 标配 | 迁移方式 | 落点 |
|---|---|---|---|
| A1 | **流式输出语言**：0.3s 首字、逐 token、光标、自动滚动 | Chat 流式呈现对齐此标准；流式 = 信任机制 | Chat 屏渲染层 |
| A2 | **输入栏浮起**：常驻底部、聚焦上浮、发送/停止双态 | 输入栏改为浮起玻璃条（paGlass + Pill 胶囊身），运行中变"停止"钮 | Chat 屏 + 输入栏组件 |
| A3 | **模型/Agent 胶囊**：composer 内 pill 选择器，按结果不按机制命名 | 选择哪个 agent/主机干活用胶囊置入输入栏，命名用任务语言（"让它修 Bug"）而非协议名 | 输入栏组件 + Radius.Pill |
| A4 | 会话即记忆：时间线分组（今天/昨天/7 天内） | Threads 列表按时间分组头（Caption 档灰字） | Threads 屏 |
| A5 | 空状态即邀请：空白输入框 + 2-4 个建议 chips | Chat 空态不放教程墙，放少量高频指令 chips | Chat 空态 |
| A6 | 思考过程可折叠面板 | agent 的计划/工具调用默认折叠成一行"正在执行 X…"，可展开 | Chat 消息组件（与语法高亮块同级） |
| A7 | 阅读行宽限制 ~65ch | Chat 正文限宽，拒绝通栏文字墙 | Chat 排版 |
| A8 | 回答卡片化（Gemini 生成式 UI 方向） | **远期**：任务结果/文件差异/Git 状态渲染为结构卡片而非纯文本 | 依赖消息协议，列入远期 |

### 2.3 效率类补充

- **Things 3 morph**：Home"发起任务"主按钮 → 任务创建/指令面板用容器变形转场（emphasized 弹簧），是低成本高级感。
- **Things 3 calm 纪律**：与既有"语义不单靠颜色 + 四词状态"纪律同向，坚持；DENIED 珊瑚只给"真失败"，不给"未读/未处理"。
- **TickTick 反面教材**：本 App 十屏高密度，重构时每屏只允许一个"大卡层级"，其余信息降级——否则改版等于把自己改成 TickTick。

---

## 三、冲突与本底（对标做法 vs 既有系统）

| # | 冲突 | 分析 | 取舍建议 |
|---|---|---|---|
| C1 | **网易云强品牌红 vs "主色只用于当前操作"+多 agent 无单一品牌色** | 本 App 服务 N 个 agent/主机，无单一"人格"可上红；全局红会污染五件套语义 | **不抄红色本身，抄"收缩策略"**（N3）：品牌记忆点=运行态能量色的出现纪律（呼吸点/进度/转场），而非色值。需接受：识别度靠"母题+节奏"而非色相 |
| C2 | **网易云深色沉浸 vs 本 App 浅色编辑式默认盘** | 全盘切深违反三盘同构与 700+ 处零改动换肤机制；但沉浸感需要暗面 | **沉浸面=Graphite 家族专属**：会话沉浸头部、代码箱、mini 条用既有 Graphite/GraphiteDrawer/GraphiteCode 三档深面，浅盘下也成立（本就是"深色重点面"语义）。立铁律：暗面只许用 Graphite 系，禁止各屏自调深色 |
| C3 | **封面取色环境化 vs 语义色纪律** | 按 agent 取色会制造"第五、六、七种语义色"，违反"状态必须带词"红线 | 限制在 duotone 图形/头像一类**装饰层**，永不进入文字、边框、按钮（N9 慎抄条款） |
| C4 | **网易云大圆角大卡 vs Radius 六档已收敛 + 信息密度高** | 新增 24/28dp 圆角档会破坏机械收敛；但 18dp 大卡配 23px 边距已够"大台阶" | 不加档。信息流层级靠**面积分配 + 留白 + 图形占比**做，不靠圆角加码 |
| C5 | **AI 助手极简 chrome vs 工具属性高密度** | Chat 屏可以极简，但文件/Git/终端屏不能照搬留白 | 分层双标：会话域（Home/Threads/Chat）用 AI 助手语言；工具域维持编辑式密度，仅统一到新母题 |
| C6 | **Things "无焦虑色" vs 本 App ATTENTION 琥珀** | 本 App 已区分 ATTENTION（待处理）与 DENIED（失败），与 Things 精神一致 | 不改；转场/徽章动效上给 ATTENTION 更"邀请"的节拍（pulse），DENIED 静止 |
| C7 | **Gemini M3 Expressive / 生成式 UI vs 依赖极简铁律** | 不引库；生成式 UI 依赖消息协议改造 | 只吸收其"同族组件同弹簧"哲学（PaMotion 已是）与回答卡片化的**远期**路线； expressive 手感用现有七档弹簧实现 |

---

## 四、设计方向提案（一页）

### 方向 A：网易云式沉浸深色信息流
- **内容**：深色优先（默认盘改深）、首页全幅大卡信息流、强品牌强调色、全屏沉浸转场。
- **改动面**：PaEditorial 默认盘与三盘策略重做；Home/Threads 全重构；新增品牌色体系；转场体系重建。
- **风险**：与编辑式浅盘基因、三盘同构机制、"主色只用于当前操作"纪律三面冲突；终端/代码/文件等生产力场景在浓妆下反而受损；改动量≈重写，周期与回归成本最高；多 agent 无品牌人格，"强品牌色"没有落点。
- **判定**：形似网易云，神不似——本 App 不是内容消费 App。

### 方向 B：编辑式守正 + 吸收 AI 助手会话语言
- **内容**：色彩/字体/圆角/动效全不动，只把 Chat/Threads 对齐 AI 助手标配（A1-A7：流式、输入栏浮起、胶囊、时间线、折叠思考、限宽）。
- **改动面**：集中在 Chat 屏 + 输入栏组件 + Threads 分组，其余九屏不动。
- **风险**：最小改动，但"看一眼记不住"只解决一半——会话屏变好，Home 依然是平面密度表；对用户"参考网易云"的诉求回应不足。
- **判定**：安全但欠账，适合作为 C 的第一阶段而非终点。

### 方向 C（推荐）：混合——"把会话页当播放页做"
- **内容**：编辑式骨架与六档/七档/八档令牌全部不动；选定**一个沉浸场景**（Chat=播放页、Home=信息流）做网易云式表达，其余屏跟随统一母题：
  1. **会话域**（Threads/Chat）：A1-A7 会话标配 + N1 双层世界（运行中会话 mini 条常驻、上滑沉浸）+ N2 石墨沉浸头部 + N4 机魂图腾；
  2. **工作台**（Home）：N5 大卡台阶（运行中/待审批升大卡，其余降列表行）+ N7 运行态动态背景 + Things 3 式发起按钮 morph；
  3. **全局**：N3 运行态能量色出现纪律 + N6 玻璃 Tab/顶栏 + C2"暗面=Graphite 专属"铁律。
- **改动面**：Home 信息流层级、Chat 沉浸头部与输入栏、mini 会话条与承接转场、机魂组件一个；PaSheet 模态家族顺势接入沉浸转场。**零新增设计令牌档位**（这是硬约束，也是卖点）。
- **风险**：编辑式浅面与石墨暗面并存，若不立"C2 铁律"会漂移出第三种黑；mini 条与底部导航抢空间（可让 mini 条替换导航上方常驻位，网易云同款布局）；机魂图腾若做成重型动画会踩性能与减弱动效红线（循环节拍必须走 pulse/sweep + rememberReduceMotion 退化）。
- **推荐理由**：用户要的是网易云的**记忆点与沉浸感**，不是它的音乐信息架构；本 App 的三盘同构、700+ 令牌消费点、生产力密度决定了推倒重来（A）性价比最低、纯守成（B）解决不了核心短板。C 用"一个场景做透 + 母题全局统一"的结构，复用 Graphite/PaGlass/PaMotion/duotone 全部既有资产，以最小结构改动换最大感知提升——且 Claude（编辑式+记忆点成立）与 Gemini（抛弃文字墙）两条外部证据都指向同一方向。

---

## 附：来源清单

**网易云音乐**
- https://js.design/special/article/netease-cloud-ui-design.html （红白黑体系、首页信息流、唱片手势）
- https://zhuanlan.zhihu.com/p/680945921 、https://www.uisdc.com/hunter/0221555420.html 、https://news.sina.cn/sx/2024-02-06/detail-inahaeum9838614.d.html （Live Cover 动态封面）
- https://blog.csdn.net/gitblog_00134/article/details/159783695 （沉浸播放界面、封面取色）
- https://en.wikipedia.org/wiki/NetEase_Cloud_Music 、https://baike.baidu.com/en/item/NetEase%20CloudMusic/936391 （黑胶 VIP、DIY 播放器）
- https://jc926.github.io/Jie_Cai/paper/mobilehci21adjunct-7.pdf （MobileHCI '21 学术研究）

**AI 助手**
- https://www.925studios.co/blog/chatgpt-interface-design-breakdown （ChatGPT 拆解：流式/空态/模型命名/65ch）
- https://screensdesign.com/articles/chatgpt-mobile-app-design/ （ChatGPT 移动端逐屏）
- https://developers.openai.com/plugins/concepts/ui-guidelines （apps-sdk-ui）
- https://medium.com/@agusetyawan/streaming-ui-the-architecture-behind-chatgpt-style-interfaces-33afd798f7d4 （流式架构）
- https://mobbin.com/colors/brand/claude 、https://open-design.ai/plugins/design-system-claude 、https://medium.com/design-bootcamp/the-quiet-genius-of-claudes-branding-less-hype-more-humanity-f4f5567051cc （Claude 色彩与风格）
- https://blog.google/innovation-and-ai/products/gemini-app/next-evolution-gemini-app 、https://www.dezeen.com/2026/05/19/google-rolls-out-neural-expressive-redesign-of-gemini-ai-tool 、https://www.fastcompany.com/91544692/google-gemini-ai-redesign 、https://www.androidauthority.com/gemini-neural-expressive-android-app-hands-on-3668985 、https://research.google/blog/generative-ui-a-rich-custom-visual-interactive-user-experience-for-any-prompt 、https://medium.com/uxdworld/material-3-expressive-googles-new-direction-in-ui-design-286dc8517ef5 （Gemini/M3 Expressive/生成式 UI）
- https://www.wired.com/story/bytedance-doubao-chatbot-popularity 、https://www.toolcenter.ai/en/articles/doubao-review-2026 、https://www.kimi.com/en （豆包/Kimi 定调）

**效率/工具**
- https://medium.com/@woojingo/why-i-use-things-3-as-my-main-task-management-app-3295578b11c 、https://www.reddit.com/r/ticktick/comments/1do0l03/tickticks_design （Things 3 / TickTick；MacStories、60fps.design、muratesmer.com 观点转引自搜索摘要）

**本 App**
- `android/app/src/main/java/dev/pocketagent/app/ui/theme/PaEditorial.kt`、`ui/anim/PaMotion.kt`、`ui/theme/PaGlass.kt`
- `docs/design/2026-09-29-pocketagent-ui/`（既有编辑式原型权威稿）

**调研限制**：搜索服务间歇限流；Termius 无一手文章（以设计知识综述补位）；豆包/Kimi 仅得定调性来源；网易云音乐底部导航细节与部分转场动效为设计知识综述。上述均已在正文就地标注。

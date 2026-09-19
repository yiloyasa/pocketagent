# Pi get_commands 斜杠命令探针记录

> 生成:2026-09-19 12:12:46 工具:daemon/cmd/piprobe(直接 node 拉起 cli.js --mode rpc,Go 手写 JSONL 分帧)

结果:**2 通过 / 0 替代 / 0 失败**(共 2 项)

## 1. get_commands(空会话) —— 通过

应答 data 形如 {commands:[{name,description,source,sourceInfo}]},source ∈ extension/prompt/skill;skill 以 "skill:<名>" 形态出现
```
{"commands":[{"description":"Open web search curator","name":"websearch","source":"extension","sourceInfo":{"baseDir":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access","origin":"package","path":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access\\index.ts","scope":"user","source":"npm:pi-web-access"}},{"description":"Toggle or configure the search curator workflow","name":"curator","source":"extension","sourceInfo":{"baseDir":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access","origin":"package","path":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access\\index.ts","scope":"user","source":"npm:pi-web-access"}},{"description":"Show the active Google account for Gemini Web","name":"google-account","source":"extension","sourceInfo":{"baseDir":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access","origin":"package","path":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access\\index.ts","scope":"user","source":"npm:pi-web-access"}},{"description":"Browse stored web search results","name":"search","source":"extension","sourceInfo":{"baseDir":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access","origin":"p…
```

## 2. 命令清单(source/名/描述) —— 通过

共 56 条
```
extension	websearch	Open web search curator
extension	curator	Toggle or configure the search curator workflow
extension	google-account	Show the active Google account for Gemini Web
extension	search	Browse stored web search results
extension	subagents-watchdog	Show or toggle the default-off subagent watchdog
extension	subagents	Administer subagents: inspect metadata and update models, thinking, or prompts
extension	run	Run one subagent through workflowScript: /run agent[output=file] [task] [--bg] […
extension	subagent-cost	Show parent and subagent child usage cost for this session
extension	subagents-doctor	Show subagent diagnostics
extension	subagents-inspect-rpc	Host integration bridge: answer an async child inspection request with a correla…
extension	subagents-guide	Show a packaged subagents guide topic
extension	subagents-refine	Generate a bounded project-local refinement overlay for one subagent
extension	subagents-fleet	Open the live subagent fleet inspector
extension	subagents-detach	Detach the active foreground single-subagent run without terminating it
extension	subagents-stop	Stop a current-session async subagent run, or one child of it with /subagents-st…
extension	subagents-steer	Steer a live async subagent run with a message, or one child of it with --child …
extension	prompt-workflow	Run a prompt template through native pi-subagents: /prompt-workflow <name> [args…
extension	subagents-models	Show model mappings for discovered subagents
extension	subagents-profiles	List saved subagent profiles
extension	subagents-load-profile	Load a subagent profile into ~/.pi/agent/settings.json
extension	subagents-refresh-provider-models	Refresh the cached model catalog for one provider
extension	subagents-generate-profiles	Generate <provider>.quota and <provider>.quality subagent profiles
extension	subagents-check-profile	Check whether a saved profile still points to usable models
extension	memory-consolidate	Manually trigger memory consolidation to free up space
extension	memory-insights	Show what's stored in persistent memory
extension	memory-skills	Manage global, active-project, and loaded external procedural skills
extension	memory-interview	Answer a few questions to pre-fill your user profile so the agent remembers you …
extension	memory-switch-project	Switch the active project for project-scoped memory
extension	learn-memory-tool	Learn how to use the pi-hermes-memory extension effectively
extension	memory-sync-markdown	Reconcile the SQLite search mirror with Markdown memories
extension	memory-preview-context	Preview the memory policy or legacy memory context blocks
extension	memory-pin	Pin a standing instruction that is injected into every session
extension	memory-index-sessions	Import past Pi sessions into the search database
extension	lens-toggle	Toggle pi-lens on/off for the current session. Usage: /lens-toggle
extension	lens-context-toggle	Toggle automatic context injection on/off for the current session (tools/LSP/rea…
extension	lens-widget-toggle	Show or hide the pi-lens diagnostics widget below the editor. Usage: /lens-widge…
extension	lens-tdi	Show Technical Debt Index (TDI) and project health trend. Usage: /lens-tdi
extension	lens-map	Render the review graph as a self-contained interactive HTML project map (pan/zo…
extension	lens-health	Show pi-lens runtime health: pipeline crashes, slow runners, and last dispatch l…
extension	lens-perf	Show the slowest latency-log phases by p50 and p99 for the current process sessi…
extension	lens-tools	Show pi-lens tool installation status: globally installed, auto-installed, or np…
extension	lens-allow-edit	Allow one edit to a file without a prior read. Usage: /lens-allow-edit <path>
extension	llama	Manage llama.cpp router models
prompt	council	Run a bounded supervisor-mediated council of advisors and write a decision memo
prompt	gather-context-and-clarify	Use subagents to gather context, then ask clarifying questions
prompt	parallel-cleanup	Parallel cleanup review
prompt	parallel-research	Parallel subagents research
prompt	parallel-review	Parallel subagents review
prompt	review-loop	Review/fix loop until clean
skill	skill:mmx-cli	Use mmx to generate text, images, video, speech, and music via the MiniMax AI pl…
skill	skill:council-mode	Run a bounded supervisor-mediated advisor council. Use when the user asks for co…
skill	skill:pi-subagents	Technical guidance for operator-requested delegation to builtin or custom
subage…
skill	skill:pi-lens-ast-grep	Use when searching or replacing code patterns - use ast-grep instead of text sea…
skill	skill:pi-lens-lsp-navigation	Navigate code with IDE features and run proactive LSP diagnostics on files/folde…
skill	skill:pi-lens-write-ast-grep-rule	Use when writing a new pi-lens ast-grep rule YAML file — covers schema, drop pat…
skill	skill:pi-lens-write-tree-sitter-rule	Use when writing a new pi-lens tree-sitter query rule YAML file — covers schema,…
```

## 原始报文附录(全部,截断)

```
{"type":"extension_ui_request","id":"0aa6682d-6dfb-415f-af02-17bf101423e2","method":"setWidget","widgetKey":"subagent-async"}
{"type":"extension_ui_request","id":"a44a0181-5d3e-49c4-8243-38e5d8bcd607","method":"setStatus","statusKey":"pi-lens-lsp","statusText":"\u001b[38;2;102;102;102mLSP Inactive\u001b[39m"}
{"id":"piprobe-1","type":"response","command":"get_state","success":true,"data":{"model":{"id":"glm-5.3","name":"GLM-5.3","api":"openai-completions","provider":"zai-coding-cn","baseUrl":"https://open.bigmodel.cn/api/coding/paas/v4","reasoning":true,"thinkingLevelMap":{"off":null,"minimal":null,"low":"low","medium":null,"high":"high","xhigh":null,"max":"max"},"input":["text"],"cost":{"input":1.4,"output":4.4,"cacheRead":0.26,"cacheWrite":0},"compat":{"supportsStore":false,"supportsDeveloperRole":false,"supportsReasoningEffort":true,"maxTokensField":"max_tokens","thinkingFormat":"zai","zaiToolSt …[截断,整行 1086 字节]
{"id":"piprobe-2","type":"response","command":"get_commands","success":true,"data":{"commands":[{"name":"websearch","description":"Open web search curator","source":"extension","sourceInfo":{"path":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access\\index.ts","source":"npm:pi-web-access","scope":"user","origin":"package","baseDir":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access"}},{"name":"curator","description":"Toggle or configure the search curator workflow","source":"extension","sourceInfo":{"path":"C:\\Users\\yanzi\\.pi\\agent\\npm\\node_modules\\pi-web-access …[截断,整行 21245 字节]
[stderr] 'npm' �����ڲ����ⲿ���Ҳ���ǿ����еĳ���
[stderr] ���������ļ���
```

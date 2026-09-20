# Vosk 摘除测试用例删除台账（批5 T-22 · 2026-09-20）

> AUDIT §4"审核资产只增不减"登记：随 Vosk 引擎整体摘除（T-22），`VoskVoiceFallbackTest.kt`（24 例）删除，
> 由新 `SenseVoiceFallbackTest.kt`（24 例）同数承接引擎回退/错误码钉子，全仓用例数 484 持平不减。
> 本文件为被删 21 例的逐例清单（批 6 审核提示项-2 的 24h 补录）。被测对象均已随引擎删除，无行为可钉。

## 被删用例清单（21 例，原 VoskVoiceFallbackTest.kt）

| # | 用例（语义） | 类别 | 承接说明 |
|---|---|---|---|
| 1 | 下载进度折算 | VoskModelManager 状态机 | 无承接：Vosk 模型下载机制已随 SenseVoice 打包内置消亡 |
| 2 | 乱序事件 | 同上 | 无承接（同上） |
| 3 | 解压满格 | 同上 | 无承接（同上） |
| 4 | 失败重试 | 同上 | 无承接（同上） |
| 5 | 磁盘刷新 | 同上 | 无承接（同上） |
| 6 | 半损坏重下 | 同上 | 无承接（同上） |
| 7 | 模型文件三件套校验 | 文件校验/解压 | 行为由 SenseVoiceModelFilesTest 覆盖（SenseVoice 三件套校验） |
| 8 | 顶层目录展开 | 文件校验/解压 | 无承接：Vosk zip 布局专属 |
| 9 | zip-slip 防护 | 文件校验/解压 | 无承接：Vosk 解压器已删；SenseVoice 走 assets 打包不解压 zip |
| 10 | 安装全通事件序 | 安装编排 | 无承接（Vosk 安装器已删） |
| 11 | 安装下载失败 | 安装编排 | 无承接（同上） |
| 12 | 安装缺文件 | 安装编排 | 无承接（同上） |
| 13 | sha256 测试向量 | 安装编排（哈希） | 无承接（同上） |
| 14 | 哈希匹配 | 安装编排（哈希） | 无承接（同上） |
| 15 | 哈希不匹配 | 安装编排（哈希） | 无承接（同上） |
| 16 | 空哈希跳过 | 安装编排（哈希） | 无承接（同上） |
| 17 | 识别循环自然结束 | 识别循环 | 行为由 SenseVoiceSessionTest 覆盖（收音结束/alive 喂狗） |
| 18 | 识别取消冲刷 | 识别循环 | 行为由 SenseVoiceSessionTest/ChatScreen 语音段测试覆盖 |
| 19 | 读块异常 | 识别循环 | 行为由 SenseVoice 链路 catch→error 事件测试覆盖 |
| 20 | JSON 提取 | 识别循环 | 无承接：Vosk JSON 输出格式专属 |
| 21 | 磁盘复核回连 | 死码清理 | voiceModelRefreshNeeded 在摘除前即不可触发（101 不可产生），纯死码 |

另：错误码映射旧例 1 例由新 SenseVoiceFallbackTest 的"101 落 OTHER 兜底""常量值迁移保真 100/102/103""提示禁显未知/错误"三钉语义承接（批 6 审核独立复核确认）。

## 对冲新增（同批）

- SenseVoiceFallbackTest.kt 24 例（引擎回退钉子/错误码映射/死区分流/常量保真/§3.3 禁显）
- codexapp client_debug_ast_test.go（批 7 T-24，AST 回归钉子 +1）

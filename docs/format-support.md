# 文件浏览与下载:格式支持矩阵

> 结论先行:**判断"能不能预览"的唯一来源是 `ui/component/FileKind.kt` 里的 `preview` 字段**
> (`PreviewMode.IN_APP` / `SYSTEM` / `SAVE_ONLY`)。任务中心、用户数据管理、文件浏览器、
> 内置浏览器、下载分派全部读这一份判定,不允许任何地方再写第二份后缀清单。

## 0. 下载在哪点

| 位置 | 动作 |
| --- | --- |
| 会话回复里的文件名 | 「在浏览器打开 `xxx`」(只展示、只打开,不放下载按钮) |
| 内置浏览器顶栏 | **浏览电脑文件时出现蓝色「下载到手机」按钮**(pc 文件的唯一下载入口) |
| 内置浏览器顶栏 | 浏览手机文件时出现「分享」按钮 |
| 文件浏览器每一行 | 右侧常驻「下载到手机」图标(不必先进浏览器) |
| 文件浏览器点文件 | 可预览的直接进内置浏览器;不可预览的弹「下载到手机」引导 |
| 任务中心 → 文件下载 | 卡片本体 → 用户数据管理;条目右侧查看 / 分享 / 删除 |

下载统一走 `PaDownloader.downloadFromPc()` → daemon `ReadFile` 分块流 → `MediaStore.Downloads`
(落 `下载/PocketAgent/`),单文件上限 512MB,前台服务保活,进度实时上报任务中心。

---

## 1. 必须适配(已内置预览 + 可下载)

这些是 agent 产出与日常办公的主力格式,必须在 App 内就能读,**不依赖手机上装任何 App**。

| 格式 | 后缀 | 预览 | 实现方式 |
| --- | --- | --- | --- |
| Markdown | `.md .markdown .mdx` | ✅ 排版渲染 | 自研极简 Markdown→HTML(标题/列表/引用/代码块/链接) |
| 纯文本 / 源码 | `.txt .log .json .xml .yaml .toml .ini .sh .kt .go .py .js .ts .css .sql …` | ✅ 等宽 | `<pre>` 保留空白 |
| 结构化数据 | `.json .jsonl .xml` | ✅ | 等宽 + 高亮底色 |
| 网页 | `.html .htm .xhtml .mhtml` | ✅ | WebView 原生渲染 |
| 表格(分隔符) | `.csv .tsv` | ✅ 真表格 | 自研带引号转义的切分 |
| 图片 | `.png .jpg .jpeg .gif .webp .bmp .svg .ico .heic .avif` | ✅ | 内联 data URL |
| Excel(新) | `.xlsx .xlsm` | ✅ 多表 | `ZipInputStream` + `XmlPullParser` 解 sharedStrings / sheetN |
| Excel(旧) | `.xls` | ✅ 多表 | **自研 OLE2(CFB)+ BIFF8 解析器**(`XlsLegacy.kt`):SST 跨 CONTINUE、RK/NUMBER/MULRK/LABELSST/FORMULA、XF 日期格式识别 |
| OpenDocument 表格 | `.ods` | ✅ | 解 `content.xml` 的 `table:table` |
| Word(新) | `.docx .docm` | ✅ 段落 + 表格 | 解 `word/document.xml` |
| Word(RTF) | `.rtf` | ✅ 富文本段落 | 剥离控制字与目标组 |
| OpenDocument 文本 | `.odt` | ✅ | 解 `content.xml` 的 `text:*` |
| PowerPoint(新) | `.pptx .pptm` | ✅ 文字大纲 | 解 `ppt/slides/slideN.xml`,按占位符类型区分标题/要点 |
| OpenDocument 演示 | `.odp` | ✅ 文字大纲 | 解 `content.xml` 的 `draw:page` |
| PDF | `.pdf` | ✅ 逐页位图 | **系统自带 `PdfRenderer`** 逐页转 JPEG 内嵌,不需要装阅读器 |

**已知边界(有意为之,不是 bug)**

- Office 只还原**内容与结构**,不还原版式:字体、颜色、页眉页脚、批注、图表、合并单元格、
  条件格式、公式重算都忽略。旧版 `.xls` 的公式显示为缓存结果。
- 大文件保护:表格最多 800 行 × 40 列 × 3 张表;PDF 最多渲染前 12 页(1000px 宽);
  PPTX 最多 40 页。超出部分页面底部会明确标注。
- 内置预览单次读取上限 **64MB**(此前传 0 会被服务端默认值静默截断成 2MB —— 这是
  "大 xlsx 显示表格里没有可显示的数据"的根因,已修)。

---

## 2. 需要适配但只能交给系统(可下载,内置不做)

| 格式 | 后缀 | 预览 | 原因 | 后续计划 |
| --- | --- | --- | --- | --- |
| Word 97-2003 | `.doc .dot` | 下载后用系统/WPS 打开 | OLE 复合文档里的 WordDocument 二进制流,要完整实现 FIB + 分片表(piece table)才能取到正文,工程量约等于再写一个 `XlsLegacy` 的 2 倍,收益低 | 若后续用户反馈频繁,先做"粗提取纯文本 + 明确标注可能不完整"的降级版 |
| PowerPoint 97-2003 | `.ppt` | 下载后用系统/WPS 打开 | 同为 OLE 容器内的 PowerPoint 记录流,版式信息强耦合,纯文本提取价值有限 | 同上,优先级低于 `.doc` |
| 安装包 | `.apk .apks .xapk` | 交系统包安装器 | 安全策略:不在 App 内解析/安装,必须由系统安装器接管并弹权限确认 | 保持 |
| 音视频 | `.mp3 .wav .flac .m4a .mp4 .mkv .mov …` | 交系统播放器 | 需要解码器 + 大体量流式播放,内置播放器属于另一个产品方向 | 保持 |
| 未知二进制 | `.bin .dat .db .sqlite .so .dll .psd .ttf …` | 无预览,只保存 | 没有通用展示语义 | 保持(会明确提示"已保存到…") |

> `.doc` / `.ppt` **可以下载**,下载后在用户数据管理里点「查看」会走 `ACTION_VIEW` 交系统应用;
> 手机没装能打开的应用时会提示"手机上还没有能打开 X 的应用 + 已保存到…",不会静默失败。

---

## 3. 只保存不预览(明确不做)

| 格式 | 后缀 | 原因 |
| --- | --- | --- |
| 压缩包 | `.zip .rar .7z .tar .gz .xz .jar` | 容器格式,合理交互是"列出条目 + 选择解压",属于文件管理器范畴;当前只保证下载 |
| 可执行 / 库 | `.exe .dll .so .dylib` | 非阅读内容,且有安全含义 |

---

## 4. 下载侧的适配要点

- **MIME 必须按文件名给**:之前 `saveTextAsFile()` 硬编码 `text/plain`,MediaStore 会据此把
  `会话.md` 存成 `会话.md.txt`。现在统一走 `FileKind.mime`。
- **持久化下载记录**:`TaskCenter` 持久化补齐了 `localUri / sourceTitle / sizeText`,
  否则进程被杀后条目还在、但"查看/分享"图标会消失(旧版本下载的历史条目就是这个表现,
  重新下载一次即恢复)。
- **分享**:`PaDownloader.shareLocalFile()` 带上按 `FileKind` 判定的 MIME;低版本落到
  `file://` 时经 `FileProvider`(`${applicationId}.fileprovider`,白名单见 `res/xml/file_paths.xml`)授权。
- **打开本地文件**按能力矩阵三态分派:能内置 → 内置浏览器;需系统 → `ACTION_VIEW`;
  都不行 → 提示保存位置。

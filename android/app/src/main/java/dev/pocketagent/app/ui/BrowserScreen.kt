package dev.pocketagent.app.ui

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Bitmap
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.component.DocFormatRegistry
import dev.pocketagent.app.ui.component.OfflineStore
import dev.pocketagent.app.ui.component.PaDownloader
import dev.pocketagent.app.ui.component.TaskCenter
import dev.pocketagent.app.ui.component.WebDownloadBridge
import dev.pocketagent.app.ui.component.WebNavPolicy
import dev.pocketagent.app.ui.component.humanSize
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaHacker
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.component.PaNavIconButton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import pocketagent.v1.DevServer
import pocketagent.v1.ListDevServersRequest
import pocketagent.v1.ReadFileRequest

/**
 * 内置浏览器。
 *
 * 【能打开四类东西】
 *  1. 公网地址 —— 正常 http(s) 加载;
 *  2. **电脑上的文件**(`pc:D:\...\index.html`)—— 走 daemon ReadFile 取回后渲染:
 *     html 直接渲染;xlsx/docx/csv 由 [PcFilePreview] 解析成表格/文档;md 渲染成排版页;
 *     图片转 data URL;其余文本等宽显示;二进制给提示页;
 *  3. **手机上的文件**(`content://…`)—— 任务中心/用户数据管理里点"查看"进来的
 *     已下载文件,经 ContentResolver 读取后走同一条渲染管线;
 *  4. 空地址 —— 内置起始页。
 *
 * 【视图切换不空白】本地渲染的页面缓存原始 HTML(lastLocal),手机/桌面切换时
 * 就地按新视图重排,不再回电脑重读;UA 用捕获到的默认值显式还原(置 null 在
 * 部分 WebView 版本上会让页面加载异常 → 切回手机视图白屏)。
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun BrowserScreen(initialUrl: String = "", onBack: () -> Unit = {}) {
    // 【色板并轨 2026-10-09】编辑式 M3 色板已上提 MainActivity NavHost 外层包全 app,
    // 本页不再重复包裹(editorialM3Scheme 定义也已上提 theme 层)
    EditorialSystemBars()
    BrowserScreenBody(initialUrl, onBack)
}

@SuppressLint("SetJavaScriptEnabled")
@Composable
private fun BrowserScreenBody(initialUrl: String = "", onBack: () -> Unit = {}) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var input by remember { mutableStateOf(stripPcMeta(initialUrl)) }
    var progress by remember { mutableFloatStateOf(0f) }
    var pageTitle by remember { mutableStateOf("") }
    var canGoBack by remember { mutableStateOf(false) }
    var canGoForward by remember { mutableStateOf(false) }
    var loadingPc by remember { mutableStateOf(false) }
    var webRef by remember { mutableStateOf<WebView?>(null) }
    // 手机视图(默认,注入 viewport + 字号保障) / 桌面视图(原样渲染,配桌面 UA)
    var desktopView by remember { mutableStateOf(false) }
    // 本地渲染内容的缓存:rawHtml to label。视图切换时用它就地重排(不回源重读)。
    var lastLocal by remember { mutableStateOf<Pair<String, String>?>(null) }
    // WebView 出厂 UA:切回手机视图时显式还原(见文件头注释)
    var defaultUa by remember { mutableStateOf("") }
    // 【当前正在看的本地文件】—— 顶栏的"下载到手机/分享"要据此决定动作:
    //   pcPath 非空 = 正在看电脑上的文件(可下载);
    //   localUri 非空 = 正在看手机上的文件(已在本地,可分享)。
    var pcPath by remember { mutableStateOf("") }
    var localUri by remember { mutableStateOf("") }
    // 【脚本桥的注册状态】只对可信来源(App 预览 / 本地文件 / 私网 daemon)挂载
    // WebDownloadBridge;公网站点一律卸载 —— 否则任意页面一行 JS 就能落盘任意文件。
    // 初值按【初始 URL】判定:pc:/content:/空起始页都是 App 预览(可信),
    // 直接以公网 http(s) 开场的则初值为 false,与 factory 里的注册保持一致。
    var bridgeRegistered by remember {
        mutableStateOf(!(initialUrl.startsWith("http://") || initialUrl.startsWith("https://")))
    }
    // ---- 电脑 dev server 面板(P2,对齐原作 BrowserService.ListDevServers)----
    var showServers by remember { mutableStateOf(false) }
    var scanning by remember { mutableStateOf(false) }
    var servers by remember { mutableStateOf<List<DevServer>>(emptyList()) }
    var scanHint by remember { mutableStateOf<String?>(null) }

    /**
     * 按当前页面来源同步脚本桥的挂载/卸载。
     * 只在来源可信(image/data/about、file/content、私网 http(s))时保留桥;
     * 状态未变化时直接返回,避免每个页面都重复 add/remove。
     */
    fun applyBridgePolicy(w: WebView?, url: String?) {
        if (w == null) return
        val trusted = WebNavPolicy.trustedBridgeSource(url.orEmpty())
        if (trusted == bridgeRegistered) return
        if (trusted) {
            w.addJavascriptInterface(WebDownloadBridge(ctx), WebDownloadBridge.JS_NAME)
        } else {
            w.removeJavascriptInterface(WebDownloadBridge.JS_NAME)
        }
        bridgeRegistered = trusted
    }

    // 【返回层级 · 分级回退】页面内还有历史 → 先网页后退;退无可退 → 才离开浏览器。
    // 之前没有 BackHandler,系统返回键会直接 pop 掉整个路由,导致用户
    // 在一个网页里点了两个链接后按返回,期望回上一页却直接回到了会话。
    BackHandler(enabled = canGoBack) { webRef?.goBack() }

    /** 把 HTML 直接喂给 WebView(不经过网络);手机视图下做移动端适配。 */
    fun renderLocal(html: String, label: String) {
        pageTitle = label
        progress = 1f
        lastLocal = html to label
        val body = if (desktopView) html else adaptForMobile(html)
        webRef?.loadDataWithBaseURL(null, body, "text/html", "utf-8", null)
    }

    /** 按类型把字节渲染成页面(电脑文件与手机本地文件共用一条管线)。
     *  @param mime content:// 的真实 MIME(可为空)。**优先按 MIME 判定**:
     *  MediaStore 会给下载文件按 MIME 补后缀(md 常被存成 `xxx.md.txt`),
     *  只看后缀会把 Markdown 当纯文本渲染。 */
    suspend fun renderBytes(name: String, bytes: ByteArray, mime: String = "") {
        // 【P0-5:解析必须离开主线程】markdownHtml/PcFilePreview.render 是重 CPU 同步解析
        // (xlsx ≤400 条 zip 流、docx XmlPullParser、pdf 逐页 1000px 位图),几 MB 的文件在
        // 主线程跑就是数百 ms 至数秒的冻结。主线程只取齐主题/来源快照,解析整段切
        // Dispatchers.Default,回 Main 后仅剩 WebView 加载(renderLocal)。
        // 线程安全性:PcFilePreview 对 Context 的使用只有 ctx.cacheDir(线程安全的缓存目录
        // getter);PdfRenderer 逐页渲染/Bitmap.compress 在单个后台线程串行执行是官方推荐姿势。
        // 主题/来源态在切线程前于主线程取快照,后台段不读任何 Compose 状态。
        val dark = PaEditorial.IsDark
        val pageBg = pal().bg
        val isPc = pcPath.isNotEmpty()
        val html = withContext(Dispatchers.Default) {
            buildPreviewHtml(ctx, name, bytes, mime, dark, pageBg, isPc)
        }
        renderLocal(html, name)
    }

    /**
     * 读电脑上的文件并渲染。
     *
     * @param cacheSize/cacheMtime 来源目录条目的 size/mtime(FilesScreen 进入时携带)。
     * 有元数据 → 先查[预览内容缓存][dev.pocketagent.app.ui.component.PreviewCache],
     * 命中免传输(离线时同样可用);未命中则拉取后回填。无元数据(消息里的裸路径)不走缓存 —— 宁可慢,不能脏。
     */
    fun openPcFile(path: String, cacheSize: Long = -1L, cacheMtime: Long = -1L) {
        input = path
        loadingPc = true
        pcPath = path
        localUri = ""
        pageTitle = path.substringAfterLast('/').substringAfterLast('\\')
        scope.launch {
            try {
                if (cacheSize >= 0 && cacheMtime >= 0) {
                    val cached = withContext(Dispatchers.IO) {
                        dev.pocketagent.app.ui.component.PreviewCache.get(path, cacheSize, cacheMtime)
                    }
                    if (cached != null) {
                        renderBytes(pageTitle, cached)
                        return@launch
                    }
                }
                val bytes = withContext(Dispatchers.IO) {
                    // 【预览走下载专用 stub】fileService 的 15s deadline 套在整条流上,
                    // 64MB 上限的大文件预览必然 DEADLINE_EXCEEDED;fileDownloadService
                    // 整流 10 分钟上界(与 PaDownloader 同款,通道同源仅 deadline 语义不同)。
                    val stream = AgentConnection.fileDownloadService().readFile(
                        // 【必须显式给上限】max_bytes=0 表示"用服务端默认值",而默认只有 2MB ——
                        // 大一点的 xlsx/docx/pdf 会被静默截断,解析器拿到的就是半截 zip,
                        // 表现成"表格里没有可显示的数据"。这里按预览需要给足 64MB。
                        ReadFileRequest.newBuilder()
                            .setPath(path)
                            .setMaxBytes(MAX_PREVIEW_BYTES)
                            .build(),
                    )
                    val buf = java.io.ByteArrayOutputStream()
                    while (stream.hasNext()) buf.write(stream.next().data.toByteArray())
                    buf.toByteArray()
                }
                // 回填预览缓存:字节数与元数据一致才写(不一致说明元数据过期,宁可不缓存)
                if (cacheSize >= 0 && cacheMtime >= 0 && bytes.size.toLong() == cacheSize) {
                    withContext(Dispatchers.IO) {
                        dev.pocketagent.app.ui.component.PreviewCache.put(path, cacheSize, cacheMtime, bytes)
                    }
                }
                renderBytes(pageTitle, bytes)
            } catch (e: Exception) {
                // 文件不存在/读不到:给可读的失败页,而不是继续白着
                renderLocal(errorPage(path, e.message ?: "读取失败"), "打开失败")
            } finally {
                loadingPc = false
            }
        }
    }

    /**
     * pc: 地址统一入口:`pc:<path>[?_pas=<size>&_pam=<mtime>]`。
     * 元数据由 FilesScreen 从目录条目带上(Windows 文件名禁止 ?,无歧义),
     * 供预览缓存做精确失效;没有就按裸路径打开。
     */
    fun openPcUrl(url: String) {
        val raw = url.removePrefix("pc:")
        val idx = raw.indexOf("?_pas=")
        if (idx < 0) {
            openPcFile(raw)
            return
        }
        val path = raw.substring(0, idx)
        val meta = raw.substring(idx + "?_pas=".length)
        val size = meta.substringBefore('&').toLongOrNull() ?: -1L
        val mtime = meta.substringAfter("_pam=", "").toLongOrNull() ?: -1L
        openPcFile(path, size, mtime)
    }

    /** 打开已下载到手机的文件(content://),与电脑文件同一条渲染管线。 */
    fun openLocalUri(uri: String) {
        input = uri
        loadingPc = true
        localUri = uri
        pcPath = ""
        // 【文件名来源】优先用任务中心那条下载记录的原文件名(用户看到的那个名字),
        // 其次 ContentResolver 的 DISPLAY_NAME,最后才用 uri 末段。
        // 名称 + MIME 共同决定渲染分支(md/xlsx/docx…),拿不到后缀就会退化成纯文本。
        var name = TaskCenter.tasks.value.firstOrNull { it.localUri == uri }?.title.orEmpty()
        if (name.isBlank()) name = uri.substringAfterLast('/')
        val mime = runCatching { ctx.contentResolver.getType(android.net.Uri.parse(uri)) }
            .getOrNull().orEmpty()
        if (!name.contains('.')) {
            runCatching {
                ctx.contentResolver.query(android.net.Uri.parse(uri), null, null, null, null)?.use { c ->
                    val idx = c.getColumnIndex(android.provider.OpenableColumns.DISPLAY_NAME)
                    if (idx >= 0 && c.moveToFirst() && !c.isNull(idx)) name = c.getString(idx)
                }
            }
        }
        if (!name.contains('.')) name += DocFormatRegistry.extFor(mime)
        pageTitle = name
        scope.launch {
            try {
                val bytes = withContext(Dispatchers.IO) {
                    ctx.contentResolver.openInputStream(android.net.Uri.parse(uri))?.use { it.readBytes() }
                        ?: throw IllegalStateException("无法读取该文件(可能已被删除)")
                }
                renderBytes(name, bytes, mime)
            } catch (e: Exception) {
                renderLocal(errorPage(name, e.message ?: "读取失败"), "打开失败")
            } finally {
                loadingPc = false
            }
        }
    }

    /**
     * 问 daemon"电脑上正跑着哪些 dev server"(HostService.ListDevServers)。
     * daemon 侧拨常见端口并发探测(1s/端口、整扫 ≤8s),结果缓存 60s ——
     * 这里的"重新扫描"一分钟内多次点也只会在过期后真正重扫。
     */
    fun refreshDevServers() {
        scope.launch {
            scanning = true
            scanHint = null
            try {
                val list = withContext(Dispatchers.IO) {
                    AgentConnection.hostService()
                        .listDevServers(ListDevServersRequest.newBuilder().build())
                        .serversList
                }
                servers = list
                if (list.isEmpty()) {
                    scanHint = "没发现正在运行的 dev server(常见端口 3000/5173/8000/8080…)"
                }
            } catch (e: Exception) {
                scanHint = "扫描失败:${e.message ?: "请先连接电脑"}"
            } finally {
                scanning = false
            }
        }
    }

    /**
     * 点列表里的一项 → WebView 加载 [devServerPreviewUrl] 拼出的地址。
     * USB 反向隧道只反代 daemon 的 7246,dev 端口不过隧道 —— 这里提前说人话,
     * 别让用户对着错误页猜原因;真正的加载失败由 onReceivedError 的失败页兜底。
     */
    fun openDevServer(s: DevServer) {
        val addr = AgentConnection.connAddress.value
        val url = devServerPreviewUrl(addr, s.port)
        if (url.isEmpty()) {
            android.widget.Toast.makeText(ctx, "还不知道电脑的地址,请先连接电脑", android.widget.Toast.LENGTH_LONG).show()
            return
        }
        if (addr.substringBeforeLast(':') == "127.0.0.1") {
            android.widget.Toast.makeText(
                ctx, "USB 连接打不开电脑端口;请在同一 WiFi 下(局域网/Tailscale)再试", android.widget.Toast.LENGTH_LONG,
            ).show()
        }
        showServers = false
        webRef?.loadUrl(url)
    }

    /** 地址栏回车:电脑路径走 pc 模式,手机文件走 content 模式,其余当网址/搜索词。 */
    fun submit(raw: String) {
        val t = raw.trim()
        if (t.isEmpty()) return
        when {
            t.startsWith("content://") -> openLocalUri(t)
            else -> {
                val pc = toPcPath(t)
                if (pc != null) openPcFile(pc) else webRef?.loadUrl(normalizeUrl(t))
            }
        }
    }

    /**
     * 网页下载入口(setDownloadListener 的落点)。
     *  · http/https → 交给 [PaDownloader.downloadFromUrl](带 UA/Cookie);
     *  · blob:/data: → 注入 JS 读成 base64 回传(浏览器内部 URL,原生取不到);
     *  · 其它 scheme → 属于外跳,给可见反馈而不是静默。
     */
    fun handleDownload(url: String, userAgent: String, contentDisposition: String, mime: String) {
        val name = WebNavPolicy.fileNameFromDisposition(contentDisposition, url)
        when {
            url.startsWith("http://") || url.startsWith("https://") -> {
                val cookie = runCatching {
                    android.webkit.CookieManager.getInstance().getCookie(url)
                }.getOrNull().orEmpty()
                PaDownloader.downloadFromUrl(
                    url = url, name = name, mime = mime,
                    userAgent = userAgent, cookie = cookie, sourceTitle = pageTitle,
                )
                android.widget.Toast.makeText(ctx, "已开始下载:$name", android.widget.Toast.LENGTH_SHORT).show()
            }
            url.startsWith("blob:") || url.startsWith("data:") -> {
                // 【安全】blob/data 只能靠页面内 JS + 脚本桥落盘,而桥仅在可信来源挂载。
                // 公网站点不注入(否则等于把"任意文件落盘"入口交给整个公网),给可见提示。
                if (!bridgeRegistered) {
                    android.widget.Toast.makeText(
                        ctx, "已限制:该站点不支持此类下载", android.widget.Toast.LENGTH_LONG,
                    ).show()
                    return
                }
                val w = webRef
                if (w == null) {
                    android.widget.Toast.makeText(ctx, "下载失败:页面未就绪", android.widget.Toast.LENGTH_LONG).show()
                    return
                }
                w.evaluateJavascript(blobFetchJs(url, name, mime), null)
                android.widget.Toast.makeText(ctx, "正在获取文件…", android.widget.Toast.LENGTH_SHORT).show()
            }
            else -> android.widget.Toast.makeText(
                ctx, "该链接需要跳转到外部 App", android.widget.Toast.LENGTH_LONG,
            ).show()
        }
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(PaEditorial.Canvas)
            .statusBarsPadding(),
    ) {
        // ---------- 顶栏:返回 + 地址栏 + 刷新(编辑式 chrome;网页本体不在此列) ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = PaEditorial.Space.NavGutter, end = 14.dp, top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PaNavIconButton(
                onClick = onBack,
                contentDescription = "返回",
                iconRes = R.drawable.ic_lucide_chevron_left,
                plain = true,
            )
            Spacer(Modifier.width(10.dp))
            Box(
                Modifier
                    .weight(1f)
                    .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.IconButton))
                    .padding(horizontal = 12.dp, vertical = 11.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                if (input.isEmpty()) {
                    Text("输入网址,或粘贴电脑上的文件路径", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Muted)
                }
                BasicTextField(
                    value = input,
                    onValueChange = { input = it },
                    singleLine = true,
                    textStyle = TextStyle(color = PaEditorial.Ink, fontSize = PaEditorial.Type.BodySmall.fontSize),
                    cursorBrush = SolidColor(PaEditorial.Primary),
                    keyboardOptions = KeyboardOptions(
                        keyboardType = KeyboardType.Uri,
                        imeAction = ImeAction.Go,
                    ),
                    keyboardActions = KeyboardActions(onGo = { submit(input) }),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Spacer(Modifier.width(10.dp))
            Spacer(Modifier.width(8.dp))
            PaNavIconButton(
                onClick = {
                    val t = input.trim()
                    when {
                        t.startsWith("content://") -> openLocalUri(t)
                        else -> {
                            val pc = toPcPath(t)
                            if (pc != null) openPcFile(pc) else webRef?.reload()
                        }
                    }
                },
                contentDescription = "刷新",
                iconRes = R.drawable.ic_lucide_refresh_cw,
            )
            Spacer(Modifier.width(8.dp))
            // 【电脑预览入口】问 daemon 要本机 dev server 清单,点开即预览(P2)。
            // 首次展开自动扫一次;之后再展开沿用结果,面板里有"重新扫描"。
            // active = 当前面板展开(主色浅底 + 主色图:主色只用于当前操作)。
            PaNavIconButton(
                onClick = {
                    showServers = !showServers
                    if (showServers && servers.isEmpty() && !scanning) refreshDevServers()
                },
                contentDescription = "电脑预览",
                iconRes = R.drawable.ic_lucide_monitor,
                active = showServers,
            )
        }

        // ---------- 第二行:标题 + 下载/分享 + 前进/后退 + 手机/桌面切换 + 外部打开 ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                when {
                    loadingPc -> "正在读取…"
                    pageTitle.isNotEmpty() -> pageTitle
                    else -> "内置浏览器"
                },
                Modifier.weight(1f),
                fontSize = PaEditorial.Type.Caption.fontSize,
                color = PaEditorial.Muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (canGoBack) {
                IconActionButton(R.drawable.ic_lucide_chevron_left, "后退", {
                    webRef?.goBack()
                }, bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            if (canGoForward) {
                IconActionButton(R.drawable.ic_lucide_chevron_right, "前进", {
                    webRef?.goForward()
                }, bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            // 【下载入口】浏览电脑上的文件时给一个明确的"存到手机"按钮。
            // 会话正文里不再放下载按钮(用户要求只留"在浏览器打开"),
            // 所以这里是 pc 文件唯一的下载入口 —— 必须显眼,用品牌色。
            if (pcPath.isNotEmpty()) {
                IconActionButton(
                    R.drawable.ic_lucide_download, "下载到手机",
                    {
                        PaDownloader.downloadFromPc(pcPath, pageTitle)
                        android.widget.Toast.makeText(ctx, "已开始下载到手机", android.widget.Toast.LENGTH_SHORT).show()
                    },
                    // 内容色读 OnPrimary 令牌(黑客盘主色电光青上白字 1.81:1,该盘=暗青)
                    bg = PaEditorial.Primary, tint = PaEditorial.OnPrimary,
                    size = 34.dp, iconSize = 16.dp,
                )
                Spacer(Modifier.width(8.dp))
            }
            if (localUri.isNotEmpty()) {
                IconActionButton(
                    R.drawable.ic_lucide_share_2, "分享",
                    { PaDownloader.shareLocalFile(ctx, localUri, pageTitle) },
                    bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
                )
                Spacer(Modifier.width(8.dp))
            }
            IconActionButton(
                if (desktopView) R.drawable.ic_lucide_smartphone else R.drawable.ic_lucide_monitor,
                if (desktopView) "切到手机视图" else "切到桌面视图",
                {
                    desktopView = !desktopView
                    val w = webRef ?: return@IconActionButton
                    w.settings.loadWithOverviewMode = desktopView
                    w.settings.useWideViewPort = true
                    // 【修复白屏】显式还原默认 UA(而不是置 null),本地页面用缓存重排
                    w.settings.userAgentString = if (desktopView) DESKTOP_UA else defaultUa
                    val local = lastLocal
                    if (local != null) {
                        pageTitle = local.second
                        progress = 1f
                        w.loadDataWithBaseURL(
                            null,
                            if (desktopView) local.first else adaptForMobile(local.first),
                            "text/html", "utf-8", null,
                        )
                    } else {
                        val cur = input.trim()
                        when {
                            cur.startsWith("content://") -> openLocalUri(cur)
                            else -> {
                                val pc = toPcPath(cur)
                                if (pc != null) openPcFile(pc)
                                else if (cur.isNotEmpty()) w.loadUrl(normalizeUrl(cur))
                                else w.reload()
                            }
                        }
                    }
                },
                bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp,
            )
            Spacer(Modifier.width(8.dp))
            IconActionButton(R.drawable.ic_lucide_globe, "用系统浏览器打开", {
                runCatching {
                    ctx.startActivity(
                        android.content.Intent(
                            android.content.Intent.ACTION_VIEW,
                            android.net.Uri.parse(normalizeUrl(input)),
                        ),
                    )
                }
            }, bg = PaEditorial.Surface, size = 34.dp, iconSize = 16.dp)
        }

        // 进度细线(编辑式:细线轨道 + 主色填充,只在进行中显示)
        Box(
            Modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(PaEditorial.Hairline),
        ) {
            if (progress in 0.01f..0.99f) {
                Box(
                    Modifier
                        .fillMaxWidth(progress)
                        .height(2.dp)
                        .background(PaEditorial.Primary),
                )
            }
        }

        // ---------- 电脑 dev server 面板(P2):展开时列在网页本体上方 ----------
        // 列表最多 11 行(端口表大小),限高 + 可滚动,不挤掉网页本体。
        if (showServers) {
            Column(
                Modifier
                    .fillMaxWidth()
                    .padding(horizontal = PaEditorial.Space.PageMargin, vertical = 6.dp)
                    .heightIn(max = 240.dp)
                    .verticalScroll(rememberScrollState())
                    .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
                    .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card)),
            ) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .padding(start = 14.dp, end = 12.dp, top = 10.dp, bottom = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text("电脑上的 dev server", Modifier.weight(1f), fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink)
                    if (scanning) {
                        Text("扫描中…", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
                    } else {
                        Text(
                            "重新扫描", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Primary,
                            modifier = Modifier.clickable { refreshDevServers() },
                        )
                    }
                }
                when {
                    scanning && servers.isEmpty() -> Text(
                        "正在拨 3000/5173/8000/8080 等常见端口…",
                        Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                    )
                    servers.isEmpty() -> Text(
                        scanHint ?: "没有正在运行的 dev server",
                        Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
                        fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted,
                    )
                    else -> servers.forEach { s ->
                        Row(
                            Modifier
                                .fillMaxWidth()
                                .clickable { openDevServer(s) }
                                .padding(horizontal = 14.dp, vertical = 9.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(":${s.port}", fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Primary)
                            Spacer(Modifier.width(10.dp))
                            Column(Modifier.weight(1f)) {
                                Text(
                                    s.title.ifBlank {
                                        s.serverHeader.ifBlank { "HTTP 服务" }
                                    },
                                    fontSize = PaEditorial.Type.BodySmall.fontSize, color = PaEditorial.Ink,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                                )
                                if (s.serverHeader.isNotBlank()) {
                                    Text(
                                        s.serverHeader, fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
                                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                                    )
                                }
                            }
                            Text("打开 →", fontSize = PaEditorial.Type.Caption.fontSize, color = PaEditorial.Muted)
                        }
                    }
                }
                if (servers.isNotEmpty()) {
                    Text(
                        "手机与电脑需在同一网络(WiFi / Tailscale)",
                        Modifier.padding(start = 14.dp, end = 14.dp, bottom = 10.dp),
                        fontSize = PaEditorial.Type.Micro.fontSize, color = PaEditorial.Muted,
                    )
                }
            }
        }

        // ---------- 网页本体(weight 占满剩余高度,不会把顶栏挤出可视区) ----------
        AndroidView(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
                .navigationBarsPadding(),
            factory = { context ->
                WebView(context).apply {
                    defaultUa = settings.userAgentString ?: ""
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    // 手机视图:按设备宽度排版(配合注入的 viewport),不再"整页缩放进一屏"
                    settings.useWideViewPort = true
                    settings.loadWithOverviewMode = false
                    // 支持双指缩放(阅读小字/H5 页面很有用),但不显示那对丑陋的悬浮按钮
                    settings.setSupportZoom(true)
                    settings.builtInZoomControls = true
                    settings.displayZoomControls = false
                    settings.textZoom = 100
                    // ---- 以下四项是"普通网站打不开/白屏"的直接原因 ----
                    // 1) 第三方 Cookie:默认**关闭**(targetSdk Lollipop+ 起)。
                    //    登录类站点(截图里的星辰 API 就是)普遍靠跨域 Cookie 维持会话,
                    //    关着就是"第一次能看到登录页,登完/再进变纯白"。
                    android.webkit.CookieManager.getInstance().setAcceptThirdPartyCookies(this, true)
                    android.webkit.CookieManager.getInstance().setAcceptCookie(true)
                    // 2) 混合内容:默认 MIXED_CONTENT_NEVER_ALLOW —— https 页面里的
                    //    http 图片/脚本/接口全被拦掉,页面就会"顶栏在、内容空"。
                    //    usesCleartextTraffic 只管主文档,管不到子资源,必须显式放开。
                    settings.mixedContentMode =
                        android.webkit.WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
                    // 3) DOM Storage / Database:现代前端(React/Vue)靠它存 token,
                    //    不给就是"接口通了但页面渲染不出来"。
                    settings.databaseEnabled = true
                    // 4) 明确的缓存策略:跟系统一致走默认(有网用网络、无网回缓存),
                    //    避免因隐式 LOAD_NO_CACHE 导致每次都是全新会话、登录态丢失。
                    settings.cacheMode = android.webkit.WebSettings.LOAD_DEFAULT
                    // 5) [本轮新增] 多窗口 / 新窗口:很多站点(target=_blank、window.open)
                    //    点开新窗口时若未开启此项,WebView 直接什么都不做 —— 用户会以为"浏览器坏了"。
                    //    配套的 onCreateWindow 见下方 WebChromeClient。
                    settings.javaScriptCanOpenWindowsAutomatically = true
                    settings.setSupportMultipleWindows(true)
                    // 6) [本轮新增] 部分 H5 视频/动效默认要求"用户手势"才播放,遥控场景常看不到内容;
                    //    同时确保图片默认加载(百度/抖音的封面多为懒加载 img)。
                    settings.mediaPlaybackRequiresUserGesture = false
                    settings.loadsImagesAutomatically = true
                    // 7) [第二轮·安全收严] 关闭本地/内容文件访问。
                    //    实测:App 预览走 loadDataWithBaseURL(baseUrl=null),WebView 回报
                    //    about:blank/data:;已下载文件由 ContentResolver 在 Kotlin 侧读成字节
                    //    再喂给 WebView —— WebView 自身**从不**加载 file:///content:// 主文档。
                    //    因此关闭这三项不影响预览与打开(API 30+ 本就默认关闭),只收窄攻击面。
                    settings.allowFileAccess = false
                    settings.allowUniversalAccessFromFileURLs = false
                    settings.allowContentAccess = false
                    // 加载闪屏底色与 pal().bg 同一 hex 镜像(三盘同构:浅=瓷白、
                    // 深=#0F141C、黑客=#050507,不再单写一套 #0b0b0d)
                    setBackgroundColor(android.graphics.Color.parseColor(pal().bg))
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(
                            view: WebView,
                            request: android.webkit.WebResourceRequest,
                        ): Boolean {
                            val url = request.url.toString()
                            // 【页内动作按钮】二进制提示页把「下载到手机 / 用系统应用打开」
                            // 做成链接,靠这个伪协议回传给原生层执行 —— 否则用户只能看着
                            // 一句"不支持预览"和一个死掉的下载图标。
                            if (url.startsWith("pa-action://")) {
                                when (url.removePrefix("pa-action://")) {
                                    "download" -> {
                                        if (pcPath.isNotEmpty()) {
                                            PaDownloader.downloadFromPc(pcPath, pageTitle)
                                            android.widget.Toast.makeText(
                                                ctx, "已开始下载到手机", android.widget.Toast.LENGTH_SHORT,
                                            ).show()
                                        }
                                    }
                                    "open" -> {
                                        PaDownloader.openLocalFile(
                                            ctx = ctx,
                                            uri = localUri,
                                            title = pageTitle,
                                            onOpenInBrowser = null,
                                        )
                                    }
                                }
                                return true
                            }
                            // WebView 能自己处理的 scheme:放行(返回 false 让 WebView 加载)。
                            if (WebNavPolicy.isWebScheme(url)) return false
                            // 【外跳 scheme 绝不崩溃】intent:// / baiduboxapp:// / snssdk1128:// /
                            // weixin:// 这些 WebView 加载不了;直接 loadUrl 会白屏,直接 startActivity
                            // 又可能抛 ActivityNotFoundException。这里尝试交给外部 App,
                            // 成功/失败都给可见反馈,绝不把异常丢给系统。
                            if (WebNavPolicy.isExternalScheme(url)) {
                                val opened = runCatching {
                                    val intent = if (url.startsWith("intent://")) {
                                        android.content.Intent.parseUri(
                                            url, android.content.Intent.URI_INTENT_SCHEME,
                                        )
                                    } else {
                                        android.content.Intent(
                                            android.content.Intent.ACTION_VIEW, android.net.Uri.parse(url),
                                        )
                                    }
                                    intent.addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
                                    ctx.startActivity(intent)
                                    true
                                }.getOrDefault(false)
                                android.widget.Toast.makeText(
                                    ctx,
                                    if (opened) "已跳转到外部 App"
                                    else "该链接需要跳转到外部 App(本机未安装对应应用)",
                                    android.widget.Toast.LENGTH_LONG,
                                ).show()
                                return true
                            }
                            return false
                        }

                        override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                            // 【安全】按当前来源同步脚本桥:可信(App 预览/本地/私网)才挂,
                            // 公网站点在内容加载前就把桥卸载,页面 JS 拿不到原生入口。
                            applyBridgePolicy(view, url)
                            if (isRemote(url)) {
                                input = url.orEmpty()
                                // 离开本地渲染页时清掉缓存(切换视图不再错误复用旧内容),
                                // 同时清掉"正在看的本地文件",避免下载按钮指向已经翻页的文件
                                lastLocal = null
                                pcPath = ""
                                localUri = ""
                            }
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onPageFinished(view: WebView?, url: String?) {
                            // 兜底再同步一次(某些跳转路径 onPageStarted 的 url 可能为空)
                            applyBridgePolicy(view, url)
                            if (isRemote(url)) input = url.orEmpty()
                            progress = 1f
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onReceivedError(
                            view: WebView?,
                            request: android.webkit.WebResourceRequest?,
                            error: android.webkit.WebResourceError?,
                        ) {
                            // 只有主文档失败才处理,子资源失败不打断阅读
                            if (request?.isForMainFrame != true) return
                            progress = 1f
                            val url = request.url?.toString().orEmpty()
                            // 【离线优先】同一 URL 命中本地快照就直接渲染,不再白屏等超时。
                            scope.launch {
                                val snap = withContext(Dispatchers.IO) { OfflineStore.getUrl(url) }
                                if (snap != null) {
                                    renderBytes(
                                        snap.name.ifBlank { pageTitle.ifBlank { url } },
                                        snap.bytes, snap.mime,
                                    )
                                    android.widget.Toast.makeText(
                                        ctx, offlineSnapshotToastCopy(snap.capturedAt), android.widget.Toast.LENGTH_SHORT,
                                    ).show()
                                } else {
                                    view?.loadDataWithBaseURL(
                                        null,
                                        errorPage(url, error?.description?.toString() ?: "无法访问该地址"),
                                        "text/html", "utf-8", null,
                                    )
                                }
                            }
                        }

                        // 【白屏的最后一块拼图】原实现只处理"传输层错误"(onReceivedError),
                        // 服务器**真的返回了 4xx/5xx** 时它不触发 —— 而这类响应体
                        // 往往就是一片空白。用户看到的是"顶栏在、内容是白的、没有任何提示",
                        // 完全无从判断是自己网络问题还是站点拒绝。
                        // 这里把主文档的 HTTP 错误也转成可见的失败页。
                        override fun onReceivedHttpError(
                            view: WebView?,
                            request: android.webkit.WebResourceRequest?,
                            errorResponse: android.webkit.WebResourceResponse?,
                        ) {
                            if (request?.isForMainFrame != true) return
                            val code = errorResponse?.statusCode ?: 0
                            // 只对明确的错误码出页;3xx 是正常重定向,不能拦
                            if (code < 400) return
                            progress = 1f
                            view?.loadDataWithBaseURL(
                                null,
                                errorPage(
                                    request.url?.toString().orEmpty(),
                                    "服务器返回 HTTP $code" +
                                        (errorResponse?.reasonPhrase?.let { " · $it" } ?: ""),
                                ),
                                "text/html", "utf-8", null,
                            )
                        }

                        // HTTPS 证书有问题时,WebView 默认静默取消 → 又是一片白。
                        // 【安全收严】只对"明确受信任的本地/自建"场景放行(回环 / 私网 /
                        // Tailscale 虚拟网内的自签证书,常见于本地开发服务器);公网站点一律
                        // cancel。绝不 proceed() 所有证书错误 —— 那是中间人攻击的漏洞。
                        @Suppress("DEPRECATION")
                        override fun onReceivedSslError(
                            view: WebView?,
                            handler: android.webkit.SslErrorHandler?,
                            error: android.net.http.SslError?,
                        ) {
                            val errorUrl = error?.url.orEmpty()
                            val host = runCatching { android.net.Uri.parse(errorUrl).host }
                                .getOrNull().orEmpty()
                            if (WebNavPolicy.trustedLocalHost(host)) {
                                handler?.proceed()
                                android.widget.Toast.makeText(
                                    ctx, "已信任本地自签证书:$host", android.widget.Toast.LENGTH_SHORT,
                                ).show()
                            } else {
                                handler?.cancel()
                                progress = 1f
                                view?.loadDataWithBaseURL(
                                    null,
                                    errorPage(errorUrl, "HTTPS 证书校验未通过,已阻止加载(不会忽略证书错误)"),
                                    "text/html", "utf-8", null,
                                )
                            }
                        }
                    }
                    webChromeClient = object : WebChromeClient() {
                        override fun onProgressChanged(view: WebView?, newProgress: Int) {
                            progress = newProgress / 100f
                        }

                        override fun onReceivedTitle(view: WebView?, title: String?) {
                            if (!title.isNullOrBlank() && title != "about:blank") pageTitle = title
                        }

                        // 【新窗口】站内 target=_blank / window.open 时若这里不处理,点开毫无反应
                        // (用户会认为"浏览器坏了")。用一个一次性子 WebView 截获目标 URL,
                        // 再在本 WebView 里加载 —— 这样返回键还能回到上一页。
                        override fun onCreateWindow(
                            view: WebView?,
                            isDialog: Boolean,
                            isUserGesture: Boolean,
                            resultMsg: android.os.Message?,
                        ): Boolean {
                            val parent = view ?: return false
                            val transport = resultMsg?.obj as? WebView.WebViewTransport ?: return false
                            val child = WebView(parent.context)
                            child.webViewClient = object : WebViewClient() {
                                override fun shouldOverrideUrlLoading(
                                    v: WebView,
                                    req: android.webkit.WebResourceRequest,
                                ): Boolean {
                                    val target = req.url.toString()
                                    if (!WebNavPolicy.isWebScheme(target)) {
                                        // 外跳 scheme:交给外部 App(失败静默不崩)
                                        runCatching {
                                            parent.context.startActivity(
                                                android.content.Intent(
                                                    android.content.Intent.ACTION_VIEW,
                                                    android.net.Uri.parse(target),
                                                ).addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK),
                                            )
                                        }
                                    } else {
                                        parent.loadUrl(target)
                                    }
                                    v.destroy()
                                    return true
                                }
                            }
                            transport.webView = child
                            resultMsg.sendToTarget()
                            return true
                        }
                    }
                    // 下载:站点触发的下载交给 PaDownloader(http/https 直下,blob/data 走 JS 兜底)
                    setDownloadListener { url, userAgent, contentDisposition, mimeType, _ ->
                        handleDownload(url, userAgent, contentDisposition, mimeType ?: "")
                    }
                    // blob:/data: 兜底通道。**仅当初始页属可信来源时才默认挂载**
                    // (pc:/content:/本地起始页都是 App 预览,回报 about:blank/data:);
                    // 若一开场就是公网 URL 则不挂 —— 不存在"先挂上、等 onPageStarted 再摘"的窗口。
                    // 之后每次页面开始/完成都由 applyBridgePolicy 按来源同步:
                    // 公网站点在内容加载前即被卸载,页面 JS 拿不到这个原生入口。
                    val initialRemote = initialUrl.startsWith("http://") || initialUrl.startsWith("https://")
                    if (!initialRemote) {
                        addJavascriptInterface(WebDownloadBridge(context), WebDownloadBridge.JS_NAME)
                    }
                    webRef = this
                    when {
                        initialUrl.startsWith("pc:") -> openPcUrl(initialUrl)
                        initialUrl.startsWith("content://") -> openLocalUri(initialUrl)
                        initialUrl.isNotBlank() -> loadUrl(normalizeUrl(initialUrl))
                        else -> renderLocal(startPage(), "内置浏览器")
                    }
                }
            },
        )
    }
}

/**
 * [BrowserScreenBody.renderBytes] 的纯解析段:字节 → 完整 HTML(P0-5)。
 * 重 CPU 同步解析(markdownHtml / PcFilePreview.render),在 Dispatchers.Default 上跑;
 * 只读入参与调用方在主线程取好的快照,不碰 UI/WebView/Compose 状态,可在任意线程执行。
 * Context 仅被 PcFilePreview 用于 ctx.cacheDir(线程安全的缓存目录 getter),后台线程安全。
 * suspend:与调用方的 withContext(Dispatchers.Default) 作用域同构,语义即"解析是挂起点"。
 *
 * @param dark   当前 App 是否深色主题(主线程快照)
 * @param pageBg 图片页底色(pal().bg 的主线程快照,三盘同构)
 * @param isPc   来源是否为电脑文件(pcPath 非空,决定二进制页的下载按钮)
 */
private suspend fun buildPreviewHtml(
    ctx: Context,
    name: String,
    bytes: ByteArray,
    mime: String,
    dark: Boolean,
    pageBg: String,
    isPc: Boolean,
): String {
    val lower = name.lowercase()
    val mLower = mime.lowercase()
    return when {
        mLower.startsWith("text/markdown") || lower.endsWith(".md") || lower.endsWith(".markdown") ->
            PcFilePreview.markdownHtml(String(bytes, Charsets.UTF_8), name, dark)
        mLower.startsWith("image/") || lower.endsWith(".png") || lower.endsWith(".jpg") ||
            lower.endsWith(".jpeg") || lower.endsWith(".gif") || lower.endsWith(".webp") ->
            "<body style=\"margin:0;background:" +
                // 图片页底色与 pal().bg 同镜像(三盘同构,黑客盘纯黑基底)
                pageBg +
                ";display:flex;align-items:center;" +
                "justify-content:center;height:100vh\"><img style=\"max-width:100%;" +
                "max-height:100vh\" src=\"data:image/png;base64," +
                android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP) +
                "\"></body>"
        mLower.startsWith("text/html") || lower.endsWith(".html") || lower.endsWith(".htm") ->
            String(bytes, Charsets.UTF_8)
        else -> {
            // 【预览配色跟主题走】否则浅色主题下打开文件,预览却是黑底
            val structured = PcFilePreview.render(ctx, lower, bytes, dark)
            when {
                structured != null -> structured
                PcFilePreview.isBinary(lower) || mLower.endsWith("octet-stream") ->
                    binaryPage(name, bytes.size.toLong(), isPc)
                else -> textPage(String(bytes, Charsets.UTF_8))
            }
        }
    }
}

/**
 * 移动端适配:给页面补 viewport、兜底字号、并保证可以纵向滚动。
 *
 * 【为什么必须做】agent 生成的页面基本都是"桌面端写死的像素稿"(14px 正文 + 100vh 布局),
 * 在手机 WebView 里要么字小到看不清,要么 body 被 overflow/height 锁死导致**完全不能下滑**。
 * 这里只做三件不破坏原设计的事:补 viewport、兜底最小字号、解除高度锁。
 */
private fun adaptForMobile(html: String): String {
    val head = "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1, " +
        "maximum-scale=5, user-scalable=yes\">"
    val css = "<style id=\"pa-mobile-adapt\">" +
        "html{-webkit-text-size-adjust:100%;}" +
        "html,body{overflow-y:auto !important;height:auto !important;min-height:100% !important;" +
        "max-height:none !important;}" +
        "body{font-size:max(16px,1em);line-height:1.6;}" +
        "</style>"
    val inject = head + css
    return when {
        html.contains("</head>") -> html.replaceFirst("</head>", inject + "</head>")
        html.contains("<body") -> html.replaceFirst("<body", "<head>$inject</head><body")
        else -> "<html><head>$inject</head><body>$html</body></html>"
    }
}

private const val DESKTOP_UA =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " +
        "Chrome/126.0 Safari/537.36"

/**
 * 内置预览时单次读取电脑文件的上限。
 * 之前传 0(服务端默认 2MB),大文件会被静默截断 → 解析器拿到半截 zip,
 * 表现成"表格里没有可显示的数据"。64MB 足够覆盖常规办公文档。
 */
private const val MAX_PREVIEW_BYTES = 64L * 1024 * 1024

/**
 * 地址栏该不该跟着 WebView 的回调走。
 *
 * 【为什么需要】本地渲染走 `loadDataWithBaseURL(null, …)`,WebView 回报的 URL 是
 * `about:blank`,直接回填会把地址栏(以及 lastLocal 缓存)冲掉 —— 视图切换就白屏了。
 * 只有真正的公网/内网地址才回填。
 */
private fun isRemote(url: String?): Boolean =
    url != null && (url.startsWith("http://") || url.startsWith("https://") ||
        url.startsWith("file:") || url.startsWith("ftp:"))

// MIME → 后缀的映射已统一到 DocFormatRegistry.extFor(此处原先的私有重复实现已删除)。

/**
 * 离线快照 toast 文案(纯函数,JVM 单测钉死):交代"看到的是哪一刻的网页",
 * 断网兜底才不是一笔糊涂账。快照带截至时间(capturedAt>0)→「离线:显示
 * M月d日 HH:mm 的网页快照」;存量快照无可靠时间戳(<=0)→ 旧文案,不编造时间。
 */
internal fun offlineSnapshotToastCopy(capturedAtMs: Long): String {
    if (capturedAtMs <= 0) return "离线:显示本地网页快照"
    val stamp = java.text.SimpleDateFormat("M月d日 HH:mm", java.util.Locale.getDefault())
        .format(java.util.Date(capturedAtMs))
    return "离线:显示 $stamp 的网页快照"
}

/**
 * 剥掉 pc: 地址携带的预览缓存元数据(`?_pas=<size>&_pam=<mtime>`),得到干净路径。
 * 地址栏要展示/回车重开的是路径本身,元数据只是缓存键的原料,混进去会让 readFile 拿到错误路径。
 */
private fun stripPcMeta(raw: String): String {
    val t = raw.removePrefix("pc:")
    val idx = t.indexOf("?_pas=")
    return if (idx < 0) t else t.substring(0, idx)
}

/**
 * 判断输入是不是"电脑上的路径"。
 * Windows 盘符(`D:\a\b.html` / `C:/a/b`)与 Unix 绝对路径都算。
 * @return 规范化后的路径,不是路径则 null
 */private fun toPcPath(raw: String): String? {
    val t = stripPcMeta(raw.trim()).removePrefix("file:///")
    val windows = Regex("^[A-Za-z]:[\\\\/]")
    return when {
        windows.containsMatchIn(t) -> t.replace('/', '\\')
        t.startsWith("/") && !t.startsWith("//") -> t
        else -> null
    }
}

/** 内置页面(起始页 / 失败页 / 纯文本 / 二进制提示)的配色 —— 统一跟 App 主题走。
 *
 *  【为什么单独抽出来】这几个页面原本各自写死黑底浅字,浅色主题下点开文件会突然变黑,
 *  和系统里的查看器一对比就像两个 App。现在所有内置页共用一套取值,只改一处。
 *
 *  【三盘同构 2026-09-30】WebView 里用不了 Compose 令牌,这里放的是 PaEditorial
 *  三盘槽位的 **hex 镜像**(浅盘=premium.css 原型值,深盘=apply(dark) 值,黑客盘
 *  单列 —— 与 PaHacker 契约一致);浅盘 muted/subtle 同步 2026-09-30 对比度终审
 *  后的加深值(#5F6C7C/#828B95),accent/danger 同样随盘(黑客=#22D3EE 电光青)。 */
private class Pal(
    val bg: String, val fg: String, val heading: String,
    val muted: String, val subtle: String, val card: String, val hint: String,
    val accent: String, val danger: String,
)

// 判定顺序:黑客先于 IsDark(黑客盘 IsDark=true,先判 IsDark 会吞掉黑客分支)。
private fun pal(): Pal = if (PaHacker.isHacker) Pal(
    bg = "#050507", fg = "#e6e9ef", heading = "#e6e9ef",
    muted = "#8b93a3", subtle = "#5b6372", card = "#0c0d11", hint = "#8b93a3",
    accent = "#22d3ee", danger = "#f87171",
) else if (PaEditorial.IsDark) Pal(
    bg = "#0f141c", fg = "#e8edf4", heading = "#e8edf4",
    muted = "#a2aebd", subtle = "#6b7887", card = "#161d28", hint = "#a2aebd",
    accent = "#6c9bf2", danger = "#f08580",
) else Pal(
    // 浅色档 = 编辑式令牌(冷白画布/深墨正文/次级灰/瓷白卡),与迁移后的 chrome 同体系
    bg = "#f6f7f6", fg = "#19222d", heading = "#19222d",
    muted = "#5f6c7c", subtle = "#828b95", card = "#ffffff", hint = "#5f6c7c",
    accent = "#386be7", danger = "#c94f4d",
)

/** 起始页:不再是一张白纸。 */
private fun startPage(): String {
    val p = pal()
    return """
    <body style="margin:0;background:${p.bg};color:${p.fg};font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:22px 18px">
        <div style="font-size:19px;font-weight:600;color:${p.heading}">内置浏览器</div>
        <div style="color:${p.subtle};margin-top:6px;font-size:12.5px">
          会话里 agent 生成的网页、报告、图片都在这台电脑上,可以直接在 App 内打开。
        </div>
        <div style="margin-top:18px;background:${p.card};border-radius:14px;padding:14px">
          <div style="color:${p.accent};font-weight:600;font-size:13px">怎么用</div>
          <div style="margin-top:8px;color:${p.muted};font-size:12.5px">
            · 在会话里点那条文件路径,自动在这里打开<br>
            · 也可以把路径粘到上面的地址栏,例如
              <span style="color:${p.fg}">D:\docs\index.html</span><br>
            · 公网地址直接输入,或用它搜索
          </div>
        </div>
      </div>
    </body>
""".trimIndent()
}

/** 失败页:明确告诉用户哪一步失败了。 */
private fun errorPage(target: String, reason: String): String {
    val p = pal()
    return """
    <body style="margin:0;background:${p.bg};color:${p.fg};font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:22px 18px">
        <div style="color:${p.danger};font-size:17px;font-weight:600">打不开这个页面</div>
        <div style="margin-top:10px;color:${p.muted};font-size:13px">${escapeHtml(reason)}</div>
        <div style="margin-top:14px;background:${p.card};border-radius:12px;padding:12px;
                    font:12px/1.6 ui-monospace,Menlo,Consolas,monospace;word-break:break-all;color:${p.subtle}">
          ${escapeHtml(target)}
        </div>
        <div style="margin-top:14px;color:${p.hint};font-size:12px">
          若这是电脑上的文件,请确认路径存在、且电脑端 agent 正在运行。
        </div>
        ${if (isPcPreviewUrl(target)) {
            "<div style=\"margin-top:10px;color:${p.hint};font-size:12px\">" +
                "如果是电脑上的 dev server:手机和电脑要在同一网络(WiFi / Tailscale)," +
                "且服务需监听 0.0.0.0(全部网卡)而不是仅 127.0.0.1。</div>"
        } else ""}
      </div>
    </body>
""".trimIndent()
}

/** 纯文本页(等宽、可换行)。 */
private fun textPage(text: String): String {
    val p = pal()
    return "<body style=\"margin:0;background:${p.bg};color:${p.fg};" +
        "font:13px/1.6 ui-monospace,Menlo,Consolas,monospace\"><pre style=\"" +
        "white-space:pre-wrap;word-break:break-word;padding:14px;margin:0\">" +
        escapeHtml(text) + "</pre></body>"
}

/** 二进制文件提示页:不只说「不行」,还把下一步动作直接给出来。 */
private fun binaryPage(name: String, size: Long, canDownload: Boolean): String {
    val p = pal()
    val btn = "display:inline-block;padding:11px 18px;border-radius:12px;" +
        "font-size:14px;font-weight:600;text-decoration:none;margin:0 10px 10px 0"
    val buttons = buildString {
        if (canDownload) {
            append("<a style=\"" + btn + ";background:${p.accent};color:#ffffff\" " +
                "href=\"pa-action://download\">下载到手机</a>")
        }
        append("<a style=\"" + btn + ";background:${p.card};color:${p.fg}\" " +
            "href=\"pa-action://open\">用系统应用打开</a>")
    }
    return """
    <body style="margin:0;background:${p.bg};color:${p.fg};font:14px/1.7 system-ui,-apple-system,sans-serif">
      <div style="padding:26px 18px">
        <div style="font-size:17px;font-weight:600;color:${p.heading}">这个格式不适合在线预览</div>
        <div style="margin-top:10px;color:${p.muted};font-size:13px">
          ${escapeHtml(name)}( ${humanSize(size)} ) 是二进制文件(安装包 / 老版 Office / 压缩包等),
          在浏览器里只能看到乱码。下载到手机后用对应应用打开,或直接交给手机上已装的应用。
        </div>
        <div style="margin-top:22px">$buttons</div>
      </div>
    </body>
    """.trimIndent()
}

private fun escapeHtml(s: String): String = s
    .replace("&", "&amp;")
    .replace("<", "&lt;")
    .replace(">", "&gt;")

/** 补全协议;不像网址的当搜索词交给 Bing。 */
private fun normalizeUrl(raw: String): String {
    val t = raw.trim()
    if (t.isEmpty()) return ""
    if (t.startsWith("http://") || t.startsWith("https://")) return t
    if (t.startsWith("about:") || t.startsWith("data:")) return t
    return if (t.contains('.') && !t.contains(' ')) "https://$t"
    else "https://www.bing.com/search?q=" + java.net.URLEncoder.encode(t, "UTF-8")
}

/**
 * dev server 的手机侧加载地址:把端口挂到**当前 daemon 连接地址**的主机部分。
 *
 * 【为什么要换主机】daemon 探测的是 127.0.0.1(它自己的回环),手机照抄的话
 * 127.0.0.1 指向手机本身。dev server 一般监听 0.0.0.0,所以把 daemon 的
 * 局域网 / Tailscale 地址接上来才打得通。
 *
 * 纯函数(单测覆盖 DevServerPreviewUrlTest):
 *  · "192.168.1.5:7246" + 5173 → "http://192.168.1.5:5173"
 *  · "127.0.0.1:7246"(USB 反向隧道)→ 原样拼出回环地址(调用方负责提示
 *    "USB 打不开 dev 端口,需同 WiFi"),拼装与提示分离,本函数保持纯粹;
 *  · 空串 / 没有冒号的畸形地址 → ""(调用方隐藏动作并提示先连接)。
 */
internal fun devServerPreviewUrl(connAddress: String, port: Int): String {
    val t = connAddress.trim()
    if (!t.contains(':')) return ""
    val host = t.substringBeforeLast(':')
    if (host.isEmpty()) return ""
    return "http://$host:$port"
}

/**
 * 是否"直连电脑 dev server"的地址(私网/回环/Tailscale 的 http)。
 * 失败页据此追加同网段提示;纯函数,单测覆盖。
 */
internal fun isPcPreviewUrl(url: String): Boolean {
    if (!url.startsWith("http://")) return false
    val host = url.removePrefix("http://").substringBefore('/').substringBeforeLast(':')
    return Regex("^(127\\.|10\\.|192\\.168\\.|172\\.(1[6-9]|2\\d|3[01])\\.|100\\.)")
        .containsMatchIn(host)
}

/**
 * 让页面用 JS 把 blob:/data: 内容读成 base64 并回传原生(见 [WebDownloadBridge])。
 * blob: 是浏览器内部 URL,原生 HttpURLConnection 取不到,只能这样"绕一圈"。
 * url/name/mime 全部走 JSON 引号,避免注入与转义问题。
 */
private fun blobFetchJs(url: String, name: String, mime: String): String {
    val js = WebDownloadBridge.JS_NAME
    return "(function(){try{" +
        "var u=${org.json.JSONObject.quote(url)},nm=${org.json.JSONObject.quote(name)}," +
        "mt=${org.json.JSONObject.quote(mime)};" +
        "fetch(u).then(function(r){return r.blob();}).then(function(b){" +
        "var fr=new FileReader();" +
        "fr.onload=function(){var s=fr.result||'';var i=s.indexOf(',');" +
        "$js.onData(nm, mt||b.type, i>=0?s.substring(i+1):s);};" +
        "fr.onerror=function(){$js.onError('读取失败');};" +
        "fr.readAsDataURL(b);" +
        "}).catch(function(e){$js.onError(String(e));});" +
        "}catch(e){$js.onError(String(e));}})();"
}

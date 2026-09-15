package dev.pocketagent.app.ui

import android.annotation.SuppressLint
import android.graphics.Bitmap
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.theme.Pa

/**
 * 内置浏览器 —— 会话里输出的页面/链接直接在 App 内看,不用跳出到系统浏览器。
 *
 * 入口:
 *  - 聊天消息里的链接(LinkAnnotation 的 UriHandler 被本页接管)
 *  - 侧边栏「网页」入口(带上次访问的地址)
 *
 * 只做"看"这件事:地址栏、后退、前进、刷新、进度条;
 * 需要系统能力(下载/分享)时用右上角"外部打开"交给系统浏览器。
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun BrowserScreen(initialUrl: String = "", onBack: () -> Unit = {}) {
    var input by remember { mutableStateOf(initialUrl) }
    var progress by remember { mutableFloatStateOf(0f) }
    var pageTitle by remember { mutableStateOf("") }
    var canGoBack by remember { mutableStateOf(false) }
    var canGoForward by remember { mutableStateOf(false) }
    var webRef by remember { mutableStateOf<WebView?>(null) }
    val ctx = androidx.compose.ui.platform.LocalContext.current

    fun go(target: String) {
        val url = normalizeUrl(target)
        if (url.isEmpty()) return
        input = url
        webRef?.loadUrl(url)
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(Pa.Bg)
            .statusBarsPadding(),
    ) {
        // ---------- 顶栏:返回 + 地址栏 ----------
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = 12.dp, end = 12.dp, top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Spacer(Modifier.width(10.dp))
            Box(
                Modifier
                    .weight(1f)
                    .background(Pa.Card, RoundedCornerShape(14.dp))
                    .padding(horizontal = 12.dp, vertical = 11.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                if (input.isEmpty()) {
                    Text("输入网址或搜索内容", fontSize = 13.sp, color = Pa.TextTertiary)
                }
                BasicTextField(
                    value = input,
                    onValueChange = { input = it },
                    singleLine = true,
                    textStyle = TextStyle(color = Pa.TextPrimary, fontSize = 13.sp),
                    cursorBrush = SolidColor(Pa.Accent),
                    keyboardOptions = KeyboardOptions(
                        keyboardType = KeyboardType.Uri,
                        imeAction = ImeAction.Go,
                    ),
                    keyboardActions = KeyboardActions(onGo = { go(input) }),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Spacer(Modifier.width(10.dp))
            IconActionButton(R.drawable.ic_lucide_refresh_cw, "刷新", {
                webRef?.reload()
            }, bg = Pa.Card)
        }

        // 页面标题 + 导航按钮
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                pageTitle.ifEmpty { "内置浏览器" },
                Modifier.weight(1f),
                fontSize = 11.sp,
                color = Pa.TextTertiary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (canGoBack) {
                IconActionButton(R.drawable.ic_lucide_chevron_left, "后退", {
                    webRef?.goBack()
                }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            if (canGoForward) {
                IconActionButton(R.drawable.ic_lucide_chevron_right, "前进", {
                    webRef?.goForward()
                }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
                Spacer(Modifier.width(8.dp))
            }
            IconActionButton(R.drawable.ic_lucide_globe, "用系统浏览器打开", {
                runCatching {
                    ctx.startActivity(
                        android.content.Intent(
                            android.content.Intent.ACTION_VIEW,
                            android.net.Uri.parse(input),
                        ),
                    )
                }
            }, bg = Pa.Card, size = 34.dp, iconSize = 16.dp)
        }

        // 进度条(细线,不抢视线)
        Box(
            Modifier
                .fillMaxWidth()
                .height(2.dp)
                .background(Pa.Card),
        ) {
            if (progress in 0.01f..0.99f) {
                Box(
                    Modifier
                        .fillMaxWidth(progress)
                        .height(2.dp)
                        .background(Pa.Accent),
                )
            }
        }

        // ---------- 网页本体 ----------
        AndroidView(
            modifier = Modifier
                .fillMaxSize()
                .navigationBarsPadding(),
            factory = { context ->
                WebView(context).apply {
                    settings.javaScriptEnabled = true
                    settings.domStorageEnabled = true
                    settings.useWideViewPort = true
                    settings.loadWithOverviewMode = true
                    webViewClient = object : WebViewClient() {
                        override fun shouldOverrideUrlLoading(
                            view: WebView,
                            request: android.webkit.WebResourceRequest,
                        ): Boolean = false // 站内跳转都留在内置浏览器里

                        override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                            input = url.orEmpty()
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }

                        override fun onPageFinished(view: WebView?, url: String?) {
                            input = url.orEmpty()
                            progress = 1f
                            canGoBack = view?.canGoBack() ?: false
                            canGoForward = view?.canGoForward() ?: false
                        }
                    }
                    webChromeClient = object : WebChromeClient() {
                        override fun onProgressChanged(view: WebView?, newProgress: Int) {
                            progress = newProgress / 100f
                        }

                        override fun onReceivedTitle(view: WebView?, title: String?) {
                            pageTitle = title.orEmpty()
                        }
                    }
                    webRef = this
                    if (initialUrl.isNotBlank()) loadUrl(normalizeUrl(initialUrl))
                }
            },
        )
    }
}

/** 补全协议;不像网址的当搜索词交给 Bing(免得用户手输 www 才能打开)。 */
private fun normalizeUrl(raw: String): String {
    val t = raw.trim()
    if (t.isEmpty()) return ""
    if (t.startsWith("http://") || t.startsWith("https://")) return t
    if (t.startsWith("about:") || t.startsWith("file:")) return t
    return if (t.contains('.') && !t.contains(' ')) "https://$t"
    else "https://www.bing.com/search?q=" + java.net.URLEncoder.encode(t, "UTF-8")
}

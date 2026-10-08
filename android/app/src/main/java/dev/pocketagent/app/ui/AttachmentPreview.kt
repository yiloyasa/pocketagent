package dev.pocketagent.app.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.pressableLong
import dev.pocketagent.app.ui.theme.Pa
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.PaGlassToken
import java.util.Locale
import kotlin.math.max

/** 附件卡片里文件名的中间省略上限(超过则保留扩展名的中间省略,见 [middleEllipsize])。 */
private const val ATTACH_CARD_NAME_MAX = 60

enum class AttachmentKind { IMAGE, FILE }

fun attachmentKind(name: String, mediaType: String): AttachmentKind {
    if (mediaType.lowercase(Locale.US).startsWith("image/")) return AttachmentKind.IMAGE
    return when (name.substringAfterLast('.', "").lowercase(Locale.US)) {
        "png", "jpg", "jpeg", "gif", "webp", "bmp", "heic" -> AttachmentKind.IMAGE
        else -> AttachmentKind.FILE
    }
}

fun formatAttachmentSize(bytes: Long): String {
    val value = max(0L, bytes).toDouble()
    return when {
        value < 1024 -> "${value.toLong()} B"
        value < 1024 * 1024 -> String.format(Locale.US, "%.1f KB", value / 1024)
        value < 1024 * 1024 * 1024 -> String.format(Locale.US, "%.1f MB", value / (1024 * 1024))
        else -> String.format(Locale.US, "%.1f GB", value / (1024 * 1024 * 1024))
    }
}

fun attachmentFromBytes(
    name: String,
    pcPath: String,
    bytes: ByteArray,
    mediaType: String,
    localUri: String?,
): Attachment = Attachment(
    name = name,
    pcPath = pcPath,
    size = bytes.size,
    mediaType = mediaType,
    localUri = localUri,
    previewData = bytes.takeIf {
        attachmentKind(name, mediaType) == AttachmentKind.IMAGE
    },
)

@Composable
fun AttachmentCard(
    attachment: Attachment,
    preview: ImageBitmap? = null,
    modifier: Modifier = Modifier,
    onClick: () -> Unit = {},
) {
    val image = attachmentKind(attachment.name, attachment.mediaType) == AttachmentKind.IMAGE
    val ctx = LocalContext.current
    // 触感:长按复制成功时给一次长按触觉确认(与全局 pressable(haptic=true) 同一纪律)
    val haptics = LocalHapticFeedback.current
    Row(
        modifier
            // 单击预览 / 长按复制完整文件名(与消息长按复制同一写法)
            .pressableLong(
                onClick = onClick,
                onLongPress = {
                    haptics.performHapticFeedback(HapticFeedbackType.LongPress)
                    val cm = ctx.getSystemService(android.content.Context.CLIPBOARD_SERVICE)
                        as android.content.ClipboardManager
                    cm.setPrimaryClip(
                        android.content.ClipData.newPlainText("PocketAgent 文件名", attachment.name),
                    )
                    android.widget.Toast.makeText(ctx, "已复制文件名", android.widget.Toast.LENGTH_SHORT).show()
                },
            )
            // 【玻璃材质语言】附件卡保持实色底(AccentDim,用户侧家族色)——卡内有
            // 缩略图与文件名正文,叠玻璃会透字;高级感由 0.5dp 发丝高光边给出
            // (与玻璃家族同一根亮边令牌),材质家族一致、可读性不打折。
            .background(PaEditorial.PrimaryContainer, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .border(0.5.dp, PaGlassToken.Highlight, RoundedCornerShape(PaEditorial.Radius.IconButton))
            .padding(8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        if (image && preview != null) {
            Image(preview, contentDescription = attachment.name, modifier = Modifier.size(64.dp))
        } else {
            Box(
                Modifier.size(54.dp).background(PaEditorial.Raised, RoundedCornerShape(PaEditorial.Radius.Chip)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(if (image) R.drawable.ic_lucide_image else R.drawable.ic_lucide_file_text),
                    contentDescription = null,
                    tint = PaEditorial.Primary,
                    modifier = Modifier.size(24.dp),
                )
            }
        }
        // 文件名最多 3 行、可换行;宽度吃满剩余空间以强制换行(否则长名被裁)。
        // 真需省略时保留扩展名(见 middleEllipsize),不让用户丢掉"这是什么文件"。
        Column(Modifier.weight(1f)) {
            Text(
                middleEllipsize(attachment.name, ATTACH_CARD_NAME_MAX),
                color = PaEditorial.Ink, fontSize = PaEditorial.Type.Caption.fontSize,
                maxLines = 3, softWrap = true, overflow = TextOverflow.Ellipsis,
            )
            Text(formatAttachmentSize(attachment.size.toLong()), color = PaEditorial.Muted, fontSize = PaEditorial.Type.Micro.fontSize)
        }
    }
}

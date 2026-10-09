package dev.pocketagent.app.ui

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.media.ExifInterface
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
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
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

// ---------------------------------------------------------------- 发送链路原图下采样(性能侦察 G3 残段)

/**
 * 发送图片的长边阈值(px):超过则等比压到长边 ≤ 此值再上传。
 * 1600px ≈ 2x 密度下一屏半的解析度,视觉/多模态输入都够用;而手机相机原图
 * (4032px 长边,1-8MB)与长截图(2400px)按原样进 TurnInput(gRPC 全量缓冲,
 * 排队时成倍驻留)是纯浪费 —— 压后通常 ~几百 KB。
 */
const val ATTACHMENT_UPLOAD_MAX_EDGE_PX = 1600

/** JPEG/WEBP 有损重编码的质量(与方案口径一致 ~85)。PNG 无损,此值不生效。 */
internal const val ATTACHMENT_UPLOAD_JPEG_QUALITY = 85

/**
 * 发送图片的保格式重编码档(纯 Kotlin 枚举,JVM 单测可钉;到 Bitmap.CompressFormat
 * 的映射只发生在 [downscaleImageBytesForUpload] 的解码现场)。
 */
enum class UploadImageFormat { JPEG, PNG, WEBP }

/**
 * 重编码格式决策(纯函数):**保持原格式**(jpeg→jpeg、png→png 保透明、
 * webp→webp),文件名与 mediaType 一字不改 —— 下采样只动像素不动身份,
 * 电脑端按路径/类型解码不会错位。
 *
 * 其余一律返回 null = 原样上传:动图 gif 重编码会丢动画;heic/bmp/未知格式
 * 没有把握安全转出同名同型的字节,宁可保守(权衡结论:格式转换的错位风险
 * 大于多省的那点字节,见"不破坏文件原样传输语义"纪律)。
 */
fun uploadImageFormatOf(mediaType: String, name: String): UploadImageFormat? {
    val mime = mediaType.lowercase(Locale.US)
    return when {
        mime == "image/jpeg" || mime == "image/jpg" -> UploadImageFormat.JPEG
        mime == "image/png" || name.substringAfterLast('.', "").lowercase(Locale.US) == "png" -> UploadImageFormat.PNG
        mime == "image/webp" -> UploadImageFormat.WEBP
        else -> null
    }
}

/**
 * 等比缩放目标尺寸(纯函数):长边压到 [maxEdge],短边按比例取整;
 * 已不超阈值或尺寸非法(≤0)原样返回(调用方据此跳过重采样)。
 */
fun uploadDownscaleSizeOf(width: Int, height: Int, maxEdge: Int = ATTACHMENT_UPLOAD_MAX_EDGE_PX): Pair<Int, Int> {
    if (width <= 0 || height <= 0) return width to height
    val longEdge = maxOf(width, height)
    if (longEdge <= maxEdge) return width to height
    val scale = maxEdge.toDouble() / longEdge
    return Math.round(width * scale).toInt() to Math.round(height * scale).toInt()
}

/**
 * 两段解码的抽样档(纯函数):返回**长边**仍 ≥ [maxEdge] 的最小 2^n
 * (先粗采样把峰值内存从 4032×3024≈48MB 压到 2016×1512≈12MB,再精确缩放到目标;
 * 与缩略图的 attachmentThumbnailSampleSize 口径不同 —— 那个按"两边都到目标",
 * 这里只认长边)。长边已不超阈值/尺寸非法按 1。
 */
fun uploadDownscaleSampleSizeOf(width: Int, height: Int, maxEdge: Int = ATTACHMENT_UPLOAD_MAX_EDGE_PX): Int {
    if (width <= 0 || height <= 0) return 1
    var sample = 1
    var longEdge = maxOf(width, height)
    while (longEdge / 2 >= maxEdge) {
        longEdge /= 2
        sample *= 2
    }
    return sample
}

/**
 * EXIF 方向值 → 需要的顺时针旋转角(纯函数):只认 90/180/270 三个纯旋转;
 * 翻转类(2/4/5/7)与未知值按 0(不转) —— 镜像无法用旋转还原,硬转反而错。
 * 重编码会把方向烘进像素,手机预览与电脑端看到的是同一张正图
 * (旧链路两端都按原始字节解码,带 EXIF 的相机照片在两端都是横的)。
 */
fun uploadExifRotationDegreesOf(orientation: Int): Int = when (orientation) {
    6 -> 90
    3 -> 180
    8 -> 270
    else -> 0
}

/**
 * 发送链路的图片字节准备(**必须 IO 线程调用**):超过长边阈值的
 * jpeg/png/webp 先粗采样解码 → EXIF 转正 → 精确缩放 → 按原格式重编码;
 * 其余附件(非图片/动图/未识别格式/已不超阈值)原字节返回。
 *
 * 【绝不阻断上传】解码/编码任一步失败都回退原始字节 —— 下采样是优化,
 * 不是发送的前置条件;压缩结果反而变大(病态小图)也回退原字节。
 * 【非图片不动】文件原样传输的语义只对图片放宽(方案口径),其余格式不进本函数路径。
 */
fun downscaleImageBytesForUpload(bytes: ByteArray, name: String, mediaType: String): ByteArray {
    val format = uploadImageFormatOf(mediaType, name) ?: return bytes
    // 纯决策档 → android 压缩格式(此映射之后不再做格式判断)
    val compressFormat = when (format) {
        UploadImageFormat.JPEG -> Bitmap.CompressFormat.JPEG
        UploadImageFormat.PNG -> Bitmap.CompressFormat.PNG
        UploadImageFormat.WEBP -> Bitmap.CompressFormat.WEBP
    }
    return runCatching {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return bytes
        val (targetW, targetH) = uploadDownscaleSizeOf(bounds.outWidth, bounds.outHeight)
        if (targetW == bounds.outWidth && targetH == bounds.outHeight) return bytes
        val opts = BitmapFactory.Options().apply {
            inSampleSize = uploadDownscaleSampleSizeOf(bounds.outWidth, bounds.outHeight)
        }
        val decoded = BitmapFactory.decodeByteArray(bytes, 0, bytes.size, opts) ?: return bytes
        val matrix = Matrix()
        if (format == UploadImageFormat.JPEG) {
            // EXIF 方向只对 jpeg 有意义;重编码后方向已烘进像素,原 EXIF 不再随行
            val degrees = runCatching {
                uploadExifRotationDegreesOf(
                    ExifInterface(ByteArrayInputStream(bytes))
                        .getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL),
                )
            }.getOrDefault(0)
            if (degrees != 0) matrix.postRotate(degrees.toFloat())
        }
        matrix.postScale(
            targetW.toFloat() / decoded.width,
            targetH.toFloat() / decoded.height,
        )
        val scaled = Bitmap.createBitmap(decoded, 0, 0, decoded.width, decoded.height, matrix, true)
        val out = ByteArrayOutputStream()
        scaled.compress(compressFormat, ATTACHMENT_UPLOAD_JPEG_QUALITY, out)
        val result = out.toByteArray()
        // 优化不许倒退:压完反而更大(病态输入)就回退原字节
        if (result.size >= bytes.size) bytes else result
    }.getOrNull() ?: bytes
}

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

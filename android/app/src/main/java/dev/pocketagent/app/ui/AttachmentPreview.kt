package dev.pocketagent.app.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.theme.Pa
import java.util.Locale
import kotlin.math.max

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
    Row(
        modifier
            .clickable(onClick = onClick)
            .background(Pa.AccentDim, RoundedCornerShape(14.dp))
            .padding(8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        if (image && preview != null) {
            Image(preview, contentDescription = attachment.name, modifier = Modifier.size(64.dp))
        } else {
            Box(
                Modifier.size(54.dp).background(Pa.CardHigh, RoundedCornerShape(10.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    painterResource(if (image) R.drawable.ic_lucide_image else R.drawable.ic_lucide_file_text),
                    contentDescription = null,
                    tint = Pa.Accent,
                    modifier = Modifier.size(24.dp),
                )
            }
        }
        Column {
            Text(attachment.name, color = Pa.TextPrimary, fontSize = 12.sp, maxLines = 1)
            Text(formatAttachmentSize(attachment.size.toLong()), color = Pa.TextSecondary, fontSize = 10.sp)
        }
    }
}

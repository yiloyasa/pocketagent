# 离线语音资产（SenseVoice 模型）——【2026-10-09 起已移出 APK】

本目录**不再有任何构建输入**：239MB 的 `model.int8.onnx` 与 `tokens.txt` 已移至
仓库 `dist/models/` 留档（`.gitignore` 双向排除，均不入 git），APK 体积因此从
约 220MB 大幅下降。目录留作占位与说明，请勿把模型文件放回这里。

模型现在的获取方式（二选一，均不走 APK）：

1. **应用内按需下载（正路）**：`SenseVoiceModelManager.prepareBundledModel()`
   在语音被启用时触发 `SenseVoiceModelFiles` 下载/断点续传，下载源与 SHA-256
   烧录常量同源（见 `ui/component/SenseVoiceModelManager.kt`）。
2. **仓库留档**：`dist/models/` 下的副本由 `android/build-ascii.ps1` 保证
   （缺失且网络可用时自动补下并校验），仅供留档/侧载，**不进 APK、不进 git**。

| 文件 | 字节数 | SHA-256 |
|---|---|---|
| `model.int8.onnx` | 239,233,841 | `c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51` |
| `tokens.txt` | 315,894 | `f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc` |

- 文件名/字节数/哈希的唯一真源：`ui/component/SenseVoiceModelManager.kt`
  的烧录常量（`files` 列表 + `TOTAL_BYTES`）与 `android/build-ascii.ps1` 的
  下载校验配置，两处必须一致。

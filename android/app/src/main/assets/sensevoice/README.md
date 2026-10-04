# 离线语音资产（SenseVoice 模型）

本目录是 **APK 构建输入**：目录内的 `*.onnx` 与 `tokens.txt` 被 `.gitignore`
排除（不入库），fresh clone 后这里只剩这份 README，此时**出不了完整 APK**。

正常构建**无需手工操作**：跑 `android/build-ascii.ps1` 会自动下载并按下表
SHA-256 校验后把文件放进本目录。手工获取时把文件放进本目录即可，
**文件名必须与下表一字不差**（Kotlin 侧按烧录常量逐字打开）。

| 文件 | 字节数 | SHA-256 |
|---|---|---|
| `model.int8.onnx` | 239,233,841 | `c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51` |
| `tokens.txt` | 315,894 | `f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc` |

- 文件名/字节数/哈希的唯一真源：`ui/component/SenseVoiceModelManager.kt`
  的烧录常量（`files` 列表 + `TOTAL_BYTES`，首启从 APK 资产解压时逐字节
  复核）与 `android/build-ascii.ps1` 的下载校验配置，两处必须一致。
- 下载源不在此罗列：以 `android/build-ascii.ps1` 内的现有双源配置为准
  （固定上游仓库与 revision 的说明另见 `android/app/libs/README.md`）。
- 手机端不做任何运行时模型下载：模型打进 APK，首启只做解压 + 校验。

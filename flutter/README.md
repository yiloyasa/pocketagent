# pocketagent_ui — PocketAgent Flutter UI 模块

手机遥控电脑 AI agent 的 Flutter 重写（方案见
`docs/design/2026-10-14-flutter-rewrite-plan.md`）。**module 模板**，
供后续 B4 阶段以 add-to-app 方式并入 `android/` 宿主。

```bash
# 国内网络
export PUB_HOSTED_URL=https://pub.flutter-io.cn
export FLUTTER_STORAGE_BASE_URL=https://storage.flutter-io.cn

flutter analyze      # 质量门：零 warning 零 info
flutter test         # 设计系统 + 组件 + 页面基线
flutter run          # 独立跑壳（module 需宿主，日常用 flutter test/analyze）
```

## ⚠ 本机中文路径坑（Windows）

`flutter analyze` 在 `G:\项目1\...` 下必崩：

```
FormatException: Unexpected end of input (at character 310)
  LspByteStreamServerChannel._readMessage
analysis server exited with code 255
```

原因是 LSP 分帧的 `Content-Length` 与中文路径的 UTF-8 字节数不一致。
绕过办法（已验证，等价于零告警证据）：

```powershell
New-Item -ItemType Junction -Path G:\pa_flutter -Target 'G:\项目1\pocketagent\flutter'
Set-Location G:\pa_flutter
C:\src\flutter\bin\flutter.bat analyze     # No issues found!
```

或直接用同套 analyzer：`C:\src\flutter\bin\dart.bat analyze .`
（`flutter test` 不走 LSP，在中文路径下正常。）

## 目录

```
lib/theme/      设计系统 token：palette(双套颜色) / typography / radii / pocket_theme
lib/components/ pa_card · pa_button · pa_status_chip · pa_tool_card · pa_approval_sheet
lib/pages/      onboarding · threads · tools · files · settings（首批静态页）
lib/main.dart   底部 4 Tab 壳 + 深浅色切换
test/           token 完整性、圆角合法性、组件渲染、页面冒烟
```

## 红线

- 主按钮永不红；危险操作走 danger 描边审批卡（破坏性 = 粗边框 + ⚠ + 按钮序反转）。
- 组件取色一律 `context.pa`，禁止散写 hex。
- 单屏圆角 ≤3 种（button 8 / card 14 / sheet 22 / avatar 10）。

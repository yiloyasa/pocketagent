# PocketAgent Chat, Approval, Attachment, and Network Fixes Implementation Plan

**Goal:** 修复聊天 Markdown 显示、状态栏遮挡、审批交互与协议、附件工作区和图片展示，并降低 Tailscale 网络波动对任务与审批的影响。

**Architecture:** Android 端将消息显示拆成可测试的 Markdown/附件展示模型；聊天页把状态提示放到系统状态栏安全区域之内，审批弹窗改为不可拖拽、默认折叠命令。daemon 端修正 Codex approval decision 映射，并把当前会话 workspace 作为附件目标；连接层使用网络变化触发的 channel 重建、候选地址健康状态和退避探活，避免重复强制断开。

**Tech Stack:** Kotlin, Jetpack Compose, gRPC/OkHttp, protobuf, Go, Codex app-server protocol, PowerShell 7, Android device smoke tests.

**Spec:** User request with screenshots dated 2026-09-14; root-cause findings recorded in the conversation.

## Global Constraints

- 面向用户的说明使用简体中文；代码、命令和技术标识保持英文。
- 使用 PowerShell 7 执行 Windows 命令。
- 不加入 TLS、推送或真正的 `CancelTurn`；本轮只修复用户列出的五类问题。
- 源码修改使用 `apply_patch`；不直接用脚本覆盖源码。
- Android、daemon 和协议变更必须分别通过对应测试；完成前必须重新构建 daemon 可执行文件和 Debug APK。
- 当前没有可用 Git 历史，验证以文件读取、测试、构建和真机检查为准。

## Task 1: Markdown and attachment presentation models

**Files:**
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/MarkdownText.kt`
- Create: `android/app/src/main/java/dev/pocketagent/app/ui/AttachmentPreview.kt`
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/MarkdownTextTest.kt`
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/AttachmentPreviewTest.kt`

**Interfaces:**
- `parseMarkdownForDisplay(source: String): List<MarkdownLine>` returns display lines for headings, bullets, paragraphs, inline code, bold spans and fenced code.
- `attachmentKind(name: String, mediaType: String): AttachmentKind` returns `IMAGE` or `FILE`.
- `formatAttachmentSize(bytes: Long): String` returns a compact user-facing size.

- [ ] Write tests that fail for raw `**bold**`, backtick code, bullets and fenced code, and for image/file classification and size formatting.
- [ ] Run the focused Android unit tests and confirm the failures are caused by missing production APIs.
- [ ] Implement the smallest parser/model used by Compose; preserve ordinary text and never emit markdown marker characters as standalone formatting markers.
- [ ] Run the focused tests and confirm they pass.

## Task 2: Render assistant messages and attachments in ChatScreen

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/HistoryMessageMapper.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt`

**Interfaces:**
- Assistant text is rendered through the Markdown display model.
- `Attachment` carries `name`, `pcPath`, `size`, and `mediaType`.
- User messages may contain an attachment preview plus the user text; attachment paths remain available to the agent.

- [ ] Load the current thread before attachment upload and keep its `workspace` in Compose state.
- [ ] Upload to `<thread.workspace>\\.pocketagent-uploads\\...`; only fall back to daemon default workspace when the thread has no workspace.
- [ ] Render image attachments with a thumbnail from the local URI before upload or a remote/file preview strategy after upload; render other files as a file card with name and size.
- [ ] Store attachment metadata in the in-memory user `ChatItem` and preserve the attachment path note in the `TurnInput`.
- [ ] Replace assistant `Text` rendering with Markdown line/span rendering while keeping tool and thinking cards visually distinct.
- [ ] Run Android unit tests, then compile the app.

## Task 3: Fix status-bar safe layout and recovery banner

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`

- [ ] Make the whole chat column apply `statusBarsPadding()` once at the top-level.
- [ ] Render load/recovery banners below the top bar, never before the status-bar-padded header.
- [ ] Keep the input area behind `imePadding()` and `navigationBarsPadding()` without changing its existing interaction.
- [ ] Add a Compose-level visual smoke check by installing the APK and capturing the chat screen after recovery state is triggered or represented.

## Task 4: Make approval interaction safe and protocol-correct

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/ui/TurnSession.kt`
- Modify: `daemon/internal/agent/codex.go`
- Modify: `daemon/internal/server/thread.go`
- Modify: `proto/v1/thread.proto`
- Modify: `android/app/src/main/proto/v1/thread.proto`
- Test: `android/app/src/test/java/dev/pocketagent/app/ui/ApprovalPresentationTest.kt`

**Interfaces:**
- The approval sheet cannot be dismissed by drag, outside tap, back navigation, or recomposition; only explicit buttons send a decision.
- `accept` is sent to Codex when approved; `decline` is sent when rejected or when no valid decision is available.
- A server-side pending approval remains associated with the thread while the mobile stream is disconnected; a reconnected client can answer it through the thread service.

- [ ] Write tests for collapsed-by-default command presentation and exact protocol decision mapping.
- [ ] Run them red.
- [ ] Add a `PendingApproval`/approval response RPC to the shared protobuf only if the existing stream cannot safely resume the request; persist enough request id/action/thread information to re-show it.
- [ ] Change the Codex bridge from `approved` to `accept`, retain `decline` for refusal, and preserve accepted-session semantics only when explicitly supported.
- [ ] Change server approval handling so a disconnected mobile stream does not immediately reject a pending approval; resume waiting until a decision, explicit cancellation, or a bounded server-side timeout, then keep the pending request visible for reconnect.
- [ ] Disable `ModalBottomSheet` drag and dismiss gestures; add a top-right expand/collapse icon and hide `jsonArgs` until expanded.
- [ ] Ensure failed approval sends leave the sheet visible with an error instead of closing it.
- [ ] Regenerate protobuf outputs using the project Gradle/Go generation path if the RPC changes.
- [ ] Run Android tests, Go tests, and build a fresh daemon executable.

## Task 5: Improve Tailscale/weak-network connection behavior

**Files:**
- Modify: `android/app/src/main/java/dev/pocketagent/app/AgentConnection.kt`
- Modify: `android/app/src/main/java/dev/pocketagent/app/MainActivity.kt` if lifecycle/network registration is needed
- Create or modify: `android/app/src/main/java/dev/pocketagent/app/NetworkMonitor.kt`
- Test: `android/app/src/test/java/dev/pocketagent/app/ConnectionDiagnosticsTest.kt`

- [ ] Add a network-change signal using Android `ConnectivityManager.NetworkCallback` and make it trigger a connection attempt without waiting for the periodic loop.
- [ ] Replace fixed 5-second retry with bounded exponential backoff and jitter, while keeping an 8-second healthy probe interval.
- [ ] Avoid calling `shutdownNow()` for every candidate transition unless the target address actually changes; do not destroy a healthy channel from a duplicate probe.
- [ ] Track current candidate and last failure independently so Tailscale address changes do not overwrite a stable LAN/USB candidate prematurely.
- [ ] Keep one `connectionEpoch` increment per actual down→up transition.
- [ ] Add diagnostics for candidate, attempt count, and transport so the UI explains whether the phone is on LAN, USB, or Tailscale.
- [ ] Run Android tests and verify with `tailscale ping`, daemon logs, and a real device reconnect smoke test.

## Task 6: Full verification and deployment

**Files:**
- Output: `android/app/build/outputs/apk/debug/app-debug.apk`
- Output: `daemon/pocketagentd.exe`

- [ ] Run `android/gradlew.bat testDebugUnitTest --no-daemon --rerun-tasks`.
- [ ] Run `C:\go\bin\go.exe test ./...` from `daemon`.
- [ ] Run `C:\go\bin\go.exe build -o pocketagentd.exe ./cmd/pocketagentd` from `daemon`.
- [ ] Run `android/gradlew.bat assembleDebug --no-daemon --rerun-tasks`.
- [ ] Install the fresh APK with `adb install -r`.
- [ ] Verify launch, home connection, settings navigation, chat rendering, attachment path, approval expand/collapse, explicit allow/deny, and process stability.
- [ ] Confirm the running daemon binary timestamp is newer than the source fix and inspect logs for absence of `approved`/`denied` protocol variants.
- [ ] Remove temporary screenshots and test artifacts that are no longer needed.


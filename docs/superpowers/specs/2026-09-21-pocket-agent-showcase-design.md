# PocketAgent Showcase Video Design

## Goal

制作一支约 45 秒、1080×1920、30fps 的竖屏产品展示片，展示 PocketAgent 的核心价值：手机是控制端，Agent 在用户自己的电脑上执行；视频必须使用已连接 Android 真机上的真实 PocketAgent 画面，并通过高级运镜把手机 UI、电脑端事件流和产品架构串成一个连续叙事。

## Audience and success criteria

目标观众是需要远程控制本地 coding agent 的开发者。视频完成的判断标准：

1. 前 5 秒明确 PocketAgent 是“手机控制、电脑执行”的产品。
2. 观众能看懂一次真实任务从手机输入、Agent 执行、审批到结果返回的闭环。
3. 至少展示 Chat、Terminal、Files、Git、Tasks 五类能力。
4. 画面具有明确的电影化运镜：推进、环绕、视差、焦点切换和连续转场，而不是静态截图轮播。
5. 不虚构性能数据、不把五柱图表伪装成统计结果、不把没有在手机上验证的能力剪成已完成事实。
6. 产出可编辑的 HyperFrames HTML 源文件、Remotion 合成工程、真机录屏和可复现的 MP4。

## Creative direction

Style name: **Pocket Drift**.

Palette:

- `#000000` — base black
- `#151517` — primary card
- `#1F1F22` — elevated card
- `#4A9DFF` — PocketAgent blue
- `#32D74B` — healthy/connected state
- `#B584FF` — MCP and agent intelligence layer
- `#F2F2F4` — primary text
- `#98989F` — secondary text

Typography:

- Display: `Barlow Condensed`, heavy weight, for short English title cards and large numerals.
- Body and UI annotation: `IBM Plex Mono`, regular/medium, for technical labels and event traces.
- Use at least 60px display type, 20px body type, and 16px data labels at the final 1080×1920 output.
- Use tabular numerals for timers, IDs, and bar labels.

Motion:

- The camera behaves like a product-film macro camera: slow dolly-in for meaning, short whip-pan for a topic shift, and restrained orbit around the phone.
- Use three depth layers for major shots: background event field, device/UI layer, foreground annotation layer.
- Use a continuous blue event line as a visual bridge between phone, daemon, and native agent.
- Vary easing and speed by scene. Avoid applying one easing or one stagger to every element.
- The primary scene transition is a medium-energy push/zoom with a blur or light-dip accent. The hero transition into the real phone demo is a cinematic zoom-through.

Anti-patterns:

- No generic purple-blue gradient wash across the whole canvas.
- No fake dashboards or invented percentages.
- No hard cuts between multi-scene sections unless the cut intentionally marks a register change.
- No UI text smaller than the video readability thresholds.
- No claim that a feature was demonstrated unless it is captured or clearly labeled as an architectural visualization.

## Storyboard

### Scene 1 — Signal in the dark (0–4s)

Black frame. A blue event trace appears, the camera follows it through a sparse field of technical labels, and the PocketAgent name resolves from shadow.

Copy:

```text
Your agent.
In your pocket.
```

### Scene 2 — The control loop (4–10s)

Three-dimensional composition showing:

```text
Phone → PocketAgent daemon → Native Agent
```

The event line carries labels such as `stream`, `approval`, and `result`. This is an architectural visualization grounded in the README, not a claim about a particular provider.

Copy:

```text
手机负责控制
Agent 在你的电脑上工作
```

### Scene 3 — Real phone demo (10–22s)

Use ADB to launch `dev.pocketagent.app`, capture a real screen recording, and record a short task flow. The intended sequence is:

1. Open a connected PocketAgent session.
2. Submit a short task prompt.
3. Show streaming reply or tool-card activity.
4. Show approval or result state if the live environment presents one.
5. If a specific state cannot be reproduced, use a verified evidence screenshot with a clear “产品界面” framing rather than fabricating a transition.

Camera treatment:

- Device body in front with subtle perspective rotation.
- Real screen as the high-detail focal plane.
- Push-in to the active control, then rack focus to the event trace behind it.
- Use a parallax crop for the phone screen so the footage remains readable in vertical format.

### Scene 4 — Five capability pillars (22–29s)

An animated five-bar chart acts as a feature map, not a quantitative chart:

```text
CHAT       ██████████
TERMINAL   ████████
FILES      ███████
GIT        ██████
TASKS      █████
```

Each bar rises from the timeline and briefly reveals a corresponding verified UI view or icon. Bars use the PocketAgent blue with green/purple semantic accents. The labels are feature categories from the project README; no numeric axis or performance unit is shown.

### Scene 5 — Connected, resilient, local (29–38s)

Show event traces reconnecting across:

- USB `adb reverse`
- LAN discovery
- event replay / resume
- approval recovery
- multi-agent switching

Only capabilities present in the README or captured evidence may be shown as product behavior. This scene can use a stylized network map but must not imply a cloud relay.

### Scene 6 — Resolve (38–45s)

The phone floats in a quiet black field. Background code and event traces settle. Finish with:

```text
Your computer.
Your agents.
Your pocket.

PocketAgent
```

End with a slow color dip to black.

## Technical architecture

The deliverable is one self-contained video workspace under `video/pocket-agent-showcase/`.

### HyperFrames

HyperFrames is the source of truth for the abstract product-film layer:

- `hyperframes/index.html` is the standalone root composition.
- `hyperframes/compositions/` contains scene-specific HTML when a scene benefits from isolated timing.
- Every timed clip has `id`, `data-start`, `data-duration`, and `data-track-index`.
- Every GSAP timeline is created synchronously, paused, and registered in `window.__timelines`.
- `hyperframes/DESIGN.md` repeats this spec's palette, typography, motion, and anti-patterns so the visual identity is traceable.

The HyperFrames source will render an abstract intro/architecture/feature layer used as a visual reference and, where practical, as a rendered media layer in the Remotion master.

### Remotion

Remotion is the final composition and rendering layer:

- Composition ID: `PocketAgentShowcase`.
- Size: `1080×1920`.
- Frame rate: `30`.
- Duration: `1350` frames.
- `useCurrentFrame()` and `interpolate()` drive all animation; no CSS animation or transition is used.
- The final composition layers HyperFrames-rendered media, real phone footage, phone frame/depth overlays, typography, the five-bar feature chart, and transitions.
- `public/` stores captured phone media and generated intermediate assets.

### Phone capture

ADB serial is discovered at runtime; the current device is `8e0e7bbe` with physical size `1080×2400`. Capture scripts must:

- verify that exactly one device is online;
- start from the installed `dev.pocketagent.app`;
- save captures only under the video workspace;
- preserve the raw recording before any crop or re-encode;
- record the exact command and screen state in a capture note.

If a live screen state is unavailable, use only evidence screenshots already present in the repository and label the scene as a UI showcase. Do not invent interaction results.

## Validation and delivery

Before delivery:

1. `npx hyperframes lint` passes.
2. `npx hyperframes validate` passes or has only intentionally documented warnings.
3. `npx hyperframes inspect` reports no unintended text overflow or off-canvas content.
4. A Remotion still render verifies the opening, phone demo, five-bar chart, and ending frames.
5. The Remotion composition bundles without TypeScript errors.
6. The final MP4 is rendered at 1080×1920, 30fps, and its duration is approximately 45 seconds.
7. `git status` confirms only files created by this video task are added or committed; existing PocketAgent changes remain untouched.

## Scope boundaries

This task creates a showcase video and its source assets. It does not modify PocketAgent Android or daemon behavior, does not change protocol contracts, does not add cloud services, and does not claim that the video itself is product verification evidence.

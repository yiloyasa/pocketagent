# PocketAgent Showcase Video

这是 PocketAgent 的 45 秒竖屏产品展示片工程，目标输出为 `1080×1920`、`30fps`、约 45 秒。

## 目录

- `storyboard.json`：唯一的场景时间轴和五项功能标签来源。
- `hyperframes/`：HyperFrames HTML/GSAP 抽象动画层。
- `remotion/`：Remotion 最终合成工程。
- `scripts/`：故事板、媒体和真机录制校验脚本。
- `public/captures/`：手机真机录屏与截图。
- `previews/`：代表性帧预览。
- `renders/`：最终 MP4（被 Git 忽略）。

## PowerShell 7 工作流

在本目录运行：

```powershell
npm install
npm run check
```

校验媒体：

```powershell
node .\scripts\verify-capture.mjs .\public\captures\raw\pocketagent-demo.mp4
```

启动 HyperFrames 预览：

```powershell
Set-Location .\hyperframes
npx hyperframes preview --port 3017
```

启动 Remotion Studio：

```powershell
Set-Location .\remotion
npm install
npm run dev
```

渲染代表性帧：

```powershell
Set-Location .\remotion
npm run still:opening
npm run still:phone
npm run still:pillars
npm run still:ending
```

渲染最终视频：

```powershell
npx remotion render PocketAgentShowcase .\renders\pocket-agent-showcase.mp4 --codec=h264 --crf=18
```

所有终端命令使用 PowerShell 7。现有 PocketAgent Android、daemon 和协议代码不属于本视频工程的修改范围。

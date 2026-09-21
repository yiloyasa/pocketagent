# PocketAgent Showcase Video Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and render a 45-second vertical PocketAgent product film that combines a HyperFrames abstract motion layer, real Android phone footage, a five-bar feature chart, and a Remotion master composition.

**Architecture:** Create one isolated video workspace at `video/pocket-agent-showcase/` without modifying PocketAgent Android, daemon, protocol, or existing dirty files. HyperFrames owns the HTML/GSAP abstract scenes and visual identity; Remotion owns the final 1080×1920 composition, real phone footage, camera moves, feature chart, and render. A small JSON storyboard plus validation scripts provide deterministic checks for scene coverage and captured assets.

**Tech Stack:** Node.js 22+, npm, HyperFrames CLI, GSAP, Remotion 4, React, TypeScript, FFmpeg, ADB, PowerShell 7.

**Spec:** `docs/superpowers/specs/2026-09-21-pocket-agent-showcase-design.md`

## Global Constraints

- Use the `Pocket Drift` palette from the spec: `#000000`, `#151517`, `#1F1F22`, `#4A9DFF`, `#32D74B`, `#B584FF`, `#F2F2F4`, `#98989F`.
- The final composition is `1080×1920`, `30fps`, and `1350` frames.
- The five bars represent feature categories, not quantitative performance data.
- Real phone media must come from the connected `dev.pocketagent.app`; do not fabricate interaction results.
- HyperFrames timelines must be synchronous, paused, registered in `window.__timelines`, and use `data-start`, `data-duration`, and `data-track-index`.
- Remotion animation must use `useCurrentFrame()` and `interpolate()`; CSS transitions and CSS animations are not allowed.
- Only add or commit files created by this video task; preserve all existing PocketAgent changes.
- Use PowerShell 7 for every command.

## Review Focus

- **Missing real-device media:** the video must still build when live capture is unavailable, using only verified repository evidence screenshots and a labeled UI showcase fallback. Test in Task 3.
- **Unreadable vertical crop:** the phone UI, feature labels, and final copy must remain inside the 1080×1920 safe area at representative frames. Test in Task 5.
- **HyperFrames/Remotion timing drift:** the abstract layer and master composition must agree on scene boundaries and total duration. Test in Tasks 2 and 5.
- **Fake chart semantics:** the five-bar scene must have no y-axis, unit, percentage, or invented metric claim. Test in Task 4.
- **Dirty-repository contamination:** unrelated PocketAgent changes must remain outside every video commit. Test in Task 1 and Task 6.

---

### Task 1: Scaffold the video workspace and deterministic checks

**Files:**
- Create: `video/pocket-agent-showcase/package.json`
- Create: `video/pocket-agent-showcase/storyboard.json`
- Create: `video/pocket-agent-showcase/scripts/validate-storyboard.mjs`
- Create: `video/pocket-agent-showcase/scripts/verify-capture.mjs`
- Create: `video/pocket-agent-showcase/.gitignore`
- Create: `video/pocket-agent-showcase/README.md`

**Interfaces:**
- `storyboard.json` produces the canonical scene IDs, time ranges, and feature labels consumed by HyperFrames and Remotion.
- `validate-storyboard.mjs` accepts no arguments, exits `0` only when scene ranges are contiguous, duration is 45 seconds at 30fps, and the feature list is exactly `Chat`, `Terminal`, `Files`, `Git`, `Tasks`.
- `verify-capture.mjs` accepts one media path and exits `0` only when the file exists, has nonzero size, and `ffprobe` reports video dimensions and duration.

- [ ] **Step 1: Define the failing storyboard contract**

Create `storyboard.json` with:

```json
{
  "compositionId": "PocketAgentShowcase",
  "width": 1080,
  "height": 1920,
  "fps": 30,
  "durationInFrames": 1350,
  "scenes": [
    {"id": "signal", "start": 0, "end": 120},
    {"id": "control-loop", "start": 120, "end": 300},
    {"id": "phone-demo", "start": 300, "end": 660},
    {"id": "feature-pillars", "start": 660, "end": 870},
    {"id": "resilient-local", "start": 870, "end": 1140},
    {"id": "resolve", "start": 1140, "end": 1350}
  ],
  "features": ["Chat", "Terminal", "Files", "Git", "Tasks"]
}
```

Write `validate-storyboard.mjs` so it intentionally fails if the scene IDs, range continuity, fps, dimensions, duration, or feature labels are changed.

- [ ] **Step 2: Run the contract check before the implementation exists**

Run:

```powershell
node .\scripts\validate-storyboard.mjs
```

Expected: `PASS storyboard contract` and exit code `0` after the file from Step 1 is present. The script itself is the executable contract for later tasks.

- [ ] **Step 3: Add the media verification helper**

Implement `verify-capture.mjs` with PowerShell-safe argument handling and `ffprobe` JSON parsing. It must reject missing files, empty files, non-video files, and videos without width/height/duration.

- [ ] **Step 4: Add workspace package scripts and ignore rules**

Use a package file with these scripts:

```json
{
  "scripts": {
    "check:storyboard": "node scripts/validate-storyboard.mjs",
    "check:capture": "node scripts/verify-capture.mjs",
    "check": "npm run check:storyboard"
  }
}
```

Ignore `node_modules/`, `renders/`, `.hyperframes/`, `public/captures/raw/`, and generated preview artifacts. Keep the raw capture directory itself present with a `.gitkeep`.

- [ ] **Step 5: Write the README**

Document the PowerShell 7 commands for installing dependencies, launching HyperFrames preview, launching Remotion Studio, checking the storyboard, verifying a capture, rendering a still, and rendering the final MP4. State that `storyboard.json` is the timing source.

- [ ] **Step 6: Verify and commit the scaffold**

Run:

```powershell
npm run check
git diff --check -- video/pocket-agent-showcase
git status --short
```

Expected: storyboard check exits `0`; diff check has no output; status shows only the new video workspace paths. Commit only those paths with:

```powershell
git add -- video/pocket-agent-showcase
git commit -m "chore: scaffold PocketAgent showcase video workspace"
```

---

### Task 2: Build the HyperFrames abstract motion layer

**Files:**
- Create: `video/pocket-agent-showcase/hyperframes/DESIGN.md`
- Create: `video/pocket-agent-showcase/hyperframes/index.html`
- Create: `video/pocket-agent-showcase/hyperframes/compositions/architecture.html`
- Create: `video/pocket-agent-showcase/hyperframes/compositions/pillars.html`
- Modify: `video/pocket-agent-showcase/storyboard.json` only if a timing correction is required by the spec

**Interfaces:**
- `hyperframes/index.html` is a standalone root composition with `data-composition-id="pocket-agent-showcase-abstract"`.
- `architecture.html` and `pillars.html` are template-backed sub-compositions loaded with `data-composition-src`.
- HyperFrames output is rendered to `public/hyperframes/abstract-layer.mp4` for the Remotion master.

- [ ] **Step 1: Write the visual identity file**

Create `DESIGN.md` with the exact `Pocket Drift` palette, Barlow Condensed / IBM Plex Mono typography, cinematic motion rules, and anti-patterns from the spec. Include a short “What this layer does not claim” section explaining that the five bars are feature categories.

- [ ] **Step 2: Create the static hero layouts**

Build the end-state HTML/CSS before writing GSAP:

- Scene 1 uses a left-anchored title and a large event trace crossing the frame.
- Scene 2 uses a three-node control loop with a phone node, daemon node, and native-agent node.
- Scene 4 uses five labeled vertical bars on a split frame with no quantitative axis.
- Every `.scene-content` fills its composition with flex layout and padding rather than a hardcoded absolute content container.

- [ ] **Step 3: Add deterministic GSAP timelines**

Register `window.__timelines["pocket-agent-showcase-abstract"]`, `window.__timelines["architecture"]`, and `window.__timelines["pillars"]`. Add offset entrances, varied eases, one ambient motion per scene, and finite repeats only. Use transitions between every scene and no intermediate exit animations.

- [ ] **Step 4: Run HyperFrames static validation**

From `video/pocket-agent-showcase/hyperframes`, run:

```powershell
npx hyperframes lint
npx hyperframes validate
npx hyperframes inspect --json
```

Expected: no lint errors, no unaddressed contrast failures, and no unintended text overflow or off-canvas elements. If `inspect` reports intentional entrance overflow, mark only the relevant decorative element with `data-layout-allow-overflow`.

- [ ] **Step 5: Render the abstract layer**

Run:

```powershell
npx hyperframes render --output ..\public\hyperframes\abstract-layer.mp4 --fps 30 --quality standard
```

Expected: an MP4 exists at the exact output path and passes the media verifier from Task 1.

- [ ] **Step 6: Commit the HyperFrames layer**

Run:

```powershell
node ..\scripts\verify-capture.mjs ..\public\hyperframes\abstract-layer.mp4
git diff --check -- hyperframes
git add -- hyperframes public/hyperframes/abstract-layer.mp4
git commit -m "feat: add HyperFrames PocketAgent motion layer"
```

---

### Task 3: Capture and verify the real phone demo

**Files:**
- Create: `video/pocket-agent-showcase/scripts/capture-phone-demo.ps1`
- Create: `video/pocket-agent-showcase/captures/capture-notes.md`
- Create: `video/pocket-agent-showcase/public/captures/raw/pocketagent-demo.mp4`
- Create: `video/pocket-agent-showcase/public/captures/screenshots/phone-demo-start.png`
- Create: `video/pocket-agent-showcase/public/captures/screenshots/phone-demo-end.png`

**Interfaces:**
- `capture-phone-demo.ps1` accepts `-Serial`, `-Package`, and `-Seconds` parameters and writes the raw MP4 plus two PNG screenshots.
- The script must leave the device in the PocketAgent app, but it must not change project files or install an APK.
- `capture-notes.md` records the device serial, package, exact commands, observed screen states, and whether an approval card appeared.

- [ ] **Step 1: Add a preflight script that fails on missing or multiple devices**

Implement:

```powershell
param(
  [string]$Serial = "",
  [string]$Package = "dev.pocketagent.app",
  [int]$Seconds = 12
)
```

The script must call `adb devices`, select one online device, resolve `$Serial` if omitted, verify the package exists, and create the raw/screenshot directories.

- [ ] **Step 2: Launch PocketAgent and capture the start state**

Use:

```powershell
adb -s $Serial shell am force-stop $Package
adb -s $Serial shell monkey -p $Package 1
Start-Sleep -Seconds 2
adb -s $Serial exec-out screencap -p > .\public\captures\screenshots\phone-demo-start.png
```

If the app opens to onboarding or an unconnected state, record that fact in `capture-notes.md` and use the verified state rather than faking a connected session.

- [ ] **Step 3: Record the real interaction window**

Start `adb shell screenrecord --size 1080x2400 --bit-rate 12000000 --time-limit $Seconds /sdcard/pocketagent-demo.mp4`, observe the app through the connected device, and stop after the actual demo window. Pull the result with `adb pull` into `public/captures/raw/pocketagent-demo.mp4`.

- [ ] **Step 4: Capture the end state and write notes**

Capture `phone-demo-end.png`, run `adb shell dumpsys activity activities` to identify the foreground activity, and write the exact sequence into `capture-notes.md`. Do not describe a feature as demonstrated unless it is visible in the recording or screenshot.

- [ ] **Step 5: Verify media and dimensions**

Run:

```powershell
node .\scripts\verify-capture.mjs .\public\captures\raw\pocketagent-demo.mp4
ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,duration -of json .\public\captures\raw\pocketagent-demo.mp4
```

Expected: nonzero duration, width `1080`, height `2400`, and a valid frame rate. If live capture is unavailable, copy only verified repository evidence into `public/captures/screenshots/` and record `live capture unavailable` in the notes; the Remotion scene will use a clearly labeled screenshot fallback.

- [ ] **Step 6: Commit only the capture assets and script**

Run:

```powershell
git diff --check -- video/pocket-agent-showcase
git add -- video/pocket-agent-showcase/scripts/capture-phone-demo.ps1 video/pocket-agent-showcase/captures video/pocket-agent-showcase/public/captures
git commit -m "feat: capture PocketAgent phone demo assets"
```

---

### Task 4: Build the Remotion master and five-bar feature scene

**Files:**
- Create: `video/pocket-agent-showcase/remotion/package.json`
- Create: `video/pocket-agent-showcase/remotion/tsconfig.json`
- Create: `video/pocket-agent-showcase/remotion/src/index.ts`
- Create: `video/pocket-agent-showcase/remotion/src/Root.tsx`
- Create: `video/pocket-agent-showcase/remotion/src/PocketAgentShowcase.tsx`
- Create: `video/pocket-agent-showcase/remotion/src/storyboard.ts`
- Create: `video/pocket-agent-showcase/remotion/src/components/PhoneFrame.tsx`
- Create: `video/pocket-agent-showcase/remotion/src/components/FeaturePillars.tsx`
- Create: `video/pocket-agent-showcase/remotion/src/components/EventTrace.tsx`
- Create: `video/pocket-agent-showcase/remotion/src/styles/colors.ts`
- Create: `video/pocket-agent-showcase/remotion/public/` asset links or copied media references

**Interfaces:**
- `Root.tsx` registers `PocketAgentShowcase` at 1080×1920, 30fps, 1350 frames.
- `PocketAgentShowcase` consumes `storyboard.json` timing and renders all six scenes.
- `FeaturePillars` accepts `{features: string[], values: number[]}` but renders the bars without units, axes, percentages, or metric claims.
- `PhoneFrame` accepts a real capture path or a verified screenshot fallback and exposes a stable focal rectangle for camera zooms.

- [ ] **Step 1: Scaffold the Remotion package**

Create the package with Remotion 4 dependencies and scripts:

```json
{
  "scripts": {
    "dev": "remotion studio --no-open",
    "typecheck": "tsc --noEmit",
    "still:opening": "remotion still PocketAgentShowcase --frame=60 --scale=0.25",
    "still:phone": "remotion still PocketAgentShowcase --frame=450 --scale=0.25",
    "still:pillars": "remotion still PocketAgentShowcase --frame=760 --scale=0.25",
    "still:ending": "remotion still PocketAgentShowcase --frame=1240 --scale=0.25"
  }
}
```

Add `@remotion/media` for video playback and use `staticFile()` for every public asset.

- [ ] **Step 2: Add the root composition and color system**

Implement a typed `Composition` registration and literal color constants matching `Pocket Drift`. Do not use CSS `animation`, CSS `transition`, or browser-time APIs. Keep all animation values derived from `useCurrentFrame()` and `useVideoConfig()`.

- [ ] **Step 3: Implement the five-bar feature chart first**

Use frame-based interpolation for each bar height and label entrance:

```tsx
const barHeight = interpolate(
  frame,
  [startFrame, startFrame + 18, startFrame + 42],
  [0, targetHeight, targetHeight],
  {extrapolateLeft: "clamp", extrapolateRight: "clamp"}
);
```

Use target heights `[0.94, 0.78, 0.68, 0.58, 0.48]` only as visual composition values. Keep labels exactly `CHAT`, `TERMINAL`, `FILES`, `GIT`, and `TASKS`. No axis, unit, percentage, or “score” copy may appear.

- [ ] **Step 4: Implement the phone frame and cinematic camera**

Compose the phone capture inside a perspective frame with:

- `rotateX`, `rotateY`, and `rotateZ` driven by frame interpolation;
- foreground glass/reflection layer;
- background event trace and blur;
- a push-in toward the screen focal rectangle during the phone demo;
- a focus pull from the screen to the event trace;
- safe-area clipping so no phone content crosses the 1080×1920 frame.

- [ ] **Step 5: Implement scene transitions and title cards**

Use scene-local `Sequence` ranges from `storyboard.json`. Every scene must have an entrance. Use a zoom-through or blur/dip transition between scenes; the final scene may fade to black. Keep the copy short enough to read in under two seconds.

- [ ] **Step 6: Run TypeScript checks**

From `video/pocket-agent-showcase/remotion`, run:

```powershell
npm install
npm run typecheck
```

Expected: TypeScript exits `0` with no implicit-any errors or missing asset imports.

- [ ] **Step 7: Commit the Remotion source**

Run:

```powershell
git diff --check -- remotion
git add -- remotion
git commit -m "feat: build Remotion PocketAgent showcase composition"
```

---

### Task 5: Integrate layers, preview, and render representative frames

**Files:**
- Modify: `video/pocket-agent-showcase/remotion/src/PocketAgentShowcase.tsx`
- Modify: `video/pocket-agent-showcase/remotion/src/components/PhoneFrame.tsx`
- Create: `video/pocket-agent-showcase/remotion/public/hyperframes/abstract-layer.mp4` link or copied asset
- Create: `video/pocket-agent-showcase/remotion/public/captures/pocketagent-demo.mp4` link or copied asset
- Create: `video/pocket-agent-showcase/previews/opening.png`
- Create: `video/pocket-agent-showcase/previews/phone-demo.png`
- Create: `video/pocket-agent-showcase/previews/feature-pillars.png`
- Create: `video/pocket-agent-showcase/previews/ending.png`

**Interfaces:**
- Remotion uses the HyperFrames MP4 and phone capture from `public/`.
- Preview stills are named by storyboard scene and are the visual QA evidence for safe-area and readability checks.

- [ ] **Step 1: Copy or reference generated media**

Copy `public/hyperframes/abstract-layer.mp4` and the verified phone capture into the Remotion public directory. Use deterministic filenames and do not copy unrelated files.

- [ ] **Step 2: Wire the abstract layer and phone footage**

Place the HyperFrames layer behind the camera scene or use it for the abstract scenes, then place the phone footage only in the phone-demo range. Add a fallback branch that uses `phone-demo-start.png` when the raw capture is unavailable, and render a small `UI SHOWCASE` label in that branch.

- [ ] **Step 3: Render four representative stills**

Run:

```powershell
npm run still:opening
npm run still:phone
npm run still:pillars
npm run still:ending
```

Expected: all four PNGs are created under `previews/` or the configured Remotion output path. Inspect them with the image viewer and correct any clipped copy, unreadable labels, phone bleed, or awkward camera framing.

- [ ] **Step 4: Run the Remotion Studio preview**

Run:

```powershell
npm run dev
```

Open the exact local Studio URL in the Codex browser, scrub each storyboard scene, and confirm the scene timing, transition continuity, and phone readability.

- [ ] **Step 5: Run the animation and asset checks**

Run:

```powershell
npm run typecheck
node ..\scripts\validate-storyboard.mjs
node ..\scripts\verify-capture.mjs .\public\captures\pocketagent-demo.mp4
```

Expected: all checks exit `0`. If the raw phone clip is absent, use the screenshot fallback and record the reason in `captures/capture-notes.md`.

- [ ] **Step 6: Commit the integrated preview state**

Run:

```powershell
git diff --check -- remotion previews
git add -- remotion previews captures/capture-notes.md
git commit -m "feat: integrate phone footage and abstract motion layer"
```

---

### Task 6: Render final MP4 and complete repository-safe QA

**Files:**
- Create: `video/pocket-agent-showcase/renders/pocket-agent-showcase.mp4`
- Create: `video/pocket-agent-showcase/renders/pocket-agent-showcase-metadata.json`
- Modify: `video/pocket-agent-showcase/README.md`

**Interfaces:**
- Final MP4 is rendered by the Remotion CLI from `PocketAgentShowcase`.
- Metadata records composition ID, width, height, fps, frame count, duration, source capture path, and verification commands.

- [ ] **Step 1: Render the final MP4**

Run:

```powershell
npx remotion render PocketAgentShowcase ..\renders\pocket-agent-showcase.mp4 --codec=h264 --crf=18
```

Expected: exit `0` and a nonzero MP4 at `video/pocket-agent-showcase/renders/pocket-agent-showcase.mp4`.

- [ ] **Step 2: Verify final media metadata**

Run:

```powershell
ffprobe -v error -show_entries format=duration:stream=width,height,r_frame_rate,codec_name -of json .\renders\pocket-agent-showcase.mp4
```

Write `renders/pocket-agent-showcase-metadata.json` with the observed values. Confirm width `1080`, height `1920`, frame rate `30/1`, H.264 video, and duration within 0.5 seconds of 45 seconds.

- [ ] **Step 3: Run the final clean checks**

Run:

```powershell
npm run typecheck
node ..\scripts\validate-storyboard.mjs
node ..\scripts\verify-capture.mjs .\renders\pocket-agent-showcase.mp4
git diff --check
git status --short
```

Expected: all checks exit `0`; `git diff --check` is empty; any status entries are limited to this task's video files and the already-existing PocketAgent modifications.

- [ ] **Step 4: Document the delivery**

Update the video README with the exact preview command, final render path, whether live phone footage or the verified screenshot fallback was used, and the observed render metadata. Do not claim features beyond the capture notes and spec.

- [ ] **Step 5: Commit the final artifact metadata**

Commit only:

```powershell
git add -- video/pocket-agent-showcase/README.md video/pocket-agent-showcase/renders/pocket-agent-showcase-metadata.json
git commit -m "feat: render PocketAgent showcase video"
```

- [ ] **Step 6: Perform the final review**

Review the final diff against the spec, check all five Review Focus items, and verify that no existing PocketAgent file appears in any video-task commit. Record the final render path and any limitation in the final response.

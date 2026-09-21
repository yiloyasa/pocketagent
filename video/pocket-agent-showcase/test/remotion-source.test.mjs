import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.join(process.cwd(), "remotion");

test("Remotion source declares the vertical PocketAgent composition contract", () => {
  const rootSource = fs.readFileSync(path.join(root, "src", "Root.tsx"), "utf8");
  const compositionSource = fs.readFileSync(
    path.join(root, "src", "PocketAgentShowcase.tsx"),
    "utf8",
  );
  assert.match(rootSource, /id=["']PocketAgentShowcase["']/);
  assert.match(rootSource, /width=\{1080\}/);
  assert.match(rootSource, /height=\{1920\}/);
  assert.match(rootSource, /fps=\{30\}/);
  assert.match(rootSource, /durationInFrames=\{1350\}/);
  assert.match(compositionSource, /useCurrentFrame/);
  assert.match(compositionSource, /interpolate/);
  assert.doesNotMatch(compositionSource, /animation\s*:/);
  assert.doesNotMatch(compositionSource, /transition\s*:/);
});

test("feature pillar source keeps five labels and avoids metric claims", () => {
  const source = fs.readFileSync(
    path.join(root, "src", "components", "FeaturePillars.tsx"),
    "utf8",
  );
  const storyboard = fs.readFileSync(
    path.join(root, "src", "storyboard.ts"),
    "utf8",
  );
  for (const label of ["CHAT", "TERMINAL", "FILES", "GIT", "TASKS"]) {
    assert.match(storyboard, new RegExp(label[0] + label.slice(1).toLowerCase()));
  }
  assert.match(source, /feature\.toUpperCase/);
  assert.doesNotMatch(source, /%|latency|score|performance|units?/i);
});

test("phone camera keyframes stay valid for the short final scene", () => {
  const source = fs.readFileSync(
    path.join(root, "src", "components", "PhoneFrame.tsx"),
    "utf8",
  );
  assert.match(source, /Math\.min\(220/);
});

test("scene surfaces crossfade instead of stacking across the timeline", () => {
  const source = fs.readFileSync(
    path.join(root, "src", "PocketAgentShowcase.tsx"),
    "utf8",
  );
  assert.match(source, /sceneOpacity/);
  assert.match(source, /\[0,\s*1,\s*1,\s*0\]/);
});

test("ending phone composition stays inside the vertical safe area", () => {
  const source = fs.readFileSync(
    path.join(root, "src", "PocketAgentShowcase.tsx"),
    "utf8",
  );
  assert.match(source, /right:\s*22/);
  assert.match(source, /0\.58\s*\+\s*resolveIn/);
});

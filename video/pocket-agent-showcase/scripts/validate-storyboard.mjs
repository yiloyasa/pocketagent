import fs from "node:fs";
import path from "node:path";
import {fileURLToPath, pathToFileURL} from "node:url";

const EXPECTED_FEATURES = ["Chat", "Terminal", "Files", "Git", "Tasks"];

export function validateStoryboard(storyboard) {
  if (storyboard.compositionId !== "PocketAgentShowcase") {
    throw new Error("Unexpected compositionId");
  }
  if (storyboard.width !== 1080 || storyboard.height !== 1920) {
    throw new Error("Storyboard must be 1080x1920");
  }
  if (storyboard.fps !== 30 || storyboard.durationInFrames !== 1350) {
    throw new Error("Storyboard must be 30fps and 1350 frames");
  }

  const scenes = storyboard.scenes;
  if (!Array.isArray(scenes) || scenes.length !== 6) {
    throw new Error("Storyboard must contain six scenes");
  }

  scenes.forEach((scene, index) => {
    if (!scene.id || !Number.isInteger(scene.start) || !Number.isInteger(scene.end)) {
      throw new Error(`Scene ${index} must have an id and integer range`);
    }
    if (scene.end <= scene.start) {
      throw new Error(`Scene ${scene.id} must have a positive duration`);
    }
    if (index === 0 && scene.start !== 0) {
      throw new Error("The first scene must start at frame 0");
    }
    if (index > 0) {
      const previous = scenes[index - 1];
      if (previous.end !== scene.start) {
        throw new Error(
          `Scene ${previous.id} must end where ${scene.id} starts`
        );
      }
    }
  });

  const lastScene = scenes.at(-1);
  if (lastScene.end !== storyboard.durationInFrames) {
    throw new Error("The last scene must end at durationInFrames");
  }

  if (
    !Array.isArray(storyboard.features) ||
    JSON.stringify(storyboard.features) !== JSON.stringify(EXPECTED_FEATURES)
  ) {
    throw new Error("Feature labels must be Chat, Terminal, Files, Git, Tasks");
  }

  return true;
}

function isMainModule() {
  const entry = process.argv[1];
  return entry && import.meta.url === pathToFileURL(path.resolve(entry)).href;
}

if (isMainModule()) {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  const storyboardPath = path.join(root, "storyboard.json");
  const storyboard = JSON.parse(fs.readFileSync(storyboardPath, "utf8"));
  validateStoryboard(storyboard);
  console.log("PASS storyboard contract");
}

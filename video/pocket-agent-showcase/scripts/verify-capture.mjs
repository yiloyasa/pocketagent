import fs from "node:fs";
import path from "node:path";
import {spawnSync} from "node:child_process";
import {fileURLToPath, pathToFileURL} from "node:url";

export function verifyProbeResult(probe) {
  const stream = probe?.streams?.find((candidate) => {
    return candidate.codec_type === undefined || candidate.codec_type === "video";
  });
  if (!stream) {
    throw new Error("Capture must contain a video stream");
  }
  if (!Number.isFinite(Number(stream.width)) || !Number.isFinite(Number(stream.height))) {
    throw new Error("Video stream must report width and height");
  }

  const duration = Number(probe?.format?.duration ?? stream.duration);
  if (!Number.isFinite(duration) || duration <= 0) {
    throw new Error("Capture must report a positive duration");
  }
  return {width: Number(stream.width), height: Number(stream.height), duration};
}

export function verifyCapture(mediaPath) {
  if (!mediaPath) {
    throw new Error("Usage: node scripts/verify-capture.mjs <media-path>");
  }
  const resolved = path.resolve(mediaPath);
  if (!fs.existsSync(resolved)) {
    throw new Error(`Capture does not exist: ${resolved}`);
  }
  if (fs.statSync(resolved).size === 0) {
    throw new Error(`Capture is empty: ${resolved}`);
  }

  const probe = spawnSync(
    "ffprobe",
    [
      "-v",
      "error",
      "-select_streams",
      "v:0",
      "-show_entries",
      "stream=codec_type,width,height,duration:format=duration",
      "-of",
      "json",
      resolved,
    ],
    {encoding: "utf8"}
  );
  if (probe.error) {
    throw new Error(`ffprobe failed to start: ${probe.error.message}`);
  }
  if (probe.status !== 0) {
    throw new Error(probe.stderr.trim() || "ffprobe returned a non-zero exit code");
  }

  return verifyProbeResult(JSON.parse(probe.stdout));
}

function isMainModule() {
  const entry = process.argv[1];
  return entry && import.meta.url === pathToFileURL(path.resolve(entry)).href;
}

if (isMainModule()) {
  const result = verifyCapture(process.argv[2]);
  console.log(
    `PASS capture ${result.width}x${result.height} ${result.duration.toFixed(2)}s`
  );
}

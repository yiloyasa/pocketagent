import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const scriptPath = path.join(
  process.cwd(),
  "scripts",
  "capture-phone-demo.ps1",
);

test("capture script exposes the approved device capture contract", () => {
  assert.equal(fs.existsSync(scriptPath), true);
  const source = fs.readFileSync(scriptPath, "utf8");
  assert.match(source, /\[string\]\$Serial/);
  assert.match(source, /\[string\]\$Package\s*=\s*"dev\.pocketagent\.app"/);
  assert.match(source, /screenrecord --size 1080x2400/);
  assert.match(source, /screencap -p/);
  assert.match(source, /dumpsys activity activities/);
});

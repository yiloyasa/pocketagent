import test from "node:test";
import assert from "node:assert/strict";
import {verifyProbeResult} from "../scripts/verify-capture.mjs";

test("accepts a media probe with video dimensions and duration", () => {
  assert.doesNotThrow(() =>
    verifyProbeResult({
      streams: [{width: 1080, height: 2400, duration: "12.5"}],
      format: {duration: "12.5"},
    })
  );
});

test("rejects media without a video stream", () => {
  assert.throws(
    () => verifyProbeResult({streams: [], format: {duration: "12.5"}}),
    /video stream/
  );
});

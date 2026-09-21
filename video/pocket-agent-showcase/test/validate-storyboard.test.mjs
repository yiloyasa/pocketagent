import test from "node:test";
import assert from "node:assert/strict";
import storyboard from "../storyboard.json" with {type: "json"};
import {validateStoryboard} from "../scripts/validate-storyboard.mjs";

test("accepts the approved 45-second six-scene storyboard", () => {
  assert.doesNotThrow(() => validateStoryboard(storyboard));
});

test("rejects a storyboard with a timing gap", () => {
  const broken = structuredClone(storyboard);
  broken.scenes[2].start += 1;

  assert.throws(
    () => validateStoryboard(broken),
    /Scene control-loop must end where phone-demo starts/
  );
});

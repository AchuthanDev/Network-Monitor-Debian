import test from "node:test";
import assert from "node:assert/strict";

test("bootstrap tests are wired", () => {
  assert.equal("internet", "internet");
});

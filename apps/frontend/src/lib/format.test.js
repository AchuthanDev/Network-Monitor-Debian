import test from "node:test";
import assert from "node:assert/strict";

function formatBytes(bytes) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = Math.max(bytes, 0);
  let index = 0;
  while (value >= 1000 && index < units.length - 1) {
    value /= 1000;
    index += 1;
  }
  const digits = value >= 100 || index === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${units[index]}`;
}

test("formatBytes uses decimal units", () => {
  assert.equal(formatBytes(170396167), "170 MB");
  assert.equal(formatBytes(5393453), "5.39 MB");
});

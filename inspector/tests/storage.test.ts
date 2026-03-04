import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  flattenStorageSnapshots,
  formatStorageValue,
  isStorageAreaFilter,
  normalizeStorageSnapshots
} from "../src/storage";
import type { QueryStorageResult, StorageSnapshot } from "../../shared/protocol/src/index";

describe("storage snapshot normalization", () => {
  it("normalizes valid snapshots and drops invalid entries", () => {
    const payload = {
      snapshots: [
        { area: " Local ", items: { alpha: 1 } },
        { area: "managed", items: { beta: true } },
        { area: "", items: { skip: true } },
        { area: "sync", items: [] },
        { area: "session", items: null },
        null
      ]
    } as unknown as QueryStorageResult;

    const normalized = normalizeStorageSnapshots(payload);
    assert.deepEqual(normalized, [
      { area: "local", items: { alpha: 1 } },
      { area: "managed", items: { beta: true } }
    ]);
  });
});

describe("storage row flattening", () => {
  const snapshots: StorageSnapshot[] = [
    { area: "sync", items: { beta: 2, alpha: 1 } },
    { area: "local", items: { gamma: true } }
  ];

  it("sorts rows by area then key", () => {
    const rows = flattenStorageSnapshots(snapshots, "all");
    assert.deepEqual(
      rows.map((row) => `${row.area}:${row.key}`),
      ["local:gamma", "sync:alpha", "sync:beta"]
    );
  });

  it("applies area filtering", () => {
    const rows = flattenStorageSnapshots(snapshots, "sync");
    assert.equal(rows.length, 2);
    assert.equal(rows[0].area, "sync");
    assert.equal(rows[1].area, "sync");
  });
});

describe("storage value formatting", () => {
  it("formats primitives and objects", () => {
    assert.equal(formatStorageValue("value"), "value");
    assert.equal(formatStorageValue(3), "3");
    assert.equal(formatStorageValue(null), "null");
    assert.equal(formatStorageValue({ key: "value" }), "{\"key\":\"value\"}");
  });

  it("falls back to String(value) for circular structures", () => {
    const circular: { self?: unknown } = {};
    circular.self = circular;
    assert.equal(formatStorageValue(circular), "[object Object]");
  });
});

describe("storage area filter guard", () => {
  it("accepts known filters and rejects unknown values", () => {
    assert.equal(isStorageAreaFilter("all"), true);
    assert.equal(isStorageAreaFilter("local"), true);
    assert.equal(isStorageAreaFilter("managed"), false);
  });
});

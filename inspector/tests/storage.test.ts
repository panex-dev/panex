import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  applyStorageDiff,
  flattenStorageSnapshots,
  formatStorageValue,
  isStorageAreaFilter,
  normalizeStorageSnapshots,
  storageRowID
} from "../src/storage";
import type { QueryStorageResult, StorageDiff, StorageSnapshot } from "@panex/protocol";

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
    assert.equal(rows[0].rowID, storageRowID("local", "gamma"));
  });

  it("applies area filtering", () => {
    const rows = flattenStorageSnapshots(snapshots, "sync");
    assert.equal(rows.length, 2);
    assert.equal(rows[0].area, "sync");
    assert.equal(rows[1].area, "sync");
  });
});

describe("storage diff application", () => {
  it("applies set/remove operations and reports changed row IDs", () => {
    const snapshots: StorageSnapshot[] = [{ area: "local", items: { a: 1, b: 2 } }];
    const diff: StorageDiff = {
      area: "local",
      changes: [{ key: "a", new_value: 10 }, { key: "b" }, { key: "c", new_value: true }]
    };

    const next = applyStorageDiff(snapshots, diff);
    assert.deepEqual(next.snapshots, [{ area: "local", items: { a: 10, c: true } }]);
    assert.deepEqual(next.changedRowIDs, ["local:a", "local:b", "local:c"]);
  });

  it("creates missing area snapshot when receiving first diff", () => {
    const next = applyStorageDiff([], {
      area: "sync",
      changes: [{ key: "feature", new_value: "on" }]
    });

    assert.deepEqual(next.snapshots, [{ area: "sync", items: { feature: "on" } }]);
    assert.deepEqual(next.changedRowIDs, ["sync:feature"]);
  });

  it("ignores malformed payloads", () => {
    const snapshots: StorageSnapshot[] = [{ area: "local", items: { a: 1 } }];

    const malformed = { area: "", changes: [{ key: "a", new_value: 2 }] } as StorageDiff;
    const next = applyStorageDiff(snapshots, malformed);
    assert.deepEqual(next.snapshots, snapshots);
    assert.deepEqual(next.changedRowIDs, []);
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

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { StorageSnapshot } from "@panex/protocol";

import { buildWorkbenchModel, summarizeStorageAreas, summarizeTimeline } from "../src/workbench";
import type { TimelineEntry } from "../src/timeline";

describe("summarizeStorageAreas", () => {
  it("orders preferred areas first and counts keys per area", () => {
    const snapshots: StorageSnapshot[] = [
      { area: "session", items: { c: true } },
      { area: "custom", items: { x: 1, y: 2 } },
      { area: "local", items: { a: 1, b: 2 } },
      { area: "sync", items: {} }
    ];

    assert.deepEqual(summarizeStorageAreas(snapshots), [
      { area: "local", keys: 2 },
      { area: "sync", keys: 0 },
      { area: "session", keys: 1 },
      { area: "custom", keys: 2 }
    ]);
  });
});

describe("summarizeTimeline", () => {
  it("counts persisted/live events and picks the latest event by timestamp", () => {
    const timeline: TimelineEntry[] = [
      entry("build.complete", 100, 11),
      entry("storage.diff", 140),
      entry("query.storage.result", 120, 18)
    ];

    assert.deepEqual(summarizeTimeline(timeline), {
      totalEvents: 3,
      liveEvents: 1,
      persistedEvents: 2,
      latestEventName: "storage.diff",
      latestEventSource: "daemon:daemon-1"
    });
  });

  it("returns empty-state values when there are no events", () => {
    assert.deepEqual(summarizeTimeline([]), {
      totalEvents: 0,
      liveEvents: 0,
      persistedEvents: 0,
      latestEventName: null,
      latestEventSource: null
    });
  });
});

describe("buildWorkbenchModel", () => {
  it("combines connection, storage, and timeline state into a stable view model", () => {
    const model = buildWorkbenchModel({
      status: "open",
      socketURL: "ws://127.0.0.1:4317/ws?token=dev-token",
      lastError: null,
      storage: [
        { area: "local", items: { theme: "dark" } },
        { area: "sync", items: {} }
      ],
      timeline: [entry("query.events.result", 99, 5)]
    });

    assert.equal(model.status, "open");
    assert.equal(model.socketURL, "ws://127.0.0.1:4317/ws?token=dev-token");
    assert.equal(model.totalStorageKeys, 1);
    assert.deepEqual(model.storageAreas, [
      { area: "local", keys: 1 },
      { area: "sync", keys: 0 }
    ]);
    assert.equal(model.timeline.latestEventName, "query.events.result");
  });
});

function entry(name: string, recordedAtMS: number, id?: number): TimelineEntry {
  return {
    key: typeof id === "number" ? `db-${id}` : `live-${recordedAtMS}`,
    id,
    recordedAtMS,
    envelope: {
      v: 1,
      t: "event",
      name: name as TimelineEntry["envelope"]["name"],
      src: { role: "daemon", id: "daemon-1" },
      data: {}
    }
  };
}

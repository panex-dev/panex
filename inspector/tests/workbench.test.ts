import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { StorageSnapshot } from "@panex/protocol";

import {
  buildWorkbenchModel,
  summarizeRuntimeProbe,
  summarizeStorageAreas,
  summarizeStoragePresets,
  summarizeTimeline
} from "../src/workbench";
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
        {
          area: "local",
          items: {
            "panex.workbench.featureFlag": {
              enabled: false,
              rollout: 1,
              source: "custom"
            }
          }
        },
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
    assert.equal(model.storagePresets[0]?.state, "customized");
    assert.equal(model.storagePresets[0]?.actionLabel, "update");
    assert.equal(model.runtimeProbe.lastResultText, null);
    assert.equal(model.timeline.latestEventName, "query.events.result");
  });
});

describe("summarizeStoragePresets", () => {
  it("marks missing, applied, and customized preset states from storage snapshots", () => {
    const snapshots: StorageSnapshot[] = [
      {
        area: "local",
        items: {
          "panex.workbench.featureFlag": {
            source: "workbench",
            rollout: 1,
            enabled: true
          }
        }
      },
      {
        area: "sync",
        items: {
          "panex.workbench.layoutProfile": {
            theme: "midnight",
            density: "compact"
          }
        }
      }
    ];

    assert.deepEqual(
      summarizeStoragePresets(snapshots).map((preset) => ({
        id: preset.id,
        state: preset.state,
        actionLabel: preset.actionLabel,
        currentValueText: preset.currentValueText
      })),
      [
        {
          id: "local-feature-flag",
          state: "applied",
          actionLabel: "remove",
          currentValueText: '{"source":"workbench","rollout":1,"enabled":true}'
        },
        {
          id: "sync-layout-profile",
          state: "customized",
          actionLabel: "update",
          currentValueText: '{"theme":"midnight","density":"compact"}'
        },
        {
          id: "session-query-draft",
          state: "missing",
          actionLabel: "apply",
          currentValueText: null
        }
      ]
    );
  });
});

describe("summarizeRuntimeProbe", () => {
  it("captures the latest runtime probe result and runtime.onMessage event from timeline entries", () => {
    const timeline: TimelineEntry[] = [
      eventEntry("chrome.api.result", 100, {
        call_id: "runtime-send-1",
        success: true,
        data: {
          kind: "panex.workbench.runtime-probe",
          probe_id: "runtime-ping",
          topic: "ping",
          source: "workbench"
        }
      }),
      eventEntry("chrome.api.event", 120, {
        namespace: "runtime",
        event: "onMessage",
        args: [
          {
            kind: "panex.workbench.runtime-probe",
            probe_id: "runtime-ping",
            topic: "ping",
            source: "workbench"
          }
        ]
      })
    ];

    const probe = summarizeRuntimeProbe(timeline);
    assert.match(probe.payloadText, /panex\.workbench\.runtime-probe/);
    assert.match(probe.lastResultText ?? "", /success:/);
    assert.match(probe.lastEventText ?? "", /runtime-probe/);
    assert.equal(probe.lastActivityAtMS, 120);
  });

  it("ignores unrelated chrome api traffic", () => {
    const timeline: TimelineEntry[] = [
      eventEntry("chrome.api.result", 200, {
        call_id: "tabs-query-1",
        success: true,
        data: [{ id: 7 }]
      }),
      eventEntry("chrome.api.event", 220, {
        namespace: "runtime",
        event: "onMessage",
        args: [{ topic: "other" }]
      })
    ];

    const probe = summarizeRuntimeProbe(timeline);
    assert.equal(probe.lastResultText, null);
    assert.equal(probe.lastEventText, null);
    assert.equal(probe.lastActivityAtMS, null);
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

function eventEntry(name: TimelineEntry["envelope"]["name"], recordedAtMS: number, data: unknown): TimelineEntry {
  return {
    key: `live-${name}-${recordedAtMS}`,
    recordedAtMS,
    envelope: {
      v: 1,
      t: "event",
      name,
      src: { role: "daemon", id: "daemon-1" },
      data
    }
  };
}

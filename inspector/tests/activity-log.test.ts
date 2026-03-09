import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { summarizeChromeAPIActivity } from "../src/activity-log";
import type { TimelineEntry } from "../src/timeline";

describe("summarizeChromeAPIActivity", () => {
  it("pairs chrome.api.call entries with matching results and computes latency", () => {
    const activity = summarizeChromeAPIActivity([
      commandEntry("chrome.api.call", 100, {
        call_id: "runtime-send-1",
        namespace: "runtime",
        method: "sendMessage",
        args: [{ kind: "panex.workbench.runtime-probe", probe_id: "runtime-ping" }]
      }),
      eventEntry("chrome.api.result", 125, {
        call_id: "runtime-send-1",
        success: true,
        data: { echoed: true }
      })
    ]);

    assert.equal(activity.length, 1);
    assert.equal(activity[0]?.title, "runtime.sendMessage");
    assert.equal(activity[0]?.status, "success");
    assert.equal(activity[0]?.latencyMS, 25);
    assert.equal(activity[0]?.callID, "runtime-send-1");
    assert.match(activity[0]?.payloadText ?? "", /echoed/);
  });

  it("surfaces unsupported simulator calls distinctly from generic failures", () => {
    const activity = summarizeChromeAPIActivity([
      commandEntry("chrome.api.call", 200, {
        call_id: "bookmarks-1",
        namespace: "bookmarks",
        method: "search",
        args: [{ query: "foo" }]
      }),
      eventEntry("chrome.api.result", 240, {
        call_id: "bookmarks-1",
        success: false,
        error: 'unsupported chrome namespace "bookmarks"'
      }),
      commandEntry("chrome.api.call", 260, {
        call_id: "storage-1",
        namespace: "storage.local",
        method: "clear",
        args: ["nope"]
      }),
      eventEntry("chrome.api.result", 280, {
        call_id: "storage-1",
        success: false,
        error: "clear expects no arguments"
      })
    ]);

    assert.equal(activity[0]?.title, "storage.local.clear");
    assert.equal(activity[0]?.status, "failure");
    assert.equal(activity[1]?.title, "bookmarks.search");
    assert.equal(activity[1]?.status, "unsupported");
  });

  it("includes standalone chrome.api.event entries and unmatched calls", () => {
    const activity = summarizeChromeAPIActivity([
      eventEntry("chrome.api.event", 300, {
        namespace: "runtime",
        event: "onMessage",
        args: [{ kind: "panex.workbench.runtime-probe", probe_id: "runtime-ping" }]
      }),
      commandEntry("chrome.api.call", 280, {
        call_id: "tabs-query-1",
        namespace: "tabs",
        method: "query",
        args: [{ active: true }]
      })
    ]);

    assert.equal(activity[0]?.title, "runtime.onMessage");
    assert.equal(activity[0]?.status, "event");
    assert.equal(activity[1]?.title, "tabs.query");
    assert.equal(activity[1]?.status, "pending");
    assert.match(activity[1]?.payloadText ?? "", /active/);
  });

  it("ignores unrelated timeline traffic", () => {
    const activity = summarizeChromeAPIActivity([
      eventEntry("build.complete", 100, { build_id: "1", success: true }),
      eventEntry("query.events.result", 120, { events: [] })
    ]);

    assert.deepEqual(activity, []);
  });
});

function commandEntry(name: TimelineEntry["envelope"]["name"], recordedAtMS: number, data: unknown): TimelineEntry {
  return {
    key: `live-${name}-${recordedAtMS}`,
    recordedAtMS,
    envelope: {
      v: 1,
      t: "command",
      name,
      src: { role: "inspector", id: "inspector-1" },
      data
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

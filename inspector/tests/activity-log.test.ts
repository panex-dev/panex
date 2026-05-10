import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  BUILD_COMPLETE_MESSAGE_NAME,
  CHROME_API_CALL_MESSAGE_NAME,
  CHROME_API_EVENT_MESSAGE_NAME,
  CHROME_API_RESULT_MESSAGE_NAME,
  QUERY_EVENTS_RESULT_MESSAGE_NAME
} from "@panex/protocol";

import { summarizeChromeAPIActivity } from "../src/activity-log";
import type { TimelineEntry } from "../src/timeline";

describe("summarizeChromeAPIActivity", () => {
  it("pairs chrome.api.call entries with matching results and computes latency", () => {
    const activity = summarizeChromeAPIActivity([
      commandEntry(CHROME_API_CALL_MESSAGE_NAME, 100, {
        call_id: "runtime-send-1",
        namespace: "runtime",
        method: "sendMessage",
        args: [{ kind: "panex.workbench.runtime-probe", probe_id: "runtime-ping" }]
      }),
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 125, {
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
      commandEntry(CHROME_API_CALL_MESSAGE_NAME, 200, {
        call_id: "bookmarks-1",
        namespace: "bookmarks",
        method: "search",
        args: [{ query: "foo" }]
      }),
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 240, {
        call_id: "bookmarks-1",
        success: false,
        error: 'unsupported chrome namespace "bookmarks"'
      }),
      commandEntry(CHROME_API_CALL_MESSAGE_NAME, 260, {
        call_id: "storage-1",
        namespace: "storage.local",
        method: "clear",
        args: ["nope"]
      }),
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 280, {
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
      eventEntry(CHROME_API_EVENT_MESSAGE_NAME, 300, {
        namespace: "runtime",
        event: "onMessage",
        args: [{ kind: "panex.workbench.runtime-probe", probe_id: "runtime-ping" }]
      }),
      commandEntry(CHROME_API_CALL_MESSAGE_NAME, 280, {
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
      eventEntry(BUILD_COMPLETE_MESSAGE_NAME, 100, { build_id: "1", success: true }),
      eventEntry(QUERY_EVENTS_RESULT_MESSAGE_NAME, 120, { events: [] })
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

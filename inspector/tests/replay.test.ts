import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  CHROME_API_EVENT_MESSAGE_NAME,
  CHROME_API_RESULT_MESSAGE_NAME
} from "@panex/protocol";

import { summarizeReplayHistory } from "../src/replay";
import type { TimelineEntry } from "../src/timeline";

describe("summarizeReplayHistory", () => {
  it("returns runtime probe event/result entries in newest-first order", () => {
    const history = summarizeReplayHistory([
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 100, {
        call_id: "runtime-send-1",
        success: true,
        data: runtimeProbePayload("result")
      }),
      eventEntry(CHROME_API_EVENT_MESSAGE_NAME, 120, {
        namespace: "runtime",
        event: "onMessage",
        args: [runtimeProbePayload("event")]
      }),
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 90, {
        call_id: "tabs-query-1",
        success: true,
        data: [{ id: 7 }]
      })
    ]);

    assert.equal(history.length, 2);
    assert.equal(history[0]?.recordedAtMS, 120);
    assert.equal(history[0]?.sourceText, "runtime.onMessage payload");
    assert.equal(history[0]?.callID, null);
    assert.match(history[0]?.payloadText ?? "", /"event"/);
    assert.equal(history[1]?.recordedAtMS, 100);
    assert.equal(history[1]?.sourceText, "runtime result payload");
    assert.equal(history[1]?.callID, "runtime-send-1");
  });

  it("ignores unrelated chrome api traffic", () => {
    const history = summarizeReplayHistory([
      eventEntry(CHROME_API_EVENT_MESSAGE_NAME, 200, {
        namespace: "runtime",
        event: "onMessage",
        args: [{ topic: "other" }]
      }),
      eventEntry(CHROME_API_RESULT_MESSAGE_NAME, 220, {
        call_id: "tabs-query-1",
        success: true,
        data: [{ id: 7 }]
      })
    ]);

    assert.deepEqual(history, []);
  });
});

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

function runtimeProbePayload(source: string) {
  return {
    kind: "panex.workbench.runtime-probe",
    probe_id: "runtime-ping",
    topic: "ping",
    source
  };
}

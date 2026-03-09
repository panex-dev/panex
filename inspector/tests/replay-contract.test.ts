import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  decodeReplayObservation,
  findLatestReplayObservation,
  isReplayPayload,
  replayFamilies
} from "../src/replay-contract";
import type { TimelineEntry } from "../src/timeline";

describe("replay-contract", () => {
  it("publishes a single runtime-probe replay family", () => {
    assert.deepEqual(replayFamilies, [
      {
        id: "runtime-probe",
        label: "runtime probe",
        payloadLabel: "runtime probe payloads"
      }
    ]);
  });

  it("decodes runtime probe results as replay observations", () => {
    const observation = decodeReplayObservation(
      eventEntry("chrome.api.result", 100, {
        call_id: "runtime-send-1",
        success: true,
        data: runtimeProbePayload("result")
      })
    );

    assert.deepEqual(observation, {
      familyID: "runtime-probe",
      familyLabel: "runtime probe",
      observedAs: "result",
      payload: runtimeProbePayload("result"),
      sourceText: "runtime result payload",
      latestSourceText: "latest runtime result payload",
      callID: "runtime-send-1"
    });
  });

  it("decodes runtime.onMessage probe events as replay observations", () => {
    const observation = decodeReplayObservation(
      eventEntry("chrome.api.event", 120, {
        namespace: "runtime",
        event: "onMessage",
        args: [runtimeProbePayload("event")]
      })
    );

    assert.deepEqual(observation, {
      familyID: "runtime-probe",
      familyLabel: "runtime probe",
      observedAs: "event",
      payload: runtimeProbePayload("event"),
      sourceText: "runtime.onMessage payload",
      latestSourceText: "latest runtime.onMessage payload",
      callID: null
    });
  });

  it("rejects unrelated payloads and non-runtime traffic", () => {
    assert.equal(isReplayPayload({ topic: "other" }), false);
    assert.equal(
      decodeReplayObservation(
        eventEntry("chrome.api.result", 200, {
          call_id: "tabs-query-1",
          success: true,
          data: [{ id: 7 }]
        })
      ),
      null
    );
    assert.equal(
      decodeReplayObservation(
        eventEntry("chrome.api.event", 220, {
          namespace: "tabs",
          event: "onUpdated",
          args: [{ id: 7 }]
        })
      ),
      null
    );
  });

  it("selects the newest replay observation from timeline history", () => {
    const observation = findLatestReplayObservation([
      eventEntry("chrome.api.result", 100, {
        call_id: "runtime-send-1",
        success: true,
        data: runtimeProbePayload("result")
      }),
      eventEntry("chrome.api.event", 120, {
        namespace: "runtime",
        event: "onMessage",
        args: [runtimeProbePayload("event")]
      }),
      eventEntry("chrome.api.result", 90, {
        call_id: "tabs-query-1",
        success: true,
        data: [{ id: 7 }]
      })
    ]);

    assert.equal(observation?.observedAs, "event");
    assert.equal(observation?.latestSourceText, "latest runtime.onMessage payload");
    assert.deepEqual(observation?.payload, runtimeProbePayload("event"));
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

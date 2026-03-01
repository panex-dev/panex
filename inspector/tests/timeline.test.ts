import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { formatTime, fromLiveEnvelope, fromSnapshot, mergeEntries, summarizeEnvelope } from "../src/timeline";
import type { Envelope, EventSnapshot } from "../src/protocol";

function envelope(name: Envelope["name"]): Envelope {
  return {
    v: 1,
    t: name === "command.reload" ? "command" : "event",
    name,
    src: { role: "daemon", id: "daemon-1" },
    data:
      name === "build.complete"
        ? { build_id: "build-1", success: true, duration_ms: 12 }
        : { reason: "build.complete", build_id: "build-1" }
  };
}

describe("timeline snapshot conversion", () => {
  it("converts query snapshots into deterministic timeline keys", () => {
    const snapshot: EventSnapshot = {
      id: 9,
      recorded_at_ms: 1234,
      envelope: envelope("build.complete")
    };

    const entry = fromSnapshot(snapshot);
    assert.equal(entry.key, "db-9");
    assert.equal(entry.id, 9);
    assert.equal(entry.recordedAtMS, 1234);
  });
});

describe("timeline merge behavior", () => {
  it("deduplicates incoming snapshots by id", () => {
    const existing = [fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope("build.complete") })];
    const incoming = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope("build.complete") }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope("command.reload") })
    ];

    const merged = mergeEntries(existing, incoming, 10);
    assert.equal(merged.length, 2);
    assert.equal(merged[1].id, 2);
  });

  it("keeps only the newest entries when over limit", () => {
    const one = fromLiveEnvelope(envelope("build.complete"), 1);
    const two = fromLiveEnvelope(envelope("command.reload"), 2);
    const three = fromLiveEnvelope(envelope("build.complete"), 3);

    const merged = mergeEntries([one, two], [three], 2);
    assert.equal(merged.length, 2);
    assert.equal(merged[0].recordedAtMS, 2);
    assert.equal(merged[1].recordedAtMS, 3);
  });
});

describe("timeline summaries", () => {
  it("summarizes known payloads", () => {
    assert.match(summarizeEnvelope(envelope("build.complete")), /build=build-1/);
    assert.match(summarizeEnvelope(envelope("command.reload")), /reason=build.complete/);
  });

  it("formats timestamps as non-empty local time strings", () => {
    const label = formatTime(1700000000000);
    assert.equal(typeof label, "string");
    assert.notEqual(label.length, 0);
  });
});

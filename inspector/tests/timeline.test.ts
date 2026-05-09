import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  BUILD_COMPLETE_MESSAGE_NAME,
  COMMAND_RELOAD_MESSAGE_NAME,
  type Envelope,
  type EventSnapshot
} from "@panex/protocol";

import {
  defaultTimelineRenderWindow,
  filterEntries,
  formatTime,
  fromLiveEnvelope,
  fromSnapshot,
  hiddenOlderTimelineCount,
  mergeEntries,
  mergeEntriesWithOverflow,
  oldestPersistedTimelineID,
  parseSearchQuery,
  renderTimelineWindow,
  summarizeEnvelope
} from "../src/timeline";

function envelope(name: Envelope["name"]): Envelope {
  return {
    v: 1,
    t: name === COMMAND_RELOAD_MESSAGE_NAME ? "command" : "event",
    name,
    src: { role: "daemon", id: "daemon-1" },
    data:
      name === BUILD_COMPLETE_MESSAGE_NAME
        ? { build_id: "build-1", success: true, duration_ms: 12 }
        : { reason: BUILD_COMPLETE_MESSAGE_NAME, build_id: "build-1" }
  };
}

describe("timeline snapshot conversion", () => {
  it("converts query snapshots into deterministic timeline keys", () => {
    const snapshot: EventSnapshot = {
      id: 9,
      recorded_at_ms: 1234,
      envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME)
    };

    const entry = fromSnapshot(snapshot);
    assert.equal(entry.key, "db-9");
    assert.equal(entry.id, 9);
    assert.equal(entry.recordedAtMS, 1234);
  });
});

describe("timeline merge behavior", () => {
  it("deduplicates incoming snapshots by id", () => {
    const existing = [fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) })];
    const incoming = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) })
    ];

    const merged = mergeEntries(existing, incoming, 10);
    assert.equal(merged.length, 2);
    assert.equal(merged[1].id, 2);
  });

  it("keeps only the newest entries when over limit", () => {
    const one = fromLiveEnvelope(envelope(BUILD_COMPLETE_MESSAGE_NAME), 1);
    const two = fromLiveEnvelope(envelope(COMMAND_RELOAD_MESSAGE_NAME), 2);
    const three = fromLiveEnvelope(envelope(BUILD_COMPLETE_MESSAGE_NAME), 3);

    const merged = mergeEntries([one, two], [three], 2);
    assert.equal(merged.length, 2);
    assert.equal(merged[0].recordedAtMS, 2);
    assert.equal(merged[1].recordedAtMS, 3);
  });

  it("prepends older snapshots ahead of existing history", () => {
    const existing = [
      fromSnapshot({ id: 3, recorded_at_ms: 3, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 4, recorded_at_ms: 4, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) })
    ];
    const older = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) })
    ];

    const merged = mergeEntries(existing, older, 10, "prepend");
    assert.deepEqual(
      merged.map((entry) => entry.id),
      [1, 2, 3, 4]
    );
  });

  it("reports when appends trim the oldest retained entries", () => {
    const one = fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) });
    const two = fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) });
    const three = fromSnapshot({ id: 3, recorded_at_ms: 3, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) });

    const merged = mergeEntriesWithOverflow([one, two], [three], 2);
    assert.deepEqual(
      merged.entries.map((entry) => entry.id),
      [2, 3]
    );
    assert.equal(merged.droppedOldest, 1);
    assert.equal(merged.droppedNewest, 0);
  });

  it("reports when older-page prepends trim newer retained entries", () => {
    const existing = [
      fromSnapshot({ id: 3, recorded_at_ms: 3, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 4, recorded_at_ms: 4, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) })
    ];
    const older = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) })
    ];

    const merged = mergeEntriesWithOverflow(existing, older, 3, "prepend");
    assert.deepEqual(
      merged.entries.map((entry) => entry.id),
      [1, 2, 3]
    );
    assert.equal(merged.droppedOldest, 0);
    assert.equal(merged.droppedNewest, 1);
  });
});

describe("timeline render window", () => {
  it("renders only the newest bounded slice of loaded history", () => {
    const entries = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) }),
      fromSnapshot({ id: 3, recorded_at_ms: 3, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) })
    ];

    const rendered = renderTimelineWindow(entries, 2);
    assert.deepEqual(
      rendered.map((entry) => entry.id),
      [2, 3]
    );
  });

  it("falls back to the default window when the requested size is invalid", () => {
    const entries = Array.from({ length: defaultTimelineRenderWindow + 2 }, (_, index) =>
      fromSnapshot({
        id: index + 1,
        recorded_at_ms: index + 1,
        envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME)
      })
    );

    const rendered = renderTimelineWindow(entries, 0);
    assert.equal(rendered.length, defaultTimelineRenderWindow);
    assert.equal(rendered[0].id, 3);
  });

  it("reports how many older loaded entries remain hidden", () => {
    const entries = [
      fromSnapshot({ id: 1, recorded_at_ms: 1, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 2, recorded_at_ms: 2, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) }),
      fromSnapshot({ id: 3, recorded_at_ms: 3, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) })
    ];

    assert.equal(hiddenOlderTimelineCount(entries, 2), 1);
    assert.equal(hiddenOlderTimelineCount(entries, 5), 0);
  });
});

describe("timeline summaries", () => {
  it("summarizes known payloads", () => {
    assert.match(summarizeEnvelope(envelope(BUILD_COMPLETE_MESSAGE_NAME)), /build=build-1/);
    assert.match(summarizeEnvelope(envelope(COMMAND_RELOAD_MESSAGE_NAME)), /reason=build.complete/);
  });

  it("formats timestamps as non-empty local time strings", () => {
    const label = formatTime(1700000000000);
    assert.equal(typeof label, "string");
    assert.notEqual(label.length, 0);
  });
});

describe("timeline filters", () => {
  it("filters by envelope type and source role", () => {
    const first = fromSnapshot({
      id: 1,
      recorded_at_ms: 1,
      envelope: {
        ...envelope(BUILD_COMPLETE_MESSAGE_NAME),
        t: "event",
        src: { role: "daemon", id: "daemon-1" }
      }
    });
    const second = fromSnapshot({
      id: 2,
      recorded_at_ms: 2,
      envelope: {
        ...envelope(COMMAND_RELOAD_MESSAGE_NAME),
        t: "command",
        src: { role: "dev-agent", id: "agent-1" }
      }
    });

    const filtered = filterEntries([first, second], {
      search: "",
      messageType: "command",
      sourceRole: "dev-agent"
    });

    assert.equal(filtered.length, 1);
    assert.equal(filtered[0].id, 2);
  });

  it("searches against envelope metadata and summary text", () => {
    const build = fromSnapshot({
      id: 1,
      recorded_at_ms: 1,
      envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME)
    });
    const reload = fromSnapshot({
      id: 2,
      recorded_at_ms: 2,
      envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME)
    });

    const byName = filterEntries([build, reload], {
      search: COMMAND_RELOAD_MESSAGE_NAME,
      messageType: "all",
      sourceRole: "all"
    });
    assert.equal(byName.length, 1);
    assert.equal(byName[0].id, 2);

    const bySummary = filterEntries([build, reload], {
      search: "build=build-1",
      messageType: "all",
      sourceRole: "all"
    });
    assert.equal(bySummary.length, 2);
  });

  it("supports structured search operators", () => {
    const daemonBuild = fromSnapshot({
      id: 1,
      recorded_at_ms: 1,
      envelope: {
        ...envelope(BUILD_COMPLETE_MESSAGE_NAME),
        src: { role: "daemon", id: "daemon-1" }
      }
    });
    const agentReload = fromSnapshot({
      id: 2,
      recorded_at_ms: 2,
      envelope: {
        ...envelope(COMMAND_RELOAD_MESSAGE_NAME),
        t: "command",
        src: { role: "dev-agent", id: "agent-1" }
      }
    });

    const filtered = filterEntries([daemonBuild, agentReload], {
      search: "name:command.reload src:agent type:command",
      messageType: "all",
      sourceRole: "all"
    });

    assert.equal(filtered.length, 1);
    assert.equal(filtered[0].id, 2);
  });
});

describe("search parser", () => {
  it("parses known operators and preserves plain text tokens", () => {
    const clauses = parseSearchQuery("name:build.complete src:daemon type:event build-1");
    assert.deepEqual(clauses, [
      { key: "name", value: BUILD_COMPLETE_MESSAGE_NAME },
      { key: "src", value: "daemon" },
      { key: "type", value: "event" },
      { key: "text", value: "build-1" }
    ]);
  });

  it("treats unknown prefixes as text clauses", () => {
    const clauses = parseSearchQuery("foo:bar");
    assert.deepEqual(clauses, [{ key: "text", value: "foo:bar" }]);
  });
});

describe("timeline cursor helpers", () => {
  it("finds the oldest persisted id in mixed history", () => {
    const entries = [
      fromSnapshot({ id: 8, recorded_at_ms: 8, envelope: envelope(BUILD_COMPLETE_MESSAGE_NAME) }),
      fromSnapshot({ id: 9, recorded_at_ms: 9, envelope: envelope(COMMAND_RELOAD_MESSAGE_NAME) }),
      fromLiveEnvelope(envelope(BUILD_COMPLETE_MESSAGE_NAME), 10)
    ];

    assert.equal(oldestPersistedTimelineID(entries), 8);
  });

  it("returns null when only live entries exist", () => {
    assert.equal(oldestPersistedTimelineID([fromLiveEnvelope(envelope(BUILD_COMPLETE_MESSAGE_NAME), 10)]), null);
  });
});

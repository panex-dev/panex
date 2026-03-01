import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { handleReloadCommand, isReloadCommand } from "../src/reload";
import type { Envelope } from "../src/protocol";

function baseEnvelope(name: Envelope["name"], type: Envelope["t"]): Envelope {
  return {
    v: 1,
    t: type,
    name,
    src: { role: "daemon", id: "daemon-1" },
    data: {}
  };
}

describe("reload command detection", () => {
  it("identifies command.reload envelopes", () => {
    const envelope = baseEnvelope("command.reload", "command");
    assert.equal(isReloadCommand(envelope), true);
  });

  it("rejects non-command envelopes", () => {
    const envelope = baseEnvelope("build.complete", "event");
    assert.equal(isReloadCommand(envelope), false);
  });
});

describe("reload command handler", () => {
  it("calls runtime reload for command.reload", () => {
    const envelope = baseEnvelope("command.reload", "command");

    let called = 0;
    const handled = handleReloadCommand(envelope, () => {
      called += 1;
    });

    assert.equal(handled, true);
    assert.equal(called, 1);
  });

  it("does not call runtime reload for unrelated messages", () => {
    const envelope = baseEnvelope("context.log", "event");

    let called = 0;
    const handled = handleReloadCommand(envelope, () => {
      called += 1;
    });

    assert.equal(handled, false);
    assert.equal(called, 0);
  });
});

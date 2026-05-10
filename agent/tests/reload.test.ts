import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  BUILD_COMPLETE_MESSAGE_NAME,
  COMMAND_RELOAD_MESSAGE_NAME,
  type Envelope
} from "@panex/protocol";

import { handleReloadCommand, isReloadCommand } from "../src/reload";

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
    const envelope = baseEnvelope(COMMAND_RELOAD_MESSAGE_NAME, "command");
    assert.equal(isReloadCommand(envelope), true);
  });

  it("rejects non-command envelopes", () => {
    const envelope = baseEnvelope(BUILD_COMPLETE_MESSAGE_NAME, "event");
    assert.equal(isReloadCommand(envelope), false);
  });
});

describe("reload command handler", () => {
  it("calls runtime reload for command.reload", () => {
    const envelope = {
      ...baseEnvelope(COMMAND_RELOAD_MESSAGE_NAME, "command"),
      data: { extension_id: "default" }
    };

    let called = 0;
    const handled = handleReloadCommand(envelope, "default", () => {
      called += 1;
    });

    assert.equal(handled, true);
    assert.equal(called, 1);
  });

  it("does not call runtime reload for unrelated messages", () => {
    const envelope = baseEnvelope(BUILD_COMPLETE_MESSAGE_NAME, "event");

    let called = 0;
    const handled = handleReloadCommand(envelope, "default", () => {
      called += 1;
    });

    assert.equal(handled, false);
    assert.equal(called, 0);
  });

  it("ignores reloads targeted at a different extension id", () => {
    const envelope = {
      ...baseEnvelope(COMMAND_RELOAD_MESSAGE_NAME, "command"),
      data: { extension_id: "other" }
    };

    let called = 0;
    const handled = handleReloadCommand(envelope, "default", () => {
      called += 1;
    });

    assert.equal(handled, false);
    assert.equal(called, 0);
  });
});

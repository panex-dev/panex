import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  envelopeNames,
  isEnvelope,
  isHelloAck,
  isQueryEventsResult,
  isQueryStorageResult,
  isReloadCommand,
  messageTypeByName,
  messageTypeForName,
  type Envelope,
  type EnvelopeName
} from "../src/index";

function makeEnvelope(name: EnvelopeName): Envelope {
  return {
    v: 1,
    t: messageTypeForName(name),
    name,
    src: { role: "daemon", id: "daemon-1" },
    data: {}
  };
}

describe("isEnvelope", () => {
  it("keeps message type mapping exhaustive against known names", () => {
    const mappedNames = Object.keys(messageTypeByName).sort();
    const knownNames = [...envelopeNames].sort();
    assert.deepEqual(mappedNames, knownNames);
  });

  it("accepts known envelope shapes", () => {
    const value = makeEnvelope("command.reload");
    assert.equal(isEnvelope(value), true);
  });

  it("rejects unknown names", () => {
    const value = {
      ...makeEnvelope("command.reload"),
      name: "unknown.message"
    };

    assert.equal(isEnvelope(value), false);
  });

  it("rejects mismatched type/name pairs", () => {
    const value = {
      ...makeEnvelope("command.reload"),
      t: "event"
    };

    assert.equal(isEnvelope(value), false);
  });

  it("rejects missing source identifiers", () => {
    const value = {
      ...makeEnvelope("build.complete"),
      src: { role: "daemon", id: " " }
    };

    assert.equal(isEnvelope(value), false);
  });
});

describe("message guards", () => {
  it("identifies hello.ack envelopes", () => {
    assert.equal(isHelloAck(makeEnvelope("hello.ack")), true);
    assert.equal(isHelloAck(makeEnvelope("hello")), false);
  });

  it("identifies query.events.result envelopes", () => {
    assert.equal(isQueryEventsResult(makeEnvelope("query.events.result")), true);
    assert.equal(isQueryEventsResult(makeEnvelope("build.complete")), false);
  });

  it("identifies command.reload envelopes", () => {
    assert.equal(isReloadCommand(makeEnvelope("command.reload")), true);
    assert.equal(isReloadCommand(makeEnvelope("context.log")), false);
  });

  it("identifies query.storage.result envelopes", () => {
    assert.equal(isQueryStorageResult(makeEnvelope("query.storage.result")), true);
    assert.equal(isQueryStorageResult(makeEnvelope("query.events.result")), false);
  });
});

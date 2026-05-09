import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  CHROME_SIM_CLIENT_KIND,
  DEFAULT_FIRST_PARTY_CLIENT_VERSION,
  DEV_AGENT_CLIENT_KIND,
  INSPECTOR_CLIENT_KIND,
  envelopeNames,
  firstPartyRequestedCapabilities,
  firstPartyClientKinds,
  firstPartySourceRolesByClientKind,
  isEnvelope,
  isHelloAck,
  isQueryEventsResult,
  isQueryStorageResult,
  isReloadCommand,
  messageTypeByName,
  messageTypeForName,
  negotiableCapabilityNames,
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
    assert.equal(isReloadCommand(makeEnvelope("build.complete")), false);
  });

  it("identifies query.storage.result envelopes", () => {
    assert.equal(isQueryStorageResult(makeEnvelope("query.storage.result")), true);
    assert.equal(isQueryStorageResult(makeEnvelope("query.events.result")), false);
  });

  it("maps storage mutation messages as commands", () => {
    assert.equal(messageTypeForName("storage.set"), "command");
    assert.equal(messageTypeForName("storage.remove"), "command");
    assert.equal(messageTypeForName("storage.clear"), "command");
  });

  it("maps chrome api simulator transport messages", () => {
    assert.equal(messageTypeForName("chrome.api.call"), "command");
    assert.equal(messageTypeForName("chrome.api.result"), "event");
    assert.equal(messageTypeForName("chrome.api.event"), "event");
  });
});

describe("capability contracts", () => {
  it("publishes the shared first-party client version contract", () => {
    assert.equal(DEFAULT_FIRST_PARTY_CLIENT_VERSION, "dev");
  });

  it("publishes named first-party client kind constants", () => {
    assert.equal(DEV_AGENT_CLIENT_KIND, "dev-agent");
    assert.equal(INSPECTOR_CLIENT_KIND, "inspector");
    assert.equal(CHROME_SIM_CLIENT_KIND, "chrome-sim");
    assert.deepEqual(firstPartyClientKinds, [
      DEV_AGENT_CLIENT_KIND,
      INSPECTOR_CLIENT_KIND,
      CHROME_SIM_CLIENT_KIND
    ]);
  });

  it("keeps response-only message names out of negotiable capabilities", () => {
    const capabilities = negotiableCapabilityNames as readonly string[];
    assert.equal(capabilities.includes("chrome.api.result"), false);
    assert.equal(capabilities.includes("query.events.result"), false);
    assert.equal(capabilities.includes("query.storage.result"), false);
  });

  it("publishes the scoped first-party request sets", () => {
    assert.deepEqual(firstPartyRequestedCapabilities["dev-agent"], ["command.reload"]);
    assert.deepEqual(firstPartyRequestedCapabilities["chrome-sim"], [
      "chrome.api.call",
      "chrome.api.event",
      "storage.diff"
    ]);
    assert.deepEqual(firstPartyRequestedCapabilities.inspector, [
      "query.events",
      "build.complete",
      "command.reload",
      "query.storage",
      "storage.diff",
      "storage.set",
      "storage.remove",
      "storage.clear",
      "chrome.api.call",
      "chrome.api.event"
    ]);
  });

  it("publishes the first-party source-role contract", () => {
    assert.deepEqual(firstPartySourceRolesByClientKind, {
      "dev-agent": "dev-agent",
      inspector: "inspector",
      "chrome-sim": "chrome-sim"
    });
  });
});

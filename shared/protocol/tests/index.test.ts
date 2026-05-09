import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  CHROME_API_CALL_MESSAGE_NAME,
  CHROME_SIM_SOURCE_ROLE,
  CHROME_SIM_CLIENT_KIND,
  CHROME_API_EVENT_MESSAGE_NAME,
  CHROME_API_RESULT_MESSAGE_NAME,
  DAEMON_SOURCE_ROLE,
  DEFAULT_FIRST_PARTY_CLIENT_VERSION,
  DEV_AGENT_SOURCE_ROLE,
  DEV_AGENT_CLIENT_KIND,
  HELLO_ACK_MESSAGE_NAME,
  HELLO_MESSAGE_NAME,
  INSPECTOR_SOURCE_ROLE,
  INSPECTOR_CLIENT_KIND,
  QUERY_EVENTS_MESSAGE_NAME,
  QUERY_EVENTS_RESULT_MESSAGE_NAME,
  QUERY_STORAGE_MESSAGE_NAME,
  QUERY_STORAGE_RESULT_MESSAGE_NAME,
  STORAGE_CLEAR_MESSAGE_NAME,
  STORAGE_DIFF_MESSAGE_NAME,
  STORAGE_REMOVE_MESSAGE_NAME,
  STORAGE_SET_MESSAGE_NAME,
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
    src: { role: DAEMON_SOURCE_ROLE, id: "daemon-1" },
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
      src: { role: DAEMON_SOURCE_ROLE, id: " " }
    };

    assert.equal(isEnvelope(value), false);
  });
});

describe("message guards", () => {
  it("identifies hello.ack envelopes", () => {
    assert.equal(isHelloAck(makeEnvelope(HELLO_ACK_MESSAGE_NAME)), true);
    assert.equal(isHelloAck(makeEnvelope(HELLO_MESSAGE_NAME)), false);
  });

  it("identifies query.events.result envelopes", () => {
    assert.equal(isQueryEventsResult(makeEnvelope(QUERY_EVENTS_RESULT_MESSAGE_NAME)), true);
    assert.equal(isQueryEventsResult(makeEnvelope("build.complete")), false);
  });

  it("identifies command.reload envelopes", () => {
    assert.equal(isReloadCommand(makeEnvelope("command.reload")), true);
    assert.equal(isReloadCommand(makeEnvelope("build.complete")), false);
  });

  it("identifies query.storage.result envelopes", () => {
    assert.equal(isQueryStorageResult(makeEnvelope(QUERY_STORAGE_RESULT_MESSAGE_NAME)), true);
    assert.equal(isQueryStorageResult(makeEnvelope(QUERY_EVENTS_RESULT_MESSAGE_NAME)), false);
  });

  it("maps storage mutation messages as commands", () => {
    assert.equal(messageTypeForName(STORAGE_SET_MESSAGE_NAME), "command");
    assert.equal(messageTypeForName(STORAGE_REMOVE_MESSAGE_NAME), "command");
    assert.equal(messageTypeForName(STORAGE_CLEAR_MESSAGE_NAME), "command");
  });

  it("maps chrome api simulator transport messages", () => {
    assert.equal(messageTypeForName(CHROME_API_CALL_MESSAGE_NAME), "command");
    assert.equal(messageTypeForName(CHROME_API_RESULT_MESSAGE_NAME), "event");
    assert.equal(messageTypeForName(CHROME_API_EVENT_MESSAGE_NAME), "event");
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

  it("publishes named source role constants", () => {
    assert.equal(DAEMON_SOURCE_ROLE, "daemon");
    assert.equal(DEV_AGENT_SOURCE_ROLE, "dev-agent");
    assert.equal(CHROME_SIM_SOURCE_ROLE, "chrome-sim");
    assert.equal(INSPECTOR_SOURCE_ROLE, "inspector");
  });

  it("publishes named handshake lifecycle message constants", () => {
    assert.equal(HELLO_MESSAGE_NAME, "hello");
    assert.equal(HELLO_ACK_MESSAGE_NAME, "hello.ack");
  });

  it("publishes named chrome api transport message constants", () => {
    assert.equal(CHROME_API_CALL_MESSAGE_NAME, "chrome.api.call");
    assert.equal(CHROME_API_RESULT_MESSAGE_NAME, "chrome.api.result");
    assert.equal(CHROME_API_EVENT_MESSAGE_NAME, "chrome.api.event");
  });

  it("publishes named query and storage message constants", () => {
    assert.equal(QUERY_EVENTS_MESSAGE_NAME, "query.events");
    assert.equal(QUERY_EVENTS_RESULT_MESSAGE_NAME, "query.events.result");
    assert.equal(QUERY_STORAGE_MESSAGE_NAME, "query.storage");
    assert.equal(QUERY_STORAGE_RESULT_MESSAGE_NAME, "query.storage.result");
    assert.equal(STORAGE_DIFF_MESSAGE_NAME, "storage.diff");
    assert.equal(STORAGE_SET_MESSAGE_NAME, "storage.set");
    assert.equal(STORAGE_REMOVE_MESSAGE_NAME, "storage.remove");
    assert.equal(STORAGE_CLEAR_MESSAGE_NAME, "storage.clear");
  });

  it("keeps response-only message names out of negotiable capabilities", () => {
    const capabilities = negotiableCapabilityNames as readonly string[];
    assert.equal(capabilities.includes(CHROME_API_RESULT_MESSAGE_NAME), false);
    assert.equal(capabilities.includes(QUERY_EVENTS_RESULT_MESSAGE_NAME), false);
    assert.equal(capabilities.includes(QUERY_STORAGE_RESULT_MESSAGE_NAME), false);
  });

  it("publishes the scoped first-party request sets", () => {
    assert.deepEqual(firstPartyRequestedCapabilities["dev-agent"], ["command.reload"]);
    assert.deepEqual(firstPartyRequestedCapabilities["chrome-sim"], [
      CHROME_API_CALL_MESSAGE_NAME,
      CHROME_API_EVENT_MESSAGE_NAME,
      STORAGE_DIFF_MESSAGE_NAME
    ]);
    assert.deepEqual(firstPartyRequestedCapabilities.inspector, [
      QUERY_EVENTS_MESSAGE_NAME,
      "build.complete",
      "command.reload",
      QUERY_STORAGE_MESSAGE_NAME,
      STORAGE_DIFF_MESSAGE_NAME,
      STORAGE_SET_MESSAGE_NAME,
      STORAGE_REMOVE_MESSAGE_NAME,
      STORAGE_CLEAR_MESSAGE_NAME,
      CHROME_API_CALL_MESSAGE_NAME,
      CHROME_API_EVENT_MESSAGE_NAME
    ]);
  });

  it("publishes the first-party source-role contract", () => {
    assert.deepEqual(firstPartySourceRolesByClientKind, {
      "dev-agent": DEV_AGENT_SOURCE_ROLE,
      inspector: INSPECTOR_SOURCE_ROLE,
      "chrome-sim": CHROME_SIM_SOURCE_ROLE
    });
  });
});

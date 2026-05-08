import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  bridgeSessionSupportsCapability,
  bridgeSessionFromHelloAck,
  buildPostHelloAckMessages,
  buildTimelineQuery,
  inspectorRequestedCapabilities,
  resolveConnectionParamsFromSearch
} from "../src/connection";

describe("resolveConnectionParamsFromSearch", () => {
  it("uses loopback defaults when no overrides are present", () => {
    assert.deepEqual(resolveConnectionParamsFromSearch(""), {
      wsURL: "ws://127.0.0.1:4317/ws",
      token: ""
    });
  });

  it("accepts top-level loopback websocket overrides", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws%3A%2F%2Flocalhost%3A9999%2Fws%3Fclient%3Dinspector%26token%3Dleak&token=secret"
      ),
      {
        wsURL: "ws://localhost:9999/ws?client=inspector",
        token: "secret"
      }
    );
  });

  it("falls back when the websocket override leaves the daemon contract", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws%3A%2F%2Fevil.example%3A4317%2Fnot-ws%3Ftoken%3Dleak&token=secret"
      ),
      {
        wsURL: "ws://127.0.0.1:4317/ws",
        token: "secret"
      }
    );
  });

  it("ignores query-param overrides when the inspector is embedded", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws://localhost:9999/ws&token=secret",
        true
      ),
      {
        wsURL: "ws://127.0.0.1:4317/ws",
        token: ""
      }
    );
  });
});

describe("buildTimelineQuery", () => {
  it("builds the initial recent-history request without a cursor", () => {
    const query = buildTimelineQuery(500);
    assert.deepEqual(query, {
      v: 1,
      t: "command",
      name: "query.events",
      src: {
        role: "inspector",
        id: query.src.id
      },
      data: { limit: 500 }
    });
  });

  it("adds before_id when loading older history", () => {
    const query = buildTimelineQuery(250, 42);
    assert.equal(query.data.limit, 250);
    assert.equal(query.data.before_id, 42);
  });
});

describe("bridgeSessionFromHelloAck", () => {
  it("normalizes runtime metadata from hello.ack", () => {
    assert.deepEqual(
      bridgeSessionFromHelloAck({
        protocol_version: 1,
        daemon_version: "dev",
        session_id: "session-1",
        auth_ok: true,
        extension_id: "popup",
        capabilities_supported: ["query.events", "storage.diff"]
      }),
      {
        daemonVersion: "dev",
        sessionID: "session-1",
        extensionID: "popup",
        capabilitiesSupported: ["query.events", "storage.diff"]
      }
    );
  });

  it("drops blank extension ids", () => {
    assert.equal(
      bridgeSessionFromHelloAck({
        protocol_version: 1,
        daemon_version: "dev",
        session_id: "session-1",
        auth_ok: true,
        extension_id: " ",
        capabilities_supported: []
      }).extensionID,
      null
    );
  });
});

describe("bridgeSessionSupportsCapability", () => {
  it("matches negotiated capabilities exactly", () => {
    const session = bridgeSessionFromHelloAck({
      protocol_version: 1,
      daemon_version: "dev",
      session_id: "session-1",
      auth_ok: true,
      capabilities_supported: ["query.events", "chrome.api.call"]
    });

    assert.equal(bridgeSessionSupportsCapability(session, "query.events"), true);
    assert.equal(bridgeSessionSupportsCapability(session, "query.storage"), false);
    assert.equal(bridgeSessionSupportsCapability(null, "query.events"), false);
  });
});

describe("buildPostHelloAckMessages", () => {
  it("queues only the follow-up queries negotiated during hello.ack", () => {
    const session = bridgeSessionFromHelloAck({
      protocol_version: 1,
      daemon_version: "dev",
      session_id: "session-1",
      auth_ok: true,
      capabilities_supported: ["query.storage"]
    });

    const messages = buildPostHelloAckMessages(session);
    assert.equal(messages.length, 1);
    assert.equal(messages[0]?.name, "query.storage");
    assert.equal(messages[0]?.t, "command");
    assert.equal(messages[0]?.src.role, "inspector");
    assert.deepEqual(messages[0]?.data, {});
  });

  it("queues both timeline and storage follow-ups when both capabilities are negotiated", () => {
    const session = bridgeSessionFromHelloAck({
      protocol_version: 1,
      daemon_version: "dev",
      session_id: "session-1",
      auth_ok: true,
      capabilities_supported: ["query.events", "query.storage"]
    });

    const messages = buildPostHelloAckMessages(session);
    assert.equal(messages[0]?.name, "query.events");
    assert.equal(messages[0]?.t, "command");
    assert.deepEqual(messages[0]?.data, { limit: 500 });
    assert.equal(messages[1]?.name, "query.storage");
    assert.equal(messages[1]?.t, "command");
    assert.deepEqual(messages[1]?.data, {});
  });
});

describe("inspectorRequestedCapabilities", () => {
  it("includes the live and command capabilities the inspector actually negotiates", () => {
    assert.equal(inspectorRequestedCapabilities.includes("build.complete"), true);
    assert.equal(inspectorRequestedCapabilities.includes("command.reload"), true);
    assert.equal(inspectorRequestedCapabilities.includes("storage.diff"), true);
    assert.equal(inspectorRequestedCapabilities.includes("chrome.api.call"), true);
    assert.equal(inspectorRequestedCapabilities.includes("chrome.api.event"), true);
  });

  it("omits response-only capability names from the handshake request set", () => {
    assert.equal(
      (inspectorRequestedCapabilities as readonly string[]).includes("chrome.api.result"),
      false,
    );
  });
});

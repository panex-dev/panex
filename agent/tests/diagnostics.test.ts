import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  COMMAND_RELOAD_MESSAGE_NAME,
  type Envelope,
  type HelloAck
} from "@panex/protocol";

import {
  createAgentDiagnostics,
  summarizeEnvelope,
  summarizeHelloAck,
  type AgentDiagnosticEntry
} from "../src/diagnostics";

describe("createAgentDiagnostics", () => {
  it("stays silent when diagnostic logging is disabled", () => {
    const entries: AgentDiagnosticEntry[] = [];
    const diagnostics = createAgentDiagnostics(false, (entry) => {
      entries.push(entry);
    });

    diagnostics.log("websocket.open", { url: "ws://127.0.0.1:4317/ws" });

    assert.deepEqual(entries, []);
  });

  it("emits structured entries when diagnostic logging is enabled", () => {
    const entries: AgentDiagnosticEntry[] = [];
    const diagnostics = createAgentDiagnostics(true, (entry) => {
      entries.push(entry);
    });

    diagnostics.log(COMMAND_RELOAD_MESSAGE_NAME, { sourceId: "daemon-1", reason: "build complete" });

    assert.deepEqual(entries, [
      {
        event: COMMAND_RELOAD_MESSAGE_NAME,
        detail: {
          sourceId: "daemon-1",
          reason: "build complete"
        }
      }
    ]);
  });
});

describe("diagnostic summaries", () => {
  it("summarizes envelope metadata without including payload data", () => {
    const envelope: Envelope = {
      v: 1,
      t: "command",
      name: COMMAND_RELOAD_MESSAGE_NAME,
      src: { role: "daemon", id: "daemon-1" },
      data: { reason: "build complete", auth_token: "should-not-log" }
    };

    assert.deepEqual(summarizeEnvelope(envelope), {
      name: COMMAND_RELOAD_MESSAGE_NAME,
      type: "command",
      sourceRole: "daemon",
      sourceId: "daemon-1"
    });
  });

  it("summarizes hello acknowledgements with negotiated state only", () => {
    const ack: HelloAck = {
      protocol_version: 1,
      daemon_version: "dev",
      session_id: "session-1",
      auth_ok: true,
      extension_id: "popup",
      capabilities_supported: [COMMAND_RELOAD_MESSAGE_NAME]
    };

    assert.deepEqual(summarizeHelloAck(ack), {
      authOK: true,
      sessionID: "session-1",
      extensionID: "popup",
      capabilitiesSupported: [COMMAND_RELOAD_MESSAGE_NAME]
    });
  });
});

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  BUILD_COMPLETE_MESSAGE_NAME,
  COMMAND_RELOAD_MESSAGE_NAME,
  DEFAULT_FIRST_PARTY_CLIENT_VERSION,
  DEV_AGENT_CLIENT_KIND,
  HELLO_ACK_MESSAGE_NAME,
  HELLO_MESSAGE_NAME,
  type Envelope,
  type HelloAck
} from "@panex/protocol";

import type { AgentConfig } from "../src/config";
import {
  buildHelloEnvelope,
  createAgentHandshakeState,
  handleDaemonEnvelope,
  resetAgentHandshakeState,
  requestedCapabilities
} from "../src/handshake";

const config: AgentConfig = {
  wsUrl: "ws://127.0.0.1:4317/ws",
  token: "dev-token",
  agentId: "agent-1",
  extensionId: "default",
  diagnosticLogging: false
};

describe("buildHelloEnvelope", () => {
  it("requests command.reload capability from the daemon", () => {
    const envelope = buildHelloEnvelope(config);

    assert.equal(envelope.name, HELLO_MESSAGE_NAME);
    assert.equal(envelope.t, "lifecycle");
    assert.equal(envelope.src.role, "dev-agent");
    assert.equal(envelope.src.id, "agent-1");
    assert.equal(envelope.data.client_kind, DEV_AGENT_CLIENT_KIND);
    assert.equal(envelope.data.extension_id, "default");
    assert.equal(envelope.data.client_version, DEFAULT_FIRST_PARTY_CLIENT_VERSION);
    assert.deepEqual(envelope.data.capabilities_requested, [...requestedCapabilities]);
  });
});

describe("handleDaemonEnvelope", () => {
  it("closes when a live command arrives before hello.ack", () => {
    const state = createAgentHandshakeState();
    const closed: Array<{ code?: number; reason?: string }> = [];
    let reloaded = 0;

    const result = handleDaemonEnvelope(reloadEnvelope(), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: (code?: number, reason?: string) => {
        closed.push({ code, reason });
      }
    });

    assert.equal(result, "closed");
    assert.equal(reloaded, 0);
    assert.deepEqual(closed, [{ code: 1002, reason: "expected hello.ack before live messages" }]);
    assert.equal(state.complete, false);
  });

  it("marks the session ready after a successful hello.ack", () => {
    const state = createAgentHandshakeState();
    resetAgentHandshakeState(state, "popup");

    const result = handleDaemonEnvelope(helloAckEnvelope({ auth_ok: true }), state, {
      runtimeReload: () => {
        throw new Error("unexpected reload");
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    assert.equal(result, "hello_ack");
    assert.equal(state.complete, true);
    assert.equal(state.extensionID, "popup");
    assert.equal(state.capabilitiesSupported.has(COMMAND_RELOAD_MESSAGE_NAME), true);
  });

  it("closes when the daemon reports a mismatched protocol version", () => {
    const state = createAgentHandshakeState();
    const closed: Array<{ code?: number; reason?: string }> = [];

    const result = handleDaemonEnvelope(helloAckEnvelope({ protocol_version: 99 }), state, {
      runtimeReload: () => {
        throw new Error("unexpected reload");
      },
      closeSocket: (code?: number, reason?: string) => {
        closed.push({ code, reason });
      }
    });

    assert.equal(result, "closed");
    assert.equal(state.complete, false);
    assert.equal(closed.length, 1);
    assert.equal(closed[0].code, 1002);
    assert.match(closed[0].reason!, /protocol version mismatch/);
  });

  it("closes when the daemon rejects the handshake", () => {
    const state = createAgentHandshakeState();
    const closed: Array<{ code?: number; reason?: string }> = [];

    const result = handleDaemonEnvelope(helloAckEnvelope({ auth_ok: false }), state, {
      runtimeReload: () => {
        throw new Error("unexpected reload");
      },
      closeSocket: (code?: number, reason?: string) => {
        closed.push({ code, reason });
      }
    });

    assert.equal(result, "closed");
    assert.equal(state.complete, false);
    assert.deepEqual(closed, [{ code: 1008, reason: "daemon rejected handshake" }]);
  });

  it("closes when the daemon does not negotiate command.reload", () => {
    const state = createAgentHandshakeState();
    const closed: Array<{ code?: number; reason?: string }> = [];

    const result = handleDaemonEnvelope(
      helloAckEnvelope({ capabilities_supported: ["query.events"] }),
      state,
      {
        runtimeReload: () => {
          throw new Error("unexpected reload");
        },
        closeSocket: (code?: number, reason?: string) => {
          closed.push({ code, reason });
        }
      }
    );

    assert.equal(result, "closed");
    assert.equal(state.complete, false);
    assert.deepEqual(closed, [{ code: 1002, reason: "daemon did not negotiate command.reload" }]);
  });

  it("reloads only after a successful hello.ack", () => {
    const state = createAgentHandshakeState();
    resetAgentHandshakeState(state, "popup");
    let reloaded = 0;

    handleDaemonEnvelope(helloAckEnvelope({ auth_ok: true }), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    const result = handleDaemonEnvelope(reloadEnvelope("popup"), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    assert.equal(result, "reload");
    assert.equal(reloaded, 1);
  });

  it("ignores reloads targeted at a different extension id", () => {
    const state = createAgentHandshakeState();
    resetAgentHandshakeState(state, "popup");
    let reloaded = 0;

    handleDaemonEnvelope(helloAckEnvelope({ auth_ok: true }), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    const result = handleDaemonEnvelope(reloadEnvelope("admin"), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    assert.equal(result, "ignored");
    assert.equal(reloaded, 0);
  });

  it("ignores unrelated envelopes after the handshake completes", () => {
    const state = createAgentHandshakeState();
    let reloaded = 0;

    handleDaemonEnvelope(helloAckEnvelope({ auth_ok: true }), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    const result = handleDaemonEnvelope(buildCompleteEnvelope(), state, {
      runtimeReload: () => {
        reloaded += 1;
      },
      closeSocket: () => {
        throw new Error("unexpected close");
      }
    });

    assert.equal(result, "ignored");
    assert.equal(reloaded, 0);
  });
});

function helloAckEnvelope(
  overrides: Partial<HelloAck> = {}
): Envelope<HelloAck> {
  return {
    v: 1,
    t: "lifecycle",
    name: HELLO_ACK_MESSAGE_NAME,
    src: { role: "daemon", id: "daemon-1" },
    data: {
      protocol_version: 1,
      daemon_version: "dev",
      session_id: "session-1",
      auth_ok: true,
      capabilities_supported: [COMMAND_RELOAD_MESSAGE_NAME],
      ...overrides
    }
  };
}

function reloadEnvelope(extensionID?: string): Envelope {
  return {
    v: 1,
    t: "command",
    name: COMMAND_RELOAD_MESSAGE_NAME,
    src: { role: "daemon", id: "daemon-1" },
    data: extensionID ? { reason: "build complete", extension_id: extensionID } : { reason: "build complete" }
  };
}

function buildCompleteEnvelope(): Envelope {
  return {
    v: 1,
    t: "event",
    name: BUILD_COMPLETE_MESSAGE_NAME,
    src: { role: "daemon", id: "daemon-1" },
    data: { build_id: "b1", success: true, duration_ms: 10 }
  };
}

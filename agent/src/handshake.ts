import {
  PROTOCOL_VERSION,
  isHelloAck,
  type Envelope,
  type Hello
} from "@panex/protocol";

import type { AgentConfig } from "./config";
import { handleReloadCommand } from "./reload";

const closeProtocolError = 1002;
const closePolicyViolation = 1008;

export const requestedCapabilities = ["command.reload"] as const;

export interface AgentHandshakeState {
  complete: boolean;
  capabilitiesSupported: Set<string>;
}

export interface AgentHandshakeHooks {
  runtimeReload: () => void;
  closeSocket: (code?: number, reason?: string) => void;
}

export type AgentEnvelopeResult = "hello_ack" | "reload" | "ignored" | "closed";

export function createAgentHandshakeState(): AgentHandshakeState {
  return {
    complete: false,
    capabilitiesSupported: new Set<string>()
  };
}

export function resetAgentHandshakeState(state: AgentHandshakeState): void {
  state.complete = false;
  state.capabilitiesSupported.clear();
}

export function buildHelloEnvelope(config: AgentConfig): Envelope<Hello> {
  return {
    v: PROTOCOL_VERSION,
    t: "lifecycle",
    name: "hello",
    src: { role: "dev-agent", id: config.agentId },
    data: {
      protocol_version: PROTOCOL_VERSION,
      auth_token: config.token,
      client_kind: "dev-agent",
      client_version: "dev",
      capabilities_requested: [...requestedCapabilities]
    }
  };
}

export function handleDaemonEnvelope(
  envelope: Envelope,
  state: AgentHandshakeState,
  hooks: AgentHandshakeHooks
): AgentEnvelopeResult {
  if (!state.complete) {
    if (!isHelloAck(envelope)) {
      hooks.closeSocket(closeProtocolError, "expected hello.ack before live messages");
      return "closed";
    }

    if (!envelope.data.auth_ok) {
      resetAgentHandshakeState(state);
      hooks.closeSocket(closePolicyViolation, "daemon rejected handshake");
      return "closed";
    }

    const capabilitiesSupported = Array.isArray(envelope.data.capabilities_supported)
      ? envelope.data.capabilities_supported.filter((value): value is string => typeof value === "string")
      : [];

    state.complete = true;
    state.capabilitiesSupported = new Set(capabilitiesSupported);

    if (!state.capabilitiesSupported.has("command.reload")) {
      resetAgentHandshakeState(state);
      hooks.closeSocket(closeProtocolError, "daemon did not negotiate command.reload");
      return "closed";
    }

    return "hello_ack";
  }

  if (handleReloadCommand(envelope, hooks.runtimeReload)) {
    return "reload";
  }

  return "ignored";
}

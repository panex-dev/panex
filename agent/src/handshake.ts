import {
  DEFAULT_FIRST_PARTY_CLIENT_VERSION,
  firstPartyRequestedCapabilities,
  firstPartySourceRolesByClientKind,
  PROTOCOL_VERSION,
  isHelloAck,
  type Envelope,
  type Hello
} from "@panex/protocol";

import type { AgentConfig } from "./config";
import { handleReloadCommand } from "./reload";

const closeProtocolError = 1002;
const closePolicyViolation = 1008;
const agentClientKind = "dev-agent";
const agentSourceRole = firstPartySourceRolesByClientKind[agentClientKind];

export const requestedCapabilities = firstPartyRequestedCapabilities[agentClientKind];

export interface AgentHandshakeState {
  complete: boolean;
  capabilitiesSupported: Set<string>;
  extensionID: string;
}

export interface AgentHandshakeHooks {
  runtimeReload: () => void;
  closeSocket: (code?: number, reason?: string) => void;
}

export type AgentEnvelopeResult = "hello_ack" | "reload" | "ignored" | "closed";

export function createAgentHandshakeState(): AgentHandshakeState {
  return {
    complete: false,
    capabilitiesSupported: new Set<string>(),
    extensionID: "default"
  };
}

export function resetAgentHandshakeState(state: AgentHandshakeState, extensionID = state.extensionID): void {
  state.complete = false;
  state.capabilitiesSupported.clear();
  state.extensionID = extensionID;
}

export function buildHelloEnvelope(config: AgentConfig): Envelope<Hello> {
  return {
    v: PROTOCOL_VERSION,
    t: "lifecycle",
    name: "hello",
    src: { role: agentSourceRole, id: config.agentId },
    data: {
      protocol_version: PROTOCOL_VERSION,
      auth_token: config.token,
      client_kind: agentClientKind,
      client_version: DEFAULT_FIRST_PARTY_CLIENT_VERSION,
      extension_id: config.extensionId,
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

    if (envelope.data.protocol_version !== PROTOCOL_VERSION) {
      resetAgentHandshakeState(state);
      hooks.closeSocket(closeProtocolError, "protocol version mismatch");
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

  if (handleReloadCommand(envelope, state.extensionID, hooks.runtimeReload)) {
    return "reload";
  }

  return "ignored";
}

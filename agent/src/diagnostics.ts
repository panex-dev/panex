import type { Envelope, HelloAck } from "@panex/protocol";

export interface AgentDiagnosticEntry {
  event: string;
  detail: Record<string, unknown>;
}

export type AgentDiagnosticSink = (entry: AgentDiagnosticEntry) => void;

export interface AgentDiagnostics {
  readonly enabled: boolean;
  log: (event: string, detail?: Record<string, unknown>) => void;
}

export function createAgentDiagnostics(
  enabled: boolean,
  sink: AgentDiagnosticSink = defaultAgentDiagnosticSink
): AgentDiagnostics {
  return {
    enabled,
    log(event, detail = {}) {
      if (!enabled) {
        return;
      }

      sink({ event, detail });
    }
  };
}

export function summarizeEnvelope(envelope: Envelope): Record<string, unknown> {
  return {
    name: envelope.name,
    type: envelope.t,
    sourceRole: envelope.src.role,
    sourceId: envelope.src.id
  };
}

export function summarizeHelloAck(data: HelloAck): Record<string, unknown> {
  return {
    authOK: data.auth_ok,
    sessionID: data.session_id,
    capabilitiesSupported: [...data.capabilities_supported]
  };
}

function defaultAgentDiagnosticSink(entry: AgentDiagnosticEntry): void {
  console.info("[panex:agent]", entry.event, entry.detail);
}

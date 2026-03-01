export const PROTOCOL_VERSION = 1;

export const envelopeTypes = ["lifecycle", "event", "command"] as const;
export type EnvelopeType = (typeof envelopeTypes)[number];

export const sourceRoles = ["daemon", "dev-agent", "inspector"] as const;
export type SourceRole = (typeof sourceRoles)[number];

export const envelopeNames = [
  "hello",
  "welcome",
  "build.complete",
  "context.log",
  "command.reload",
  "query.events",
  "query.events.result"
] as const;
export type EnvelopeName = (typeof envelopeNames)[number];

export const messageTypeByName: Readonly<Record<EnvelopeName, EnvelopeType>> = {
  hello: "lifecycle",
  welcome: "lifecycle",
  "build.complete": "event",
  "context.log": "event",
  "command.reload": "command",
  "query.events": "command",
  "query.events.result": "event"
};

export interface Source {
  role: SourceRole;
  id: string;
}

export interface Envelope<TData = unknown> {
  v: number;
  t: EnvelopeType;
  name: EnvelopeName;
  src: Source;
  data: TData;
}

export interface Hello {
  protocol_version: number;
  capabilities?: string[];
}

export interface Welcome {
  protocol_version: number;
  session_id: string;
  server_version: string;
}

export interface BuildComplete {
  build_id: string;
  success: boolean;
  duration_ms: number;
  changed_files?: string[];
}

export interface ContextLog {
  context_id: string;
  level: string;
  message: string;
  timestamp_s: number;
}

export interface CommandReload {
  reason: string;
  build_id?: string;
}

export interface QueryEvents {
  limit?: number;
}

export interface EventSnapshot {
  id: number;
  recorded_at_ms: number;
  envelope: Envelope;
}

export interface QueryEventsResult {
  events: EventSnapshot[];
}

export function isEnvelopeType(value: unknown): value is EnvelopeType {
  return typeof value === "string" && (envelopeTypes as readonly string[]).includes(value);
}

export function isEnvelopeName(value: unknown): value is EnvelopeName {
  return typeof value === "string" && (envelopeNames as readonly string[]).includes(value);
}

export function isSourceRole(value: unknown): value is SourceRole {
  return typeof value === "string" && (sourceRoles as readonly string[]).includes(value);
}

export function messageTypeForName(name: EnvelopeName): EnvelopeType {
  return messageTypeByName[name];
}

export function isEnvelope(value: unknown): value is Envelope {
  if (!isRecord(value)) {
    return false;
  }

  if (typeof value.v !== "number") {
    return false;
  }
  if (!isEnvelopeType(value.t)) {
    return false;
  }
  if (!isEnvelopeName(value.name)) {
    return false;
  }
  if (messageTypeByName[value.name] !== value.t) {
    return false;
  }
  if (!isRecord(value.src)) {
    return false;
  }
  if (!isSourceRole(value.src.role)) {
    return false;
  }
  if (typeof value.src.id !== "string" || value.src.id.trim().length === 0) {
    return false;
  }

  // `data` remains message-specific and is validated by dedicated handlers.
  return "data" in value;
}

export function isWelcome(envelope: Envelope): envelope is Envelope<Welcome> {
  return envelope.t === "lifecycle" && envelope.name === "welcome";
}

export function isQueryEventsResult(envelope: Envelope): envelope is Envelope<QueryEventsResult> {
  return envelope.t === "event" && envelope.name === "query.events.result";
}

export function isReloadCommand(envelope: Envelope): envelope is Envelope<CommandReload> {
  return envelope.t === "command" && envelope.name === "command.reload";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

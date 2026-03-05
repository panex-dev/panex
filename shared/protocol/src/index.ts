export const PROTOCOL_VERSION = 1;

export const envelopeTypes = ["lifecycle", "event", "command"] as const;
export type EnvelopeType = (typeof envelopeTypes)[number];

export const sourceRoles = ["daemon", "dev-agent", "inspector"] as const;
export type SourceRole = (typeof sourceRoles)[number];

export const envelopeNames = [
  "hello",
  "hello.ack",
  "build.complete",
  "context.log",
  "command.reload",
  "query.events",
  "query.events.result",
  "query.storage",
  "query.storage.result",
  "storage.diff",
  "storage.set",
  "storage.remove",
  "storage.clear",
  "chrome.api.call",
  "chrome.api.result",
  "chrome.api.event"
] as const;
export type EnvelopeName = (typeof envelopeNames)[number];

export const messageTypeByName: Readonly<Record<EnvelopeName, EnvelopeType>> = {
  hello: "lifecycle",
  "hello.ack": "lifecycle",
  "build.complete": "event",
  "context.log": "event",
  "command.reload": "command",
  "query.events": "command",
  "query.events.result": "event",
  "query.storage": "command",
  "query.storage.result": "event",
  "storage.diff": "event",
  "storage.set": "command",
  "storage.remove": "command",
  "storage.clear": "command",
  "chrome.api.call": "command",
  "chrome.api.result": "event",
  "chrome.api.event": "event"
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
  client_kind?: string;
  client_version?: string;
  capabilities_requested?: string[];
  // Retained for backward compatibility with early clients.
  capabilities?: string[];
}

export interface HelloAck {
  protocol_version: number;
  daemon_version: string;
  session_id: string;
  auth_ok: boolean;
  capabilities_supported: string[];
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

export interface QueryStorage {
  area?: string;
}

export interface StorageSnapshot {
  area: string;
  items: Record<string, unknown>;
}

export interface QueryStorageResult {
  snapshots: StorageSnapshot[];
}

export interface StorageChange {
  key: string;
  old_value?: unknown;
  new_value?: unknown;
}

export interface StorageDiff {
  area: string;
  changes: StorageChange[];
}

export interface StorageSet {
  area: string;
  key: string;
  value: unknown;
}

export interface StorageRemove {
  area: string;
  key: string;
}

export interface StorageClear {
  area: string;
}

export interface ChromeAPICall {
  call_id: string;
  namespace: string;
  method: string;
  args?: unknown[];
}

export interface ChromeAPIResult {
  call_id: string;
  success: boolean;
  data?: unknown;
  error?: string;
}

export interface ChromeAPIEvent {
  namespace: string;
  event: string;
  args?: unknown[];
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

export function isHelloAck(envelope: Envelope): envelope is Envelope<HelloAck> {
  return envelope.t === "lifecycle" && envelope.name === "hello.ack";
}

export function isQueryEventsResult(envelope: Envelope): envelope is Envelope<QueryEventsResult> {
  return envelope.t === "event" && envelope.name === "query.events.result";
}

export function isQueryStorageResult(envelope: Envelope): envelope is Envelope<QueryStorageResult> {
  return envelope.t === "event" && envelope.name === "query.storage.result";
}

export function isReloadCommand(envelope: Envelope): envelope is Envelope<CommandReload> {
  return envelope.t === "command" && envelope.name === "command.reload";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

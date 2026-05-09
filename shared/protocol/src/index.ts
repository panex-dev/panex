export const PROTOCOL_VERSION = 1;
export const MAX_WEBSOCKET_MESSAGE_BYTES = 1 << 20;
export const DEFAULT_DAEMON_WEBSOCKET_PATH = "/ws";
export const DEFAULT_DAEMON_WEBSOCKET_URL = "ws://127.0.0.1:4317/ws";
export const DEFAULT_FIRST_PARTY_CLIENT_VERSION = "dev";
export const DEV_AGENT_CLIENT_KIND = "dev-agent";
export const INSPECTOR_CLIENT_KIND = "inspector";
export const CHROME_SIM_CLIENT_KIND = "chrome-sim";

export const envelopeTypes = ["lifecycle", "event", "command"] as const;
export type EnvelopeType = (typeof envelopeTypes)[number];

export const sourceRoles = ["daemon", "dev-agent", "chrome-sim", "inspector"] as const;
export type SourceRole = (typeof sourceRoles)[number];

export const envelopeNames = [
  "hello",
  "hello.ack",
  "build.complete",
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

export const negotiableCapabilityNames = [
  "build.complete",
  "command.reload",
  "query.events",
  "query.storage",
  "storage.diff",
  "storage.set",
  "storage.remove",
  "storage.clear",
  "chrome.api.call",
  "chrome.api.event"
] as const;
export type NegotiableCapabilityName = (typeof negotiableCapabilityNames)[number];

export const firstPartyClientKinds = ["dev-agent", "inspector", "chrome-sim"] as const;
export type FirstPartyClientKind = (typeof firstPartyClientKinds)[number];

export const firstPartySourceRolesByClientKind = {
  "dev-agent": "dev-agent",
  inspector: "inspector",
  "chrome-sim": "chrome-sim"
} as const satisfies Readonly<Record<FirstPartyClientKind, Exclude<SourceRole, "daemon">>>;

export const firstPartyRequestedCapabilities = {
  "dev-agent": ["command.reload"],
  inspector: [
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
  ],
  "chrome-sim": ["chrome.api.call", "chrome.api.event", "storage.diff"]
} as const satisfies Readonly<Record<FirstPartyClientKind, readonly NegotiableCapabilityName[]>>;

export const messageTypeByName: Readonly<Record<EnvelopeName, EnvelopeType>> = {
  hello: "lifecycle",
  "hello.ack": "lifecycle",
  "build.complete": "event",
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
  auth_token?: string;
  client_kind?: string;
  client_version?: string;
  extension_id?: string;
  capabilities_requested?: string[];
  // Retained for backward compatibility with early clients.
  capabilities?: string[];
}

export interface HelloAck {
  protocol_version: number;
  daemon_version: string;
  session_id: string;
  auth_ok: boolean;
  extension_id?: string;
  capabilities_supported: string[];
}

export interface BuildComplete {
  build_id: string;
  success: boolean;
  duration_ms: number;
  extension_id?: string;
  triggering_files?: string[];
  diagnostics?: string[];
}

export interface CommandReload {
  reason: string;
  build_id?: string;
  extension_id?: string;
}

export interface QueryEvents {
  limit?: number;
  before_id?: number;
}

export interface EventSnapshot {
  id: number;
  recorded_at_ms: number;
  envelope: Envelope;
}

export interface QueryEventsResult {
  events: EventSnapshot[];
  has_more: boolean;
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

export type WebSocketMessageDataResult =
  | { kind: "bytes"; bytes: Uint8Array }
  | { kind: "too_large"; size: number }
  | { kind: "unsupported" };

export function readWebSocketMessageData(
  raw: unknown,
  maxBytes = MAX_WEBSOCKET_MESSAGE_BYTES
): WebSocketMessageDataResult {
  let bytes: Uint8Array | null = null;
  if (raw instanceof Uint8Array) {
    bytes = raw;
  } else if (raw instanceof ArrayBuffer) {
    bytes = new Uint8Array(raw);
  } else if (ArrayBuffer.isView(raw)) {
    bytes = new Uint8Array(raw.buffer, raw.byteOffset, raw.byteLength);
  }

  if (!bytes) {
    return { kind: "unsupported" };
  }
  if (bytes.byteLength > maxBytes) {
    return { kind: "too_large", size: bytes.byteLength };
  }
  return { kind: "bytes", bytes };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

// --- Daemon URL utilities ---

const loopbackHosts = new Set(["127.0.0.1", "localhost"]);

/**
 * Strip the token query parameter from a daemon WebSocket URL.
 * Used to build a display-safe URL that does not leak credentials.
 */
export function buildDaemonURL(wsURL: string): string {
  const url = new URL(wsURL);
  url.searchParams.delete("token");
  return url.toString();
}

/**
 * Return `value` trimmed if non-empty, otherwise `fallback`.
 */
export function nonEmpty(value: string | null | undefined, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

/**
 * Validate and normalize a daemon WebSocket URL.
 * Only allows ws:// protocol on loopback hosts with /ws path.
 * Returns `fallback` for any invalid input.
 */
export function normalizeDaemonWebSocketURL(
  value: string | null | undefined,
  fallback: string
): string {
  const trimmed = nonEmpty(value, "");
  if (trimmed === "") {
    return fallback;
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return fallback;
  }

  if (
    parsed.protocol !== "ws:" ||
    !loopbackHosts.has(parsed.hostname) ||
    parsed.pathname !== DEFAULT_DAEMON_WEBSOCKET_PATH ||
    parsed.username !== "" ||
    parsed.password !== ""
  ) {
    return fallback;
  }

  parsed.hash = "";
  parsed.searchParams.delete("token");
  return parsed.toString();
}

import { decode, encode } from "@msgpack/msgpack";
import {
  PROTOCOL_VERSION,
  isEnvelope,
  isHelloAck,
  isQueryEventsResult,
  isQueryStorageResult,
  readWebSocketMessageData,
  type ChromeAPICall,
  type Envelope,
  type Hello,
  type QueryEvents,
  type QueryEventsResult,
  type QueryStorage,
  type QueryStorageResult,
  type StorageClear,
  type StorageDiff,
  type StorageRemove,
  type StorageSet,
  type StorageSnapshot
} from "@panex/protocol";
import {
  createContext,
  createSignal,
  onCleanup,
  useContext,
  type Accessor,
  type ParentProps
} from "solid-js";

import { reconnectDelay } from "./reconnect";
import {
  applyStorageDiff,
  normalizeStorageSnapshots,
  type AppliedStorageDiff
} from "./storage";
import {
  defaultTimelineLimit,
  fromLiveEnvelope,
  fromSnapshot,
  mergeEntries,
  type TimelineEntry
} from "./timeline";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

const defaultDaemonWSURL = "ws://127.0.0.1:4317/ws";
const defaultDaemonToken = "dev-token";
const loopbackHosts = new Set(["127.0.0.1", "localhost"]);
const closeMessageTooBig = 1009;

interface ConnectionContextValue {
  status: Accessor<ConnectionStatus>;
  timeline: Accessor<TimelineEntry[]>;
  storage: Accessor<StorageSnapshot[]>;
  storageHighlights: Accessor<Set<string>>;
  lastError: Accessor<string | null>;
  socketURL: Accessor<string>;
  send: (envelope: Envelope) => boolean;
  refreshStorage: (area?: QueryStorage["area"]) => boolean;
  setStorageItem: (area: string, key: string, value: unknown) => boolean;
  removeStorageItem: (area: string, key: string) => boolean;
  clearStorageArea: (area: string) => boolean;
  sendRuntimeMessage: (message: unknown) => boolean;
}

const ConnectionContext = createContext<ConnectionContextValue>();
const inspectorID = `inspector-${safeClientID()}`;

export function ConnectionProvider(props: ParentProps) {
  const [status, setStatus] = createSignal<ConnectionStatus>("connecting");
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [storage, setStorage] = createSignal<StorageSnapshot[]>([]);
  const [storageHighlights, setStorageHighlights] = createSignal(new Set<string>());
  const [lastError, setLastError] = createSignal<string | null>(null);
  const { wsURL, token } = resolveConnectionParams();
  const daemonURL = buildDaemonURL(wsURL, token);

  let socket: WebSocket | null = null;
  let reconnectTimer: number | undefined;
  let reconnectAttempt = 0;
  let stopped = false;
  let chromeCallSeq = 0;

  const send = (envelope: Envelope): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    socket.send(encode(envelope));
    return true;
  };

  const refreshStorage = (area?: QueryStorage["area"]): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    socket.send(encode(buildStorageQuery(area)));
    return true;
  };

  const setStorageItem = (area: string, key: string, value: unknown): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    const command = buildStorageSet(area, key, value);
    if (!command) {
      return false;
    }

    socket.send(encode(command));
    return true;
  };

  const removeStorageItem = (area: string, key: string): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    const command = buildStorageRemove(area, key);
    if (!command) {
      return false;
    }

    socket.send(encode(command));
    return true;
  };

  const clearStorageArea = (area: string): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    const command = buildStorageClear(area);
    if (!command) {
      return false;
    }

    socket.send(encode(command));
    return true;
  };

  const sendRuntimeMessage = (message: unknown): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    socket.send(encode(buildChromeRuntimeSendMessage(message, ++chromeCallSeq)));
    return true;
  };

  const connect = () => {
    if (stopped) {
      return;
    }

    const next = new WebSocket(daemonURL);
    next.binaryType = "arraybuffer";
    socket = next;

    next.addEventListener("open", () => {
      if (socket !== next) {
        return;
      }

      reconnectAttempt = 0;
      setLastError(null);

      const hello: Envelope<Hello> = {
        v: PROTOCOL_VERSION,
        t: "lifecycle",
        name: "hello",
        src: { role: "inspector", id: inspectorID },
        data: {
          protocol_version: PROTOCOL_VERSION,
          client_kind: "inspector",
          client_version: "dev",
          capabilities_requested: [
            "query.events",
            "query.storage",
            "storage.diff",
            "storage.set",
            "storage.remove",
            "storage.clear",
            "chrome.api.call",
            "chrome.api.result",
            "chrome.api.event"
          ]
        }
      };

      next.send(encode(hello));
    });

    next.addEventListener("message", (event) => {
      const message = readWebSocketMessageData(event.data);
      if (message.kind === "unsupported") {
        return;
      }
      if (message.kind === "too_large") {
        setLastError(`daemon message exceeded ${message.size} bytes`);
        next.close(closeMessageTooBig, "message exceeds limit");
        return;
      }

      let decoded: unknown;
      try {
        decoded = decode(message.bytes);
      } catch {
        return;
      }
      if (!isEnvelope(decoded)) {
        return;
      }

      if (isHelloAck(decoded)) {
        if (!decoded.data.auth_ok) {
          setLastError("handshake rejected by daemon");
          next.close();
          return;
        }

        setStatus("open");
        setLastError(null);

        const query: Envelope<QueryEvents> = {
          v: PROTOCOL_VERSION,
          t: "command",
          name: "query.events",
          src: { role: "inspector", id: inspectorID },
          data: { limit: defaultTimelineLimit }
        };
        next.send(encode(query));
        next.send(encode(buildStorageQuery()));
        return;
      }

      if (isQueryEventsResult(decoded)) {
        applyQueryResult(decoded.data, setTimeline);
        return;
      }

      if (isQueryStorageResult(decoded)) {
        applyStorageQueryResult(decoded.data, setStorage, setStorageHighlights);
      }

      if (isStorageDiffEnvelope(decoded)) {
        applyStorageDiffEnvelope(decoded.data, storage, setStorage, setStorageHighlights);
      }

      if (decoded.name === "query.storage.result") {
        return;
      }

      setTimeline((existing) =>
        mergeEntries(existing, [fromLiveEnvelope(decoded)], defaultTimelineLimit)
      );
    });

    next.addEventListener("close", () => {
      if (socket !== next) {
        return;
      }
      if (stopped) {
        setStatus("closed");
        return;
      }

      setStatus("reconnecting");
      const delay = reconnectDelay(reconnectAttempt);
      reconnectAttempt += 1;

      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = undefined;
        connect();
      }, delay);
    });

    next.addEventListener("error", () => {
      if (socket !== next) {
        return;
      }

      setLastError("websocket transport error");
      next.close();
    });
  };

  connect();

  onCleanup(() => {
    stopped = true;
    if (typeof reconnectTimer === "number") {
      window.clearTimeout(reconnectTimer);
    }
    socket?.close();
    setStatus("closed");
  });

  return ConnectionContext.Provider({
    value: {
      status,
      timeline,
      storage,
      storageHighlights,
      lastError,
      socketURL: () => daemonURL,
      send,
      refreshStorage,
      setStorageItem,
      removeStorageItem,
      clearStorageArea,
      sendRuntimeMessage
    },
    get children() {
      return props.children;
    }
  });
}

export function useConnection(): ConnectionContextValue {
  const context = useContext(ConnectionContext);
  if (!context) {
    throw new Error("useConnection must be used within ConnectionProvider");
  }
  return context;
}

function applyQueryResult(
  payload: QueryEventsResult,
  setTimeline: (updater: (existing: TimelineEntry[]) => TimelineEntry[]) => void
): void {
  if (!Array.isArray(payload.events)) {
    return;
  }

  const snapshots = payload.events.filter((event): event is QueryEventsResult["events"][number] => {
    return (
      typeof event === "object" &&
      event !== null &&
      typeof event.id === "number" &&
      typeof event.recorded_at_ms === "number" &&
      isEnvelope(event.envelope)
    );
  });

  setTimeline((existing) =>
    mergeEntries(
      existing,
      snapshots.map((snapshot) => fromSnapshot(snapshot)),
      defaultTimelineLimit
    )
  );
}

function applyStorageQueryResult(
  payload: QueryStorageResult,
  setStorage: (next: StorageSnapshot[]) => void,
  setStorageHighlights: (next: Set<string>) => void
): void {
  setStorage(normalizeStorageSnapshots(payload));
  setStorageHighlights(new Set());
}

function applyStorageDiffEnvelope(
  payload: StorageDiff,
  storage: Accessor<StorageSnapshot[]>,
  setStorage: (next: StorageSnapshot[]) => void,
  setStorageHighlights: (next: Set<string>) => void
): void {
  const next = applyStorageDiff(storage(), payload);
  applyStorageDiffState(next, setStorage, setStorageHighlights);
}

function applyStorageDiffState(
  next: AppliedStorageDiff,
  setStorage: (next: StorageSnapshot[]) => void,
  setStorageHighlights: (next: Set<string>) => void
): void {
  if (next.changedRowIDs.length === 0) {
    return;
  }

  setStorage(next.snapshots);
  setStorageHighlights(new Set(next.changedRowIDs));
}

function buildStorageQuery(area?: QueryStorage["area"]): Envelope<QueryStorage> {
  const normalizedArea = normalizeStorageArea(area);

  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "query.storage",
    src: { role: "inspector", id: inspectorID },
    data: typeof normalizedArea === "string" ? { area: normalizedArea } : {}
  };
}

function buildStorageSet(area: string, key: string, value: unknown): Envelope<StorageSet> | null {
  const normalizedArea = normalizeStorageArea(area);
  const normalizedKey = normalizeStorageKey(key);
  if (!normalizedArea || !normalizedKey) {
    return null;
  }

  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "storage.set",
    src: { role: "inspector", id: inspectorID },
    data: {
      area: normalizedArea,
      key: normalizedKey,
      value
    }
  };
}

function buildStorageRemove(area: string, key: string): Envelope<StorageRemove> | null {
  const normalizedArea = normalizeStorageArea(area);
  const normalizedKey = normalizeStorageKey(key);
  if (!normalizedArea || !normalizedKey) {
    return null;
  }

  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "storage.remove",
    src: { role: "inspector", id: inspectorID },
    data: {
      area: normalizedArea,
      key: normalizedKey
    }
  };
}

function buildStorageClear(area: string): Envelope<StorageClear> | null {
  const normalizedArea = normalizeStorageArea(area);
  if (!normalizedArea) {
    return null;
  }

  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "storage.clear",
    src: { role: "inspector", id: inspectorID },
    data: {
      area: normalizedArea
    }
  };
}

function buildChromeRuntimeSendMessage(
  message: unknown,
  seq: number
): Envelope<ChromeAPICall> {
  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "chrome.api.call",
    src: { role: "inspector", id: inspectorID },
    data: {
      call_id: `runtime-send-${seq}`,
      namespace: "runtime",
      method: "sendMessage",
      args: [message]
    }
  };
}

function resolveConnectionParams(): { wsURL: string; token: string } {
  return resolveConnectionParamsFromSearch(window.location.search, isEmbeddedContext());
}

export function resolveConnectionParamsFromSearch(
  search: string,
  embedded = false
): { wsURL: string; token: string } {
  if (embedded) {
    return {
      wsURL: defaultDaemonWSURL,
      token: defaultDaemonToken
    };
  }

  const params = new URLSearchParams(search);
  return {
    wsURL: normalizeDaemonWebSocketURL(params.get("ws"), defaultDaemonWSURL),
    token: nonEmpty(params.get("token"), defaultDaemonToken)
  };
}

function buildDaemonURL(wsURL: string, token: string): string {
  const url = new URL(wsURL);
  url.searchParams.set("token", token);
  return url.toString();
}

function safeClientID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `${Date.now()}`;
}

function nonEmpty(value: string | null, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

function normalizeDaemonWebSocketURL(value: string | null, fallback: string): string {
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
    parsed.pathname !== "/ws" ||
    parsed.username !== "" ||
    parsed.password !== ""
  ) {
    return fallback;
  }

  parsed.hash = "";
  return parsed.toString();
}

function isEmbeddedContext(): boolean {
  return window.self !== window.top;
}

function normalizeStorageArea(
  area: QueryStorage["area"]
): "local" | "sync" | "session" | undefined {
  if (typeof area !== "string") {
    return undefined;
  }

  const trimmed = area.trim().toLowerCase();
  if (trimmed === "local" || trimmed === "sync" || trimmed === "session") {
    return trimmed;
  }

  return undefined;
}

function normalizeStorageKey(key: string): string | undefined {
  const trimmed = key.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function isStorageDiffEnvelope(envelope: Envelope): envelope is Envelope<StorageDiff> {
  return envelope.t === "event" && envelope.name === "storage.diff";
}

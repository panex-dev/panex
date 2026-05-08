import { decode, encode } from "@msgpack/msgpack";
import {
  PROTOCOL_VERSION,
  buildDaemonURL,
  firstPartyRequestedCapabilities,
  isEnvelope,
  isHelloAck,
  isQueryEventsResult,
  isQueryStorageResult,
  nonEmpty,
  normalizeDaemonWebSocketURL,
  readWebSocketMessageData,
  type ChromeAPICall,
  type Envelope,
  type Hello,
  type HelloAck,
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
  defaultTimelineWorkingSetLimit,
  fromLiveEnvelope,
  fromSnapshot,
  mergeEntriesWithOverflow,
  oldestPersistedTimelineID,
  type TimelineEntry
} from "./timeline";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

export interface BridgeSession {
  daemonVersion: string;
  sessionID: string;
  extensionID: string | null;
  capabilitiesSupported: string[];
}

export function bridgeSessionSupportsCapability(
  session: BridgeSession | null,
  capability: string
): boolean {
  return !!session?.capabilitiesSupported.includes(capability);
}

const defaultDaemonWSURL = "ws://127.0.0.1:4317/ws";
const defaultDaemonToken = "";
const closeMessageTooBig = 1009;
export const inspectorRequestedCapabilities = firstPartyRequestedCapabilities.inspector;

interface ConnectionContextValue {
  status: Accessor<ConnectionStatus>;
  bridgeSession: Accessor<BridgeSession | null>;
  timeline: Accessor<TimelineEntry[]>;
  canLoadOlderTimeline: Accessor<boolean>;
  loadingOlderTimeline: Accessor<boolean>;
  loadingLatestTimeline: Accessor<boolean>;
  trimmedOlderTimelineCount: Accessor<number>;
  trimmedNewerTimelineCount: Accessor<number>;
  storage: Accessor<StorageSnapshot[]>;
  storageHighlights: Accessor<Set<string>>;
  lastError: Accessor<string | null>;
  socketURL: Accessor<string>;
  send: (envelope: Envelope) => boolean;
  loadOlderTimeline: () => boolean;
  jumpToLatestTimeline: () => boolean;
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
  const [bridgeSession, setBridgeSession] = createSignal<BridgeSession | null>(null);
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [canLoadOlderTimeline, setCanLoadOlderTimeline] = createSignal(false);
  const [loadingOlderTimeline, setLoadingOlderTimeline] = createSignal(false);
  const [loadingLatestTimeline, setLoadingLatestTimeline] = createSignal(false);
  const [trimmedOlderTimelineCount, setTrimmedOlderTimelineCount] = createSignal(0);
  const [trimmedNewerTimelineCount, setTrimmedNewerTimelineCount] = createSignal(0);
  const [storage, setStorage] = createSignal<StorageSnapshot[]>([]);
  const [storageHighlights, setStorageHighlights] = createSignal(new Set<string>());
  const [lastError, setLastError] = createSignal<string | null>(null);
  const { wsURL, token } = resolveConnectionParams();
  const daemonURL = buildDaemonURL(wsURL);

  let socket: WebSocket | null = null;
  let reconnectTimer: number | undefined;
  let reconnectAttempt = 0;
  let stopped = false;
  let chromeCallSeq = 0;
  let pendingOlderTimelineBeforeID: number | null = null;
  let pendingLatestTimelineReset = false;
  let bufferedLatestTimelineLiveEntries: TimelineEntry[] = [];

  const send = (envelope: Envelope): boolean => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }

    socket.send(encode(envelope));
    return true;
  };

  const loadOlderTimeline = (): boolean => {
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "query.events") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN ||
      loadingOlderTimeline() ||
      loadingLatestTimeline()
    ) {
      return false;
    }

    const beforeID = oldestPersistedTimelineID(timeline());
    if (beforeID === null || !canLoadOlderTimeline()) {
      return false;
    }

    pendingOlderTimelineBeforeID = beforeID;
    setLoadingOlderTimeline(true);
    socket.send(encode(buildTimelineQuery(defaultTimelineLimit, beforeID)));
    return true;
  };

  const jumpToLatestTimeline = (): boolean => {
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "query.events") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN ||
      loadingOlderTimeline() ||
      loadingLatestTimeline() ||
      trimmedNewerTimelineCount() === 0
    ) {
      return false;
    }

    pendingOlderTimelineBeforeID = null;
    pendingLatestTimelineReset = true;
    bufferedLatestTimelineLiveEntries = [];
    setLoadingLatestTimeline(true);
    socket.send(encode(buildTimelineQuery(defaultTimelineLimit)));
    return true;
  };

  const refreshStorage = (area?: QueryStorage["area"]): boolean => {
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "query.storage") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN
    ) {
      return false;
    }

    socket.send(encode(buildStorageQuery(area)));
    return true;
  };

  const setStorageItem = (area: string, key: string, value: unknown): boolean => {
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "storage.set") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN
    ) {
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
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "storage.remove") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN
    ) {
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
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "storage.clear") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN
    ) {
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
    if (
      !bridgeSessionSupportsCapability(bridgeSession(), "chrome.api.call") ||
      !socket ||
      socket.readyState !== WebSocket.OPEN
    ) {
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
      pendingOlderTimelineBeforeID = null;
      pendingLatestTimelineReset = false;
      bufferedLatestTimelineLiveEntries = [];
      setLoadingOlderTimeline(false);
      setLoadingLatestTimeline(false);
      setCanLoadOlderTimeline(false);
      setTrimmedOlderTimelineCount(0);
      setTrimmedNewerTimelineCount(0);
      setBridgeSession(null);
      setTimeline([]);
      setStorage([]);
      setStorageHighlights(new Set<string>());

      const hello: Envelope<Hello> = {
        v: PROTOCOL_VERSION,
        t: "lifecycle",
        name: "hello",
        src: { role: "inspector", id: inspectorID },
        data: {
          protocol_version: PROTOCOL_VERSION,
          auth_token: token,
          client_kind: "inspector",
          client_version: "dev",
          capabilities_requested: [...inspectorRequestedCapabilities]
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
        if (decoded.data.protocol_version !== PROTOCOL_VERSION) {
          setLastError("protocol version mismatch");
          next.close();
          return;
        }

        if (!decoded.data.auth_ok) {
          setLastError("handshake rejected by daemon");
          next.close();
          return;
        }

        const nextBridgeSession = bridgeSessionFromHelloAck(decoded.data);
        setBridgeSession(nextBridgeSession);
        setStatus("open");
        setLastError(null);
        for (const message of buildPostHelloAckMessages(nextBridgeSession)) {
          next.send(encode(message));
        }
        return;
      }

      if (isQueryEventsResult(decoded)) {
        if (pendingLatestTimelineReset) {
          pendingLatestTimelineReset = false;
          setLoadingLatestTimeline(false);
          applyLatestQueryResult(
            decoded.data,
            bufferedLatestTimelineLiveEntries,
            setTimeline,
            setTrimmedOlderTimelineCount,
            setTrimmedNewerTimelineCount
          );
          bufferedLatestTimelineLiveEntries = [];
          setCanLoadOlderTimeline(decoded.data.has_more === true);
          return;
        }

        const mergePosition = pendingOlderTimelineBeforeID === null ? "append" : "prepend";
        pendingOlderTimelineBeforeID = null;
        setLoadingOlderTimeline(false);
        applyQueryResult(
          decoded.data,
          setTimeline,
          setTrimmedOlderTimelineCount,
          setTrimmedNewerTimelineCount,
          defaultTimelineWorkingSetLimit,
          mergePosition
        );
        setCanLoadOlderTimeline(decoded.data.has_more === true);
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

      const liveEntry = fromLiveEnvelope(decoded);
      if (pendingLatestTimelineReset) {
        bufferedLatestTimelineLiveEntries = mergeEntriesWithOverflow(
          bufferedLatestTimelineLiveEntries,
          [liveEntry],
          defaultTimelineWorkingSetLimit
        ).entries;
        return;
      }

      let droppedOldest = 0;
      setTimeline((existing) => {
        const merged = mergeEntriesWithOverflow(
          existing,
          [liveEntry],
          defaultTimelineWorkingSetLimit
        );
        droppedOldest = merged.droppedOldest;
        return merged.entries;
      });
      if (droppedOldest > 0) {
        setTrimmedOlderTimelineCount((count) => count + droppedOldest);
      }
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
      pendingOlderTimelineBeforeID = null;
      pendingLatestTimelineReset = false;
      bufferedLatestTimelineLiveEntries = [];
      setLoadingOlderTimeline(false);
      setLoadingLatestTimeline(false);
      setCanLoadOlderTimeline(false);
      setBridgeSession(null);
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
      bridgeSession,
      timeline,
      canLoadOlderTimeline,
      loadingOlderTimeline,
      loadingLatestTimeline,
      trimmedOlderTimelineCount,
      trimmedNewerTimelineCount,
      storage,
      storageHighlights,
      lastError,
      socketURL: () => daemonURL,
      send,
      loadOlderTimeline,
      jumpToLatestTimeline,
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

export function bridgeSessionFromHelloAck(data: HelloAck): BridgeSession {
  return {
    daemonVersion: data.daemon_version,
    sessionID: data.session_id,
    extensionID:
      typeof data.extension_id === "string" && data.extension_id.trim().length > 0
        ? data.extension_id
        : null,
    capabilitiesSupported: Array.isArray(data.capabilities_supported)
      ? data.capabilities_supported.filter((value): value is string => typeof value === "string")
      : []
  };
}

export function buildPostHelloAckMessages(session: BridgeSession): Envelope[] {
  const messages: Envelope[] = [];
  if (bridgeSessionSupportsCapability(session, "query.events")) {
    messages.push(buildTimelineQuery(defaultTimelineLimit));
  }
  if (bridgeSessionSupportsCapability(session, "query.storage")) {
    messages.push(buildStorageQuery());
  }
  return messages;
}

function applyQueryResult(
  payload: QueryEventsResult,
  setTimeline: (updater: (existing: TimelineEntry[]) => TimelineEntry[]) => void,
  setTrimmedOlderTimelineCount: (updater: (count: number) => number) => void,
  setTrimmedNewerTimelineCount: (updater: (count: number) => number) => void,
  maxEntries: number,
  mergePosition: "append" | "prepend"
): void {
  const snapshots = normalizeQuerySnapshots(payload);
  let droppedOldest = 0;
  let droppedNewest = 0;
  setTimeline((existing) => {
    const merged = mergeEntriesWithOverflow(
      existing,
      snapshots.map((snapshot) => fromSnapshot(snapshot)),
      maxEntries,
      mergePosition
    );
    droppedOldest = merged.droppedOldest;
    droppedNewest = merged.droppedNewest;
    return merged.entries;
  });
  if (droppedOldest > 0) {
    setTrimmedOlderTimelineCount((count) => count + droppedOldest);
  }
  if (droppedNewest > 0) {
    setTrimmedNewerTimelineCount((count) => count + droppedNewest);
  }
}

function applyLatestQueryResult(
  payload: QueryEventsResult,
  liveEntries: TimelineEntry[],
  setTimeline: (next: TimelineEntry[]) => void,
  setTrimmedOlderTimelineCount: (next: number) => void,
  setTrimmedNewerTimelineCount: (next: number) => void
): void {
  const snapshots = normalizeQuerySnapshots(payload).map((snapshot) => fromSnapshot(snapshot));
  const merged = mergeEntriesWithOverflow(
    snapshots,
    liveEntries,
    defaultTimelineWorkingSetLimit
  );
  setTimeline(merged.entries);
  setTrimmedOlderTimelineCount(merged.droppedOldest);
  setTrimmedNewerTimelineCount(0);
}

function normalizeQuerySnapshots(payload: QueryEventsResult): QueryEventsResult["events"] {
  if (!Array.isArray(payload.events)) {
    return [];
  }

  return payload.events.filter((event): event is QueryEventsResult["events"][number] => {
    return (
      typeof event === "object" &&
      event !== null &&
      typeof event.id === "number" &&
      typeof event.recorded_at_ms === "number" &&
      isEnvelope(event.envelope)
    );
  });
}

export function buildTimelineQuery(limit = defaultTimelineLimit, beforeID?: number): Envelope<QueryEvents> {
  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "query.events",
    src: { role: "inspector", id: inspectorID },
    data: typeof beforeID === "number" ? { limit, before_id: beforeID } : { limit }
  };
}

function applyStorageQueryResult(
  payload: QueryStorageResult,
  setStorage: (next: StorageSnapshot[]) => void,
  setStorageHighlights: (next: Set<string>) => void
): void {
  setStorage(normalizeStorageSnapshots(payload));
  setStorageHighlights(new Set<string>());
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

function safeClientID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    const buf = new Uint8Array(16);
    crypto.getRandomValues(buf);
    return Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
  }

  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
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

import { decode, encode } from "@msgpack/msgpack";
import {
  PROTOCOL_VERSION,
  isEnvelope,
  isHelloAck,
  isQueryEventsResult,
  isQueryStorageResult,
  type Envelope,
  type Hello,
  type QueryEvents,
  type QueryEventsResult,
  type QueryStorage,
  type QueryStorageResult,
  type StorageSnapshot
} from "../../shared/protocol/src/index";
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
  defaultTimelineLimit,
  fromLiveEnvelope,
  fromSnapshot,
  mergeEntries,
  type TimelineEntry
} from "./timeline";
import { normalizeStorageSnapshots } from "./storage";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

interface ConnectionContextValue {
  status: Accessor<ConnectionStatus>;
  timeline: Accessor<TimelineEntry[]>;
  storage: Accessor<StorageSnapshot[]>;
  lastError: Accessor<string | null>;
  socketURL: Accessor<string>;
  send: (envelope: Envelope) => boolean;
  refreshStorage: (area?: QueryStorage["area"]) => boolean;
}

const ConnectionContext = createContext<ConnectionContextValue>();
const inspectorID = `inspector-${safeClientID()}`;

export function ConnectionProvider(props: ParentProps) {
  const [status, setStatus] = createSignal<ConnectionStatus>("connecting");
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [storage, setStorage] = createSignal<StorageSnapshot[]>([]);
  const [lastError, setLastError] = createSignal<string | null>(null);
  const { wsURL, token } = resolveConnectionParams();
  const daemonURL = buildDaemonURL(wsURL, token);

  let socket: WebSocket | null = null;
  let reconnectTimer: number | undefined;
  let reconnectAttempt = 0;
  let stopped = false;

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
          capabilities_requested: ["query.events", "query.storage"]
        }
      };

      next.send(encode(hello));
    });

    next.addEventListener("message", (event) => {
      if (!(event.data instanceof ArrayBuffer)) {
        return;
      }

      let decoded: unknown;
      try {
        decoded = decode(new Uint8Array(event.data));
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
        applyStorageQueryResult(decoded.data, setStorage);
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
      lastError,
      socketURL: () => daemonURL,
      send,
      refreshStorage
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
  setStorage: (next: StorageSnapshot[]) => void
): void {
  setStorage(normalizeStorageSnapshots(payload));
}

function buildStorageQuery(area?: QueryStorage["area"]): Envelope<QueryStorage> {
  const normalizedArea = normalizeArea(area);

  return {
    v: PROTOCOL_VERSION,
    t: "command",
    name: "query.storage",
    src: { role: "inspector", id: inspectorID },
    data: typeof normalizedArea === "string" ? { area: normalizedArea } : {}
  };
}

function resolveConnectionParams(): { wsURL: string; token: string } {
  const params = new URLSearchParams(window.location.search);
  return {
    wsURL: nonEmpty(params.get("ws"), "ws://127.0.0.1:4317/ws"),
    token: nonEmpty(params.get("token"), "dev-token")
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

function normalizeArea(area: QueryStorage["area"]): string | undefined {
  if (typeof area !== "string") {
    return undefined;
  }

  const trimmed = area.trim().toLowerCase();
  return trimmed.length > 0 ? trimmed : undefined;
}

import { decode, encode } from "@msgpack/msgpack";
import {
  PROTOCOL_VERSION,
  isEnvelope,
  isHelloAck,
  readWebSocketMessageData,
  type ChromeAPICall,
  type ChromeAPIEvent,
  type Envelope,
  type Hello,
  type StorageDiff
} from "@panex/protocol";
import { reconnectCeilingMS, reconnectDelay, reconnectFloorMS } from "./reconnect";
import { resolveDefaultExtensionID } from "./runtime";

export type TransportStatus = "connecting" | "open" | "reconnecting" | "closed";

export interface ChromeSimTransport {
  call(namespace: string, method: string, args?: unknown[]): Promise<unknown>;
  close(): void;
  status(): TransportStatus;
  subscribeEvents(handler: (event: Envelope<ChromeAPIEvent>) => void): () => void;
  subscribeStorageDiff(handler: (event: Envelope<StorageDiff>) => void): () => void;
}

export interface TransportSocket {
  readyState: number;
  binaryType?: string;
  addEventListener(type: "open" | "message" | "error" | "close", listener: (event: any) => void): void;
  send(data: Uint8Array): void;
  close(code?: number, reason?: string): void;
}

export type WebSocketFactory = (url: string) => TransportSocket;

export interface ChromeSimTransportOptions {
  daemonURL?: string;
  authToken?: string;
  extensionID?: string;
  callTimeoutMS?: number;
  handshakeTimeoutMS?: number;
  reconnectFloorMS?: number;
  reconnectCeilingMS?: number;
  webSocketFactory?: WebSocketFactory;
  callIDFactory?: () => string;
  clientID?: string;
}

interface PendingCall {
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
  timeout: ReturnType<typeof setTimeout>;
}

const socketOpenState = 1;
const defaultCallTimeoutMS = 5000;
const defaultHandshakeTimeoutMS = 5000;
const closeMessageTooBig = 1009;

export function createChromeSimTransport(options: ChromeSimTransportOptions = {}): ChromeSimTransport {
  const resolvedDaemonBaseURL =
    nonEmpty(options.daemonURL, resolveDefaultDaemonURL()) ?? resolveDefaultDaemonURL();
  const authToken = nonEmpty(options.authToken, resolveDefaultAuthToken());
  const extensionID = nonEmpty(options.extensionID, resolveDefaultExtensionID());
  const daemonURL = buildDaemonURL(resolvedDaemonBaseURL);
  const callTimeoutMS = normalizeTimeout(options.callTimeoutMS, defaultCallTimeoutMS);
  const handshakeTimeoutMS = normalizeTimeout(options.handshakeTimeoutMS, defaultHandshakeTimeoutMS);
  const floor = normalizeTimeout(options.reconnectFloorMS, reconnectFloorMS);
  const ceiling = normalizeTimeout(options.reconnectCeilingMS, reconnectCeilingMS);
  const webSocketFactory = options.webSocketFactory ?? defaultWebSocketFactory;
  const clientID = nonEmpty(options.clientID, `chrome-sim-${safeClientID()}`) ?? `chrome-sim-${safeClientID()}`;

  let state: TransportStatus = "closed";
  let closed = false;
  let callSeq = 0;
  let reconnectAttempt = 0;
  let socket: TransportSocket | null = null;
  let connectPromise: Promise<void> | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
  const pendingCalls = new Map<string, PendingCall>();
  const eventHandlers = new Set<(event: Envelope<ChromeAPIEvent>) => void>();
  const storageDiffHandlers = new Set<(event: Envelope<StorageDiff>) => void>();

  const setState = (next: TransportStatus) => {
    state = next;
  };

  const rejectPendingCalls = (reason: string) => {
    if (pendingCalls.size === 0) {
      return;
    }

    for (const [callID, pending] of pendingCalls.entries()) {
      clearTimeout(pending.timeout);
      pending.reject(new Error(`${reason} (call_id=${callID})`));
      pendingCalls.delete(callID);
    }
  };

  const scheduleReconnect = () => {
    if (closed || reconnectTimer) {
      return;
    }

    const delay = reconnectDelay(reconnectAttempt, floor, ceiling);
    reconnectAttempt += 1;
    setState("reconnecting");

    reconnectTimer = setTimeout(() => {
      reconnectTimer = undefined;
      void ensureConnected().catch(() => {
        // keep background reconnect best-effort; explicit calls will surface errors.
      });
    }, delay);
  };

  const handleChromeAPIResult = (payload: unknown) => {
    if (!isRecord(payload)) {
      return;
    }

    const callID = normalizeString(payload.call_id);
    if (!callID) {
      return;
    }

    const pending = pendingCalls.get(callID);
    if (!pending) {
      return;
    }

    clearTimeout(pending.timeout);
    pendingCalls.delete(callID);

    if (payload.success === true) {
      pending.resolve(payload.data);
      return;
    }

    const message = normalizeString(payload.error) ?? "chrome api call failed";
    pending.reject(new Error(message));
  };

  const handleSocketMessage = (event: any) => {
    const message = readWebSocketMessageData(event?.data);
    if (message.kind === "unsupported") {
      return;
    }
    if (message.kind === "too_large") {
      socket?.close(closeMessageTooBig, "message exceeds limit");
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

    if (decoded.name === "chrome.api.result" && decoded.t === "event") {
      handleChromeAPIResult(decoded.data);
      return;
    }

    if (decoded.name === "storage.diff" && decoded.t === "event") {
      for (const handler of storageDiffHandlers) {
        handler(decoded as Envelope<StorageDiff>);
      }
    }

    if (decoded.name === "chrome.api.event" && decoded.t === "event") {
      for (const handler of eventHandlers) {
        handler(decoded as Envelope<ChromeAPIEvent>);
      }
    }
  };

  const connect = () => {
    const nextSocket = webSocketFactory(daemonURL);
    nextSocket.binaryType = "arraybuffer";
    socket = nextSocket;
    setState(state === "reconnecting" ? "reconnecting" : "connecting");

    return new Promise<void>((resolve, reject) => {
      let settled = false;

      const resolveOnce = () => {
        if (settled) {
          return;
        }
        settled = true;
        resolve();
      };

      const rejectOnce = (reason: Error) => {
        if (settled) {
          return;
        }
        settled = true;
        reject(reason);
      };

      const handshakeTimer = setTimeout(() => {
        rejectOnce(new Error("chrome-sim transport handshake timed out"));
        nextSocket.close();
      }, handshakeTimeoutMS);

      nextSocket.addEventListener("open", () => {
        const hello: Envelope<Hello> = {
          v: PROTOCOL_VERSION,
          t: "lifecycle",
          name: "hello",
          src: { role: "inspector", id: clientID },
          data: {
            protocol_version: PROTOCOL_VERSION,
            auth_token: authToken,
            client_kind: "chrome-sim",
            client_version: "dev",
            extension_id: extensionID,
            capabilities_requested: ["chrome.api.call", "chrome.api.result", "chrome.api.event", "storage.diff"]
          }
        };

        nextSocket.send(encode(hello));
      });

      nextSocket.addEventListener("message", (event) => {
        const message = readWebSocketMessageData(event?.data);
        if (message.kind === "unsupported") {
          return;
        }
        if (message.kind === "too_large") {
          nextSocket.close(closeMessageTooBig, "message exceeds limit");
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
          clearTimeout(handshakeTimer);
          if (decoded.data.protocol_version !== PROTOCOL_VERSION) {
            rejectOnce(new Error("protocol version mismatch"));
            nextSocket.close();
            return;
          }

          if (!decoded.data.auth_ok) {
            rejectOnce(new Error("daemon rejected chrome-sim handshake"));
            nextSocket.close();
            return;
          }

          reconnectAttempt = 0;
          setState("open");
          resolveOnce();
          return;
        }

        handleSocketMessage({ data: message.bytes });
      });

      nextSocket.addEventListener("error", () => {
        // Close triggers reconnect + call rejection through the close handler.
        nextSocket.close();
      });

      nextSocket.addEventListener("close", () => {
        clearTimeout(handshakeTimer);
        if (socket === nextSocket) {
          socket = null;
        }

        if (!settled) {
          rejectOnce(new Error("chrome-sim websocket closed before hello.ack"));
        }

        if (!closed) {
          rejectPendingCalls("chrome-sim transport disconnected");
          scheduleReconnect();
        }
      });
    });
  };

  const ensureConnected = async () => {
    if (closed) {
      throw new Error("chrome-sim transport is closed");
    }

    if (socket && socket.readyState === socketOpenState && state === "open") {
      return;
    }

    if (!connectPromise) {
      const inFlight = connect();
      connectPromise = inFlight;
      try {
        await inFlight;
      } finally {
        if (connectPromise === inFlight) {
          connectPromise = null;
        }
      }
      return;
    }

    await connectPromise;
  };

  return {
    async call(namespace: string, method: string, args: unknown[] = []): Promise<unknown> {
      const normalizedNamespace = normalizeString(namespace);
      const normalizedMethod = normalizeString(method);
      if (!normalizedNamespace) {
        throw new Error("chrome api namespace is required");
      }
      if (!normalizedMethod) {
        throw new Error("chrome api method is required");
      }

      await ensureConnected();
      if (!socket || socket.readyState !== socketOpenState) {
        throw new Error("chrome-sim transport is not connected");
      }

      const callID = options.callIDFactory?.() ?? `call-${++callSeq}`;
      const envelope: Envelope<ChromeAPICall> = {
        v: PROTOCOL_VERSION,
        t: "command",
        name: "chrome.api.call",
        src: { role: "inspector", id: clientID },
        data: {
          call_id: callID,
          namespace: normalizedNamespace,
          method: normalizedMethod,
          args
        }
      };

      return await new Promise<unknown>((resolve, reject) => {
        const timeout = setTimeout(() => {
          pendingCalls.delete(callID);
          reject(new Error(`chrome.api.call timed out after ${callTimeoutMS}ms`));
        }, callTimeoutMS);

        pendingCalls.set(callID, { resolve, reject, timeout });

        try {
          socket?.send(encode(envelope));
        } catch (error) {
          clearTimeout(timeout);
          pendingCalls.delete(callID);
          reject(
            error instanceof Error
              ? error
              : new Error(`failed to send chrome.api.call: ${String(error)}`)
          );
        }
      });
    },

    close() {
      if (closed) {
        return;
      }
      closed = true;
      setState("closed");
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = undefined;
      }
      rejectPendingCalls("chrome-sim transport closed");
      socket?.close();
      socket = null;
    },

    status() {
      return state;
    },

    subscribeEvents(handler) {
      eventHandlers.add(handler);
      return () => {
        eventHandlers.delete(handler);
      };
    },

    subscribeStorageDiff(handler) {
      storageDiffHandlers.add(handler);
      return () => {
        storageDiffHandlers.delete(handler);
      };
    }
  };
}

function defaultWebSocketFactory(url: string): TransportSocket {
  if (typeof WebSocket === "undefined") {
    throw new Error("global WebSocket is unavailable; provide webSocketFactory");
  }
  return new WebSocket(url);
}

function isRecord(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null;
}

function normalizeString(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function normalizeTimeout(value: number | undefined, fallback: number): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return fallback;
  }
  return Math.round(value);
}

function nonEmpty(value: string | undefined, fallback: string | undefined): string | undefined {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (trimmed.length > 0) {
    return trimmed;
  }

  const fallbackTrimmed = typeof fallback === "string" ? fallback.trim() : "";
  return fallbackTrimmed.length > 0 ? fallbackTrimmed : undefined;
}

function buildDaemonURL(baseURL: string): string {
  const url = new URL(baseURL);
  url.searchParams.delete("token");
  return url.toString();
}

function resolveDefaultDaemonURL(): string {
  if (typeof window !== "undefined") {
    const candidate = (window as any).__PANEX_DAEMON_URL__;
    if (typeof candidate === "string" && candidate.trim().length > 0) {
      return candidate;
    }
  }
  return "ws://127.0.0.1:4317/ws";
}

function resolveDefaultAuthToken(): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const candidate = (window as any).__PANEX_DAEMON_TOKEN__;
  return typeof candidate === "string" && candidate.trim().length > 0 ? candidate : undefined;
}

function safeClientID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return String(Date.now());
}

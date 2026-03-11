import { decode, encode } from "@msgpack/msgpack";
import {
  type Envelope,
  readWebSocketMessageData,
  isHelloAck,
  isEnvelope
} from "@panex/protocol";

import { buildDaemonURL, loadConfig } from "./config";
import {
  createAgentDiagnostics,
  summarizeEnvelope,
  summarizeHelloAck
} from "./diagnostics";
import {
  buildHelloEnvelope,
  createAgentHandshakeState,
  handleDaemonEnvelope,
  requestedCapabilities,
  resetAgentHandshakeState
} from "./handshake";

const reconnectFloorMS = 500;
const reconnectCeilingMS = 5000;
const closeMessageTooBig = 1009;

let socket: WebSocket | null = null;
let reconnectAttempts = 0;
const handshakeState = createAgentHandshakeState();

void connect();

async function connect(): Promise<void> {
  const config = await loadConfig();
  const url = buildDaemonURL(config.wsUrl);
  const diagnostics = createAgentDiagnostics(config.diagnosticLogging);

  diagnostics.log("websocket.connecting", {
    attempt: reconnectAttempts + 1,
    url
  });

  const nextSocket = new WebSocket(url);
  nextSocket.binaryType = "arraybuffer";
  socket = nextSocket;

  nextSocket.addEventListener("open", () => {
    if (socket !== nextSocket) {
      return;
    }

    reconnectAttempts = 0;
    resetAgentHandshakeState(handshakeState, config.extensionId);

    diagnostics.log("websocket.open", { url });
    nextSocket.send(encode(buildHelloEnvelope(config)));
    diagnostics.log("websocket.hello_sent", {
      capabilitiesRequested: [...requestedCapabilities]
    });
  });

  nextSocket.addEventListener("message", (event) => {
    if (socket !== nextSocket) {
      return;
    }

    const message = readWebSocketMessageData(event.data);
    if (message.kind === "unsupported") {
      diagnostics.log("websocket.message_unsupported");
      return;
    }
    if (message.kind === "too_large") {
      diagnostics.log("websocket.message_too_large", { size: message.size });
      nextSocket.close(closeMessageTooBig, "message exceeds limit");
      return;
    }

    let decoded: unknown;
    try {
      decoded = decode(message.bytes);
    } catch {
      diagnostics.log("protocol.decode_failed");
      return;
    }
    if (!isEnvelope(decoded)) {
      diagnostics.log("protocol.invalid_envelope");
      return;
    }

    const result = handleDaemonEnvelope(decoded, handshakeState, {
      runtimeReload: () => chrome.runtime.reload(),
      closeSocket: (code?: number, reason?: string) => {
        diagnostics.log("websocket.close_requested", { code, reason });
        nextSocket.close(code, reason);
      }
    });

    switch (result) {
      case "hello_ack":
        if (isHelloAck(decoded)) {
          diagnostics.log("handshake.completed", {
            ...summarizeEnvelope(decoded),
            ...summarizeHelloAck(decoded.data)
          });
        }
        break;
      case "reload":
        diagnostics.log("command.reload", {
          ...summarizeEnvelope(decoded),
          reason: readReloadReason(decoded)
        });
        break;
      case "ignored":
        diagnostics.log("protocol.ignored", summarizeEnvelope(decoded));
        break;
      case "closed":
        break;
    }
  });

  nextSocket.addEventListener("close", (event) => {
    if (socket !== nextSocket) {
      return;
    }

    diagnostics.log("websocket.closed", {
      code: event.code,
      reason: event.reason,
      wasClean: event.wasClean
    });
    socket = null;
    resetAgentHandshakeState(handshakeState);
    scheduleReconnect(diagnostics);
  });

  nextSocket.addEventListener("error", () => {
    if (socket !== nextSocket) {
      return;
    }

    diagnostics.log("websocket.error");
    nextSocket.close();
  });
}

function scheduleReconnect(diagnostics: ReturnType<typeof createAgentDiagnostics>): void {
  reconnectAttempts += 1;
  const delay = Math.min(reconnectFloorMS * 2 ** (reconnectAttempts - 1), reconnectCeilingMS);

  diagnostics.log("websocket.reconnect_scheduled", {
    delayMS: delay,
    nextAttempt: reconnectAttempts + 1
  });

  // MV3 service workers are short-lived; bounded backoff avoids tight reconnect loops
  // while still recovering quickly after daemon restarts during local development.
  setTimeout(() => {
    void connect();
  }, delay);
}

function readReloadReason(envelope: Envelope): string | undefined {
  if (typeof envelope.data !== "object" || envelope.data === null) {
    return undefined;
  }

  const reason = (envelope.data as { reason?: unknown }).reason;
  return typeof reason === "string" ? reason : undefined;
}

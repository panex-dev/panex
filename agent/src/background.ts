import { decode, encode } from "@msgpack/msgpack";
import {
  readWebSocketMessageData,
  isEnvelope
} from "@panex/protocol";

import { buildDaemonURL, loadConfig } from "./config";
import {
  buildHelloEnvelope,
  createAgentHandshakeState,
  handleDaemonEnvelope,
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

  const nextSocket = new WebSocket(url);
  nextSocket.binaryType = "arraybuffer";
  socket = nextSocket;

  nextSocket.addEventListener("open", () => {
    if (socket !== nextSocket) {
      return;
    }

    reconnectAttempts = 0;
    resetAgentHandshakeState(handshakeState);

    nextSocket.send(encode(buildHelloEnvelope(config)));
  });

  nextSocket.addEventListener("message", (event) => {
    if (socket !== nextSocket) {
      return;
    }

    const message = readWebSocketMessageData(event.data);
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

    handleDaemonEnvelope(decoded, handshakeState, {
      runtimeReload: () => chrome.runtime.reload(),
      closeSocket: (code?: number, reason?: string) => {
        nextSocket.close(code, reason);
      }
    });
  });

  nextSocket.addEventListener("close", () => {
    if (socket !== nextSocket) {
      return;
    }

    socket = null;
    resetAgentHandshakeState(handshakeState);
    scheduleReconnect();
  });

  nextSocket.addEventListener("error", () => {
    if (socket !== nextSocket) {
      return;
    }

    nextSocket.close();
  });
}

function scheduleReconnect(): void {
  reconnectAttempts += 1;
  const delay = Math.min(reconnectFloorMS * 2 ** (reconnectAttempts - 1), reconnectCeilingMS);

  // MV3 service workers are short-lived; bounded backoff avoids tight reconnect loops
  // while still recovering quickly after daemon restarts during local development.
  setTimeout(() => {
    void connect();
  }, delay);
}

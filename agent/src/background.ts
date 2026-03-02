import { decode, encode } from "@msgpack/msgpack";
import { PROTOCOL_VERSION, isEnvelope, type Envelope, type Hello } from "../../shared/protocol/src/index";

import { buildDaemonURL, loadConfig } from "./config";
import { handleReloadCommand } from "./reload";

const reconnectFloorMS = 500;
const reconnectCeilingMS = 5000;

let socket: WebSocket | null = null;
let reconnectAttempts = 0;

void connect();

async function connect(): Promise<void> {
  const config = await loadConfig();
  const url = buildDaemonURL(config.wsUrl, config.token);

  socket = new WebSocket(url);
  socket.binaryType = "arraybuffer";

  socket.addEventListener("open", () => {
    reconnectAttempts = 0;

    const hello: Envelope<Hello> = {
      v: PROTOCOL_VERSION,
      t: "lifecycle",
      name: "hello",
      src: { role: "dev-agent", id: config.agentId },
      data: {
        protocol_version: PROTOCOL_VERSION,
        client_kind: "dev-agent",
        client_version: "dev",
        capabilities_requested: ["command.reload"]
      }
    };

    socket?.send(encode(hello));
  });

  socket.addEventListener("message", (event) => {
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

    handleReloadCommand(decoded, () => chrome.runtime.reload());
  });

  socket.addEventListener("close", () => {
    scheduleReconnect();
  });

  socket.addEventListener("error", () => {
    socket?.close();
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

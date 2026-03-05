import { createStorageArea } from "./storage";
import { createChromeSimTransport, type ChromeSimTransport, type ChromeSimTransportOptions } from "./transport";

export interface SimulatedChrome {
  storage: {
    local: ReturnType<typeof createStorageArea>;
    sync: ReturnType<typeof createStorageArea>;
    session: ReturnType<typeof createStorageArea>;
  };
}

export interface InstallChromeSimOptions extends ChromeSimTransportOptions {
  transport?: ChromeSimTransport;
}

export function installChromeSim(options: InstallChromeSimOptions = {}): ChromeSimTransport | null {
  if (typeof window === "undefined") {
    return null;
  }

  const transport =
    options.transport ??
    createChromeSimTransport({
      daemonURL: options.daemonURL,
      authToken: options.authToken,
      callTimeoutMS: options.callTimeoutMS,
      handshakeTimeoutMS: options.handshakeTimeoutMS,
      reconnectFloorMS: options.reconnectFloorMS,
      reconnectCeilingMS: options.reconnectCeilingMS,
      webSocketFactory: options.webSocketFactory,
      callIDFactory: options.callIDFactory,
      clientID: options.clientID
    });

  const existing = ((window as any).chrome ?? {}) as Record<string, unknown>;
  const currentStorage = isRecord(existing.storage) ? existing.storage : {};

  const simulatedStorage = {
    ...currentStorage,
    local: createStorageArea("local", transport),
    sync: createStorageArea("sync", transport),
    session: createStorageArea("session", transport)
  };

  (window as any).chrome = {
    ...existing,
    storage: simulatedStorage
  } as SimulatedChrome;

  return transport;
}

if (typeof window !== "undefined" && !(window as any).__PANEX_CHROME_SIM_DISABLE_AUTO_INSTALL__) {
  installChromeSim();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

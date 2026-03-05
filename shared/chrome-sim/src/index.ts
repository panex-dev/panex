import { createRuntimeNamespace } from "./runtime";
import { createStorageArea } from "./storage";
import { createChromeSimTransport, type ChromeSimTransport, type ChromeSimTransportOptions } from "./transport";

export interface SimulatedChrome {
  runtime: ReturnType<typeof createRuntimeNamespace>;
  storage: {
    local: ReturnType<typeof createStorageArea>;
    sync: ReturnType<typeof createStorageArea>;
    session: ReturnType<typeof createStorageArea>;
  };
}

export interface InstallChromeSimOptions extends ChromeSimTransportOptions {
  extensionID?: string;
  transport?: ChromeSimTransport;
}

export function installChromeSim(options: InstallChromeSimOptions = {}): ChromeSimTransport | null {
  if (typeof window === "undefined") {
    return null;
  }

  const resolved = resolveInstallOptions(options);

  const transport =
    resolved.transport ??
    createChromeSimTransport({
      daemonURL: resolved.daemonURL,
      authToken: resolved.authToken,
      callTimeoutMS: resolved.callTimeoutMS,
      handshakeTimeoutMS: resolved.handshakeTimeoutMS,
      reconnectFloorMS: resolved.reconnectFloorMS,
      reconnectCeilingMS: resolved.reconnectCeilingMS,
      webSocketFactory: resolved.webSocketFactory,
      callIDFactory: resolved.callIDFactory,
      clientID: resolved.clientID
    });

  const existing = ((window as any).chrome ?? {}) as Record<string, unknown>;
  const currentStorage = isRecord(existing.storage) ? existing.storage : {};
  const currentRuntime = isRecord(existing.runtime) ? existing.runtime : {};

  const simulatedStorage = {
    ...currentStorage,
    local: createStorageArea("local", transport),
    sync: createStorageArea("sync", transport),
    session: createStorageArea("session", transport)
  };
  const simulatedRuntime = {
    ...currentRuntime,
    ...createRuntimeNamespace(transport, resolved.extensionID)
  };

  (window as any).chrome = {
    ...existing,
    runtime: simulatedRuntime,
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

function resolveInstallOptions(options: InstallChromeSimOptions): InstallChromeSimOptions {
  const query = resolveWindowQueryOptions();

  return {
    ...options,
    daemonURL: nonEmpty(options.daemonURL, query.daemonURL),
    authToken: nonEmpty(options.authToken, query.authToken),
    extensionID: nonEmpty(options.extensionID, query.extensionID)
  };
}

function resolveWindowQueryOptions(): { daemonURL?: string; authToken?: string; extensionID?: string } {
  if (typeof window === "undefined") {
    return {};
  }

  const location = (window as any).location;
  const search = typeof location?.search === "string" ? location.search : "";
  const params = new URLSearchParams(search);

  return {
    daemonURL: nonEmpty(readParam(params, "ws"), readWindowString("__PANEX_DAEMON_URL__")),
    authToken: nonEmpty(readParam(params, "token"), readWindowString("__PANEX_DAEMON_TOKEN__")),
    extensionID: nonEmpty(readParam(params, "extension_id"), readWindowString("__PANEX_EXTENSION_ID__"))
  };
}

function readParam(params: URLSearchParams, name: string): string | undefined {
  return nonEmpty(params.get(name), undefined);
}

function readWindowString(key: string): string | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  return nonEmpty((window as any)[key], undefined);
}

function nonEmpty(value: unknown, fallback: string | undefined): string | undefined {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed.length > 0) {
      return trimmed;
    }
  }
  return fallback;
}

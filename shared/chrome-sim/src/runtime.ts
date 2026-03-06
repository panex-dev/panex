import type { Envelope } from "@panex/protocol";
import type { ChromeSimTransport } from "./transport";

export interface RuntimeOnMessageListener {
  (message: unknown): void;
}

export interface RuntimeNamespace {
  id: string;
  sendMessage(...args: unknown[]): Promise<unknown>;
  onMessage: {
    addListener(listener: RuntimeOnMessageListener): void;
    removeListener(listener: RuntimeOnMessageListener): void;
    hasListener(listener: RuntimeOnMessageListener): boolean;
  };
}

const defaultExtensionID = "panex.simulated.extension";

export function createRuntimeNamespace(
  transport: ChromeSimTransport,
  extensionID?: string
): RuntimeNamespace {
  const listeners = new Set<RuntimeOnMessageListener>();

  transport.subscribeEvents((event) => {
    const payload = extractRuntimeOnMessageEvent(event);
    if (!payload) {
      return;
    }

    for (const listener of listeners) {
      listener(payload.message);
    }
  });

  return {
    id: normalizeExtensionID(extensionID) ?? resolveDefaultExtensionID(),
    async sendMessage(...args: unknown[]): Promise<unknown> {
      if (args.length === 0) {
        throw new Error("runtime.sendMessage requires at least one argument");
      }
      return await transport.call("runtime", "sendMessage", args);
    },
    onMessage: {
      addListener(listener: RuntimeOnMessageListener) {
        listeners.add(listener);
      },
      removeListener(listener: RuntimeOnMessageListener) {
        listeners.delete(listener);
      },
      hasListener(listener: RuntimeOnMessageListener) {
        return listeners.has(listener);
      }
    }
  };
}

export function resolveDefaultExtensionID(): string {
  if (typeof window === "undefined") {
    return defaultExtensionID;
  }

  const candidate = (window as any).__PANEX_EXTENSION_ID__;
  return normalizeExtensionID(candidate) ?? defaultExtensionID;
}

function extractRuntimeOnMessageEvent(event: Envelope): { message: unknown } | null {
  if (event.name !== "chrome.api.event" || event.t !== "event") {
    return null;
  }

  if (!isRecord(event.data)) {
    return null;
  }

  if (event.data.namespace !== "runtime" || event.data.event !== "onMessage") {
    return null;
  }

  if (!Array.isArray(event.data.args) || event.data.args.length === 0) {
    return { message: undefined };
  }

  return { message: event.data.args[0] };
}

function normalizeExtensionID(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function isRecord(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null;
}

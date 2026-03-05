import type { ChromeSimTransport } from "./transport";

export interface SimulatedTab {
  id: number;
  windowId: number;
  active: boolean;
  currentWindow: boolean;
  url?: string;
  title?: string;
}

export interface TabsNamespace {
  query(queryInfo?: Record<string, unknown>): Promise<SimulatedTab[]>;
}

export function createTabsNamespace(transport: ChromeSimTransport): TabsNamespace {
  return {
    async query(queryInfo?: Record<string, unknown>): Promise<SimulatedTab[]> {
      const args = typeof queryInfo === "undefined" ? [] : [queryInfo];
      const result = await transport.call("tabs", "query", args);
      return asTabArray(result);
    }
  };
}

function asTabArray(value: unknown): SimulatedTab[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const tabs: SimulatedTab[] = [];
  for (const entry of value) {
    const normalized = asTab(entry);
    if (normalized) {
      tabs.push(normalized);
    }
  }
  return tabs;
}

function asTab(value: unknown): SimulatedTab | null {
  if (!isRecord(value)) {
    return null;
  }

  const id = asNumber(value.id);
  const windowID = asNumber(value.windowId);
  const active = asBool(value.active);
  const currentWindow = asBool(value.currentWindow);
  if (
    typeof id !== "number" ||
    typeof windowID !== "number" ||
    typeof active !== "boolean" ||
    typeof currentWindow !== "boolean"
  ) {
    return null;
  }

  return {
    id,
    windowId: windowID,
    active,
    currentWindow,
    url: asString(value.url),
    title: asString(value.title)
  };
}

function asNumber(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  return undefined;
}

function asBool(value: unknown): boolean | undefined {
  if (typeof value === "boolean") {
    return value;
  }
  return undefined;
}

function asString(value: unknown): string | undefined {
  if (typeof value === "string") {
    return value;
  }
  return undefined;
}

function isRecord(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null;
}

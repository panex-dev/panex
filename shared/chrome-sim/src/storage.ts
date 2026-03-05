import type { ChromeSimTransport } from "./transport";

export type StorageArea = "local" | "sync" | "session";
export type StorageSelection = string | string[] | Record<string, unknown> | null;

export interface StorageAreaAPI {
  get(selection?: StorageSelection): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string | string[]): Promise<void>;
  clear(): Promise<void>;
  getBytesInUse(selection?: StorageSelection): Promise<number>;
}

export function createStorageArea(area: StorageArea, transport: ChromeSimTransport): StorageAreaAPI {
  const namespace = `storage.${area}`;

  return {
    async get(selection?: StorageSelection): Promise<Record<string, unknown>> {
      const args = typeof selection === "undefined" ? [] : [selection];
      const result = await transport.call(namespace, "get", args);
      return asRecord(result);
    },

    async set(items: Record<string, unknown>): Promise<void> {
      await transport.call(namespace, "set", [items]);
    },

    async remove(keys: string | string[]): Promise<void> {
      await transport.call(namespace, "remove", [keys]);
    },

    async clear(): Promise<void> {
      await transport.call(namespace, "clear");
    },

    async getBytesInUse(selection?: StorageSelection): Promise<number> {
      const args = typeof selection === "undefined" ? [] : [selection];
      const result = await transport.call(namespace, "getBytesInUse", args);
      return asNumber(result);
    }
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return {};
  }
  return value as Record<string, unknown>;
}

function asNumber(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  throw new Error(`expected numeric getBytesInUse result, got ${String(value)}`);
}

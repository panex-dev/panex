import type { QueryStorageResult, StorageSnapshot } from "../../shared/protocol/src/index";

export type StorageArea = "local" | "sync" | "session";
export type StorageAreaFilter = "all" | StorageArea;

export interface StorageRow {
  area: string;
  key: string;
  valueText: string;
}

const storageAreaFilterSet = new Set<StorageAreaFilter>(["all", "local", "sync", "session"]);

export function isStorageAreaFilter(value: string): value is StorageAreaFilter {
  return storageAreaFilterSet.has(value as StorageAreaFilter);
}

export function normalizeStorageSnapshots(payload: QueryStorageResult): StorageSnapshot[] {
  if (!Array.isArray(payload.snapshots)) {
    return [];
  }

  const snapshots: StorageSnapshot[] = [];
  for (const snapshot of payload.snapshots) {
    if (!isRecord(snapshot)) {
      continue;
    }

    const rawArea = typeof snapshot.area === "string" ? snapshot.area.trim().toLowerCase() : "";
    if (rawArea.length === 0) {
      continue;
    }

    if (!isRecord(snapshot.items) || Array.isArray(snapshot.items)) {
      continue;
    }

    snapshots.push({
      area: rawArea,
      items: snapshot.items
    });
  }

  return snapshots;
}

export function flattenStorageSnapshots(
  snapshots: StorageSnapshot[],
  areaFilter: StorageAreaFilter
): StorageRow[] {
  const rows: StorageRow[] = [];
  for (const snapshot of snapshots) {
    if (areaFilter !== "all" && snapshot.area !== areaFilter) {
      continue;
    }

    const keys = Object.keys(snapshot.items).sort((left, right) => left.localeCompare(right));
    for (const key of keys) {
      rows.push({
        area: snapshot.area,
        key,
        valueText: formatStorageValue(snapshot.items[key])
      });
    }
  }

  return rows.sort((left, right) => {
    if (left.area === right.area) {
      return left.key.localeCompare(right.key);
    }
    return left.area.localeCompare(right.area);
  });
}

export function formatStorageValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean" || value === null) {
    return String(value);
  }
  if (typeof value === "undefined") {
    return "undefined";
  }

  try {
    const encoded = JSON.stringify(value);
    return typeof encoded === "string" ? encoded : String(value);
  } catch {
    return String(value);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

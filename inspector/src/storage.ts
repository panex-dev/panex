import type {
  QueryStorageResult,
  StorageDiff,
  StorageSnapshot
} from "../../shared/protocol/src/index";

export type StorageArea = "local" | "sync" | "session";
export type StorageAreaFilter = "all" | StorageArea;

export interface StorageRow {
  rowID: string;
  area: string;
  key: string;
  valueText: string;
}

export interface AppliedStorageDiff {
  snapshots: StorageSnapshot[];
  changedRowIDs: string[];
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
        rowID: storageRowID(snapshot.area, key),
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

export function applyStorageDiff(
  snapshots: StorageSnapshot[],
  payload: StorageDiff
): AppliedStorageDiff {
  if (!isRecord(payload)) {
    return { snapshots, changedRowIDs: [] };
  }

  const area = typeof payload.area === "string" ? payload.area.trim().toLowerCase() : "";
  if (area.length === 0 || !Array.isArray(payload.changes)) {
    return { snapshots, changedRowIDs: [] };
  }

  const changedRowIDs: string[] = [];
  const byArea = new Map<string, StorageSnapshot>();
  for (const snapshot of snapshots) {
    byArea.set(snapshot.area, {
      area: snapshot.area,
      items: { ...snapshot.items }
    });
  }

  const current = byArea.get(area) ?? { area, items: {} };
  byArea.set(area, current);

  for (const change of payload.changes) {
    if (!isRecord(change) || typeof change.key !== "string") {
      continue;
    }
    const key = change.key.trim();
    if (key.length === 0) {
      continue;
    }

    if (hasOwn(change, "new_value") && typeof change.new_value !== "undefined") {
      current.items[key] = change.new_value;
    } else {
      delete current.items[key];
    }

    changedRowIDs.push(storageRowID(area, key));
  }

  const nextSnapshots = Array.from(byArea.values()).sort((left, right) =>
    left.area.localeCompare(right.area)
  );

  return {
    snapshots: nextSnapshots,
    changedRowIDs: deduplicate(changedRowIDs)
  };
}

export function storageRowID(area: string, key: string): string {
  return `${area}:${key}`;
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

function hasOwn(record: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(record, key);
}

function deduplicate(values: string[]): string[] {
  const seen = new Set<string>();
  const deduped: string[] = [];
  for (const value of values) {
    if (seen.has(value)) {
      continue;
    }
    seen.add(value);
    deduped.push(value);
  }
  return deduped;
}

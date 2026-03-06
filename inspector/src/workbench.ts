import type { StorageSnapshot } from "@panex/protocol";

import type { ConnectionStatus } from "./connection";
import type { TimelineEntry } from "./timeline";

export interface WorkbenchStorageAreaSummary {
  area: string;
  keys: number;
}

export interface WorkbenchTimelineSummary {
  totalEvents: number;
  liveEvents: number;
  persistedEvents: number;
  latestEventName: string | null;
  latestEventSource: string | null;
}

export interface WorkbenchModel {
  status: ConnectionStatus;
  socketURL: string;
  lastError: string | null;
  totalStorageKeys: number;
  storageAreas: WorkbenchStorageAreaSummary[];
  timeline: WorkbenchTimelineSummary;
}

const preferredAreaOrder = new Map<string, number>([
  ["local", 0],
  ["sync", 1],
  ["session", 2]
]);

export function buildWorkbenchModel(args: {
  status: ConnectionStatus;
  socketURL: string;
  lastError: string | null;
  storage: StorageSnapshot[];
  timeline: TimelineEntry[];
}): WorkbenchModel {
  return {
    status: args.status,
    socketURL: args.socketURL,
    lastError: args.lastError,
    totalStorageKeys: countStorageKeys(args.storage),
    storageAreas: summarizeStorageAreas(args.storage),
    timeline: summarizeTimeline(args.timeline)
  };
}

export function summarizeStorageAreas(storage: StorageSnapshot[]): WorkbenchStorageAreaSummary[] {
  return storage
    .map((snapshot) => ({
      area: snapshot.area,
      keys: Object.keys(snapshot.items).length
    }))
    .sort((left, right) => compareAreaNames(left.area, right.area));
}

export function countStorageKeys(storage: StorageSnapshot[]): number {
  let total = 0;
  for (const snapshot of storage) {
    total += Object.keys(snapshot.items).length;
  }
  return total;
}

export function summarizeTimeline(timeline: TimelineEntry[]): WorkbenchTimelineSummary {
  let liveEvents = 0;
  let persistedEvents = 0;
  let latest: TimelineEntry | null = null;

  for (const entry of timeline) {
    if (typeof entry.id === "number") {
      persistedEvents += 1;
    } else {
      liveEvents += 1;
    }

    if (!latest || entry.recordedAtMS >= latest.recordedAtMS) {
      latest = entry;
    }
  }

  return {
    totalEvents: timeline.length,
    liveEvents,
    persistedEvents,
    latestEventName: latest ? latest.envelope.name : null,
    latestEventSource: latest ? `${latest.envelope.src.role}:${latest.envelope.src.id}` : null
  };
}

function compareAreaNames(left: string, right: string): number {
  const leftRank = preferredAreaOrder.get(left) ?? Number.MAX_SAFE_INTEGER;
  const rightRank = preferredAreaOrder.get(right) ?? Number.MAX_SAFE_INTEGER;
  if (leftRank !== rightRank) {
    return leftRank - rightRank;
  }
  return left.localeCompare(right);
}

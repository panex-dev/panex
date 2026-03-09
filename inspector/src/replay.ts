import { formatStorageValue } from "./storage";
import { decodeReplayObservation } from "./replay-contract";
import type { TimelineEntry } from "./timeline";

export interface ReplayHistoryEntry {
  key: string;
  recordedAtMS: number;
  payload: Record<string, unknown>;
  payloadText: string;
  sourceText: string;
  callID: string | null;
}

export function summarizeReplayHistory(timeline: TimelineEntry[]): ReplayHistoryEntry[] {
  const entries: ReplayHistoryEntry[] = [];

  for (let index = timeline.length - 1; index >= 0; index -= 1) {
    const entry = timeline[index];
    if (!entry) {
      continue;
    }

    const decoded = decodeReplayObservation(entry);
    if (!decoded) {
      continue;
    }

    entries.push({
      key: `${entry.key}-${decoded.observedAs}`,
      recordedAtMS: entry.recordedAtMS,
      payload: decoded.payload,
      payloadText: formatStorageValue(decoded.payload),
      sourceText: decoded.sourceText,
      callID: decoded.callID
    });
  }

  return entries;
}

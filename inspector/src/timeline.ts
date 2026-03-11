import type {
  Envelope,
  EnvelopeType,
  EventSnapshot,
  Source
} from "@panex/protocol";

export interface TimelineEntry {
  key: string;
  id?: number;
  recordedAtMS: number;
  envelope: Envelope;
}

export interface TimelineFilter {
  search: string;
  messageType: EnvelopeType | "all";
  sourceRole: Source["role"] | "all";
}

export type TimelineMergePosition = "append" | "prepend";

export interface QueryClause {
  key: "text" | "name" | "src" | "type";
  value: string;
}

export interface MergeEntriesResult {
  entries: TimelineEntry[];
  droppedOldest: number;
  droppedNewest: number;
}

export const defaultTimelineLimit = 500;
export const defaultTimelineRenderWindow = 200;
export const defaultTimelineWorkingSetLimit = defaultTimelineLimit * 4;
export const defaultTimelineFilter: TimelineFilter = {
  search: "",
  messageType: "all",
  sourceRole: "all"
};

let liveSeq = 0;

export function fromSnapshot(snapshot: EventSnapshot): TimelineEntry {
  return {
    key: `db-${snapshot.id}`,
    id: snapshot.id,
    recordedAtMS: snapshot.recorded_at_ms,
    envelope: snapshot.envelope
  };
}

export function fromLiveEnvelope(envelope: Envelope, recordedAtMS = Date.now()): TimelineEntry {
  liveSeq += 1;
  return {
    key: `live-${liveSeq}`,
    recordedAtMS,
    envelope
  };
}

export function mergeEntries(
  existing: TimelineEntry[],
  incoming: TimelineEntry[],
  maxEntries = defaultTimelineLimit,
  position: TimelineMergePosition = "append"
): TimelineEntry[] {
  return mergeEntriesWithOverflow(existing, incoming, maxEntries, position).entries;
}

export function mergeEntriesWithOverflow(
  existing: TimelineEntry[],
  incoming: TimelineEntry[],
  maxEntries = defaultTimelineLimit,
  position: TimelineMergePosition = "append"
): MergeEntriesResult {
  if (incoming.length === 0) {
    return {
      entries: existing,
      droppedOldest: 0,
      droppedNewest: 0
    };
  }

  const byID = new Set<number>();
  for (const entry of existing) {
    if (typeof entry.id === "number") {
      byID.add(entry.id);
    }
  }

  const nextIncoming: TimelineEntry[] = [];
  for (const entry of incoming) {
    if (typeof entry.id === "number" && byID.has(entry.id)) {
      continue;
    }
    if (typeof entry.id === "number") {
      byID.add(entry.id);
    }
    nextIncoming.push(entry);
  }

  const merged =
    position === "prepend" ? [...nextIncoming, ...existing] : [...existing, ...nextIncoming];

  if (merged.length <= maxEntries) {
    return {
      entries: merged,
      droppedOldest: 0,
      droppedNewest: 0
    };
  }

  const overflow = merged.length - maxEntries;
  if (position === "prepend") {
    return {
      entries: merged.slice(0, maxEntries),
      droppedOldest: 0,
      droppedNewest: overflow
    };
  }

  return {
    entries: merged.slice(overflow),
    droppedOldest: overflow,
    droppedNewest: 0
  };
}

export function oldestPersistedTimelineID(entries: TimelineEntry[]): number | null {
  for (const entry of entries) {
    if (typeof entry.id === "number") {
      return entry.id;
    }
  }

  return null;
}

export function renderTimelineWindow(
  entries: TimelineEntry[],
  visibleCount = defaultTimelineRenderWindow
): TimelineEntry[] {
  const boundedVisibleCount = boundVisibleCount(visibleCount);
  if (entries.length <= boundedVisibleCount) {
    return entries;
  }

  return entries.slice(entries.length - boundedVisibleCount);
}

export function hiddenOlderTimelineCount(
  entries: TimelineEntry[],
  visibleCount = defaultTimelineRenderWindow
): number {
  return Math.max(0, entries.length - boundVisibleCount(visibleCount));
}

export function summarizeEnvelope(envelope: Envelope): string {
  if (envelope.name === "build.complete" && typeof envelope.data === "object" && envelope.data !== null) {
    const payload = envelope.data as { build_id?: string; success?: boolean; extension_id?: string };
    const summary = `build=${payload.build_id ?? "unknown"} success=${String(payload.success ?? false)}`;
    if (typeof payload.extension_id === "string" && payload.extension_id.trim().length > 0) {
      return `${summary} ext=${payload.extension_id}`;
    }
    return summary;
  }
  if (envelope.name === "command.reload" && typeof envelope.data === "object" && envelope.data !== null) {
    const payload = envelope.data as { reason?: string; build_id?: string; extension_id?: string };
    const summary = `reason=${payload.reason ?? "unknown"} build=${payload.build_id ?? "n/a"}`;
    if (typeof payload.extension_id === "string" && payload.extension_id.trim().length > 0) {
      return `${summary} ext=${payload.extension_id}`;
    }
    return summary;
  }
  if (envelope.name === "query.events.result" && typeof envelope.data === "object" && envelope.data !== null) {
    const payload = envelope.data as { events?: unknown[] };
    return `events=${Array.isArray(payload.events) ? payload.events.length : 0}`;
  }

  return "payload captured";
}

export function formatTime(msEpoch: number): string {
  return new Date(msEpoch).toLocaleTimeString();
}

export function filterEntries(entries: TimelineEntry[], filter: TimelineFilter): TimelineEntry[] {
  const clauses = parseSearchQuery(filter.search);

  return entries.filter((entry) => {
    if (filter.messageType !== "all" && entry.envelope.t !== filter.messageType) {
      return false;
    }
    if (filter.sourceRole !== "all" && entry.envelope.src.role !== filter.sourceRole) {
      return false;
    }
    if (clauses.length === 0) {
      return true;
    }

    let indexedText = "";
    const source = `${entry.envelope.src.role}:${entry.envelope.src.id}`.toLowerCase();

    for (const clause of clauses) {
      switch (clause.key) {
        case "name":
          if (!entry.envelope.name.toLowerCase().includes(clause.value)) {
            return false;
          }
          break;
        case "src":
          if (!source.includes(clause.value)) {
            return false;
          }
          break;
        case "type":
          if (!entry.envelope.t.toLowerCase().startsWith(clause.value)) {
            return false;
          }
          break;
        default:
          if (indexedText.length === 0) {
            indexedText = [
              entry.envelope.name,
              entry.envelope.t,
              entry.envelope.src.role,
              entry.envelope.src.id,
              summarizeEnvelope(entry.envelope)
            ]
              .join(" ")
              .toLowerCase();
          }
          if (!indexedText.includes(clause.value)) {
            return false;
          }
      }
    }

    return true;
  });
}

export function parseSearchQuery(search: string): QueryClause[] {
  // Tiny query DSL for fast triage: name:<msg> src:<role|id> type:<kind> + free text tokens.
  const rawTokens = search
    .trim()
    .split(/\s+/)
    .map((token) => token.trim())
    .filter((token) => token.length > 0);

  const clauses: QueryClause[] = [];
  for (const rawToken of rawTokens) {
    const token = rawToken.toLowerCase();
    const splitIndex = token.indexOf(":");
    if (splitIndex > 0) {
      const key = token.slice(0, splitIndex);
      const value = token.slice(splitIndex + 1);
      if (value.length === 0) {
        continue;
      }

      if (key === "name" || key === "src" || key === "type") {
        clauses.push({ key, value });
        continue;
      }
    }

    clauses.push({ key: "text", value: token });
  }

  return clauses;
}

function boundVisibleCount(visibleCount: number): number {
  if (!Number.isFinite(visibleCount) || visibleCount <= 0) {
    return defaultTimelineRenderWindow;
  }

  return Math.floor(visibleCount);
}

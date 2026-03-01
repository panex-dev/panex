import type {
  Envelope,
  EnvelopeType,
  EventSnapshot,
  Source
} from "../../shared/protocol/src/index";

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

export interface QueryClause {
  key: "text" | "name" | "src" | "type";
  value: string;
}

export const defaultTimelineLimit = 500;
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
  maxEntries = defaultTimelineLimit
): TimelineEntry[] {
  if (incoming.length === 0) {
    return existing;
  }

  const byID = new Set<number>();
  for (const entry of existing) {
    if (typeof entry.id === "number") {
      byID.add(entry.id);
    }
  }

  const merged = [...existing];
  for (const entry of incoming) {
    if (typeof entry.id === "number" && byID.has(entry.id)) {
      continue;
    }
    if (typeof entry.id === "number") {
      byID.add(entry.id);
    }
    merged.push(entry);
  }

  if (merged.length <= maxEntries) {
    return merged;
  }

  return merged.slice(merged.length - maxEntries);
}

export function summarizeEnvelope(envelope: Envelope): string {
  if (envelope.name === "build.complete" && typeof envelope.data === "object" && envelope.data !== null) {
    const payload = envelope.data as { build_id?: string; success?: boolean };
    return `build=${payload.build_id ?? "unknown"} success=${String(payload.success ?? false)}`;
  }
  if (envelope.name === "command.reload" && typeof envelope.data === "object" && envelope.data !== null) {
    const payload = envelope.data as { reason?: string; build_id?: string };
    return `reason=${payload.reason ?? "unknown"} build=${payload.build_id ?? "n/a"}`;
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

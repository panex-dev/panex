import type { Envelope, EventSnapshot } from "./protocol";

export interface TimelineEntry {
  key: string;
  id?: number;
  recordedAtMS: number;
  envelope: Envelope;
}

export const defaultTimelineLimit = 500;

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

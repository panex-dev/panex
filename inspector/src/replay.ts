import { formatStorageValue } from "./storage";
import type { TimelineEntry } from "./timeline";

export interface ReplayHistoryEntry {
  key: string;
  recordedAtMS: number;
  payload: Record<string, unknown>;
  payloadText: string;
  sourceText: string;
  callID: string | null;
}

const runtimeProbeKind = "panex.workbench.runtime-probe";
const runtimeProbeID = "runtime-ping";

export function summarizeReplayHistory(timeline: TimelineEntry[]): ReplayHistoryEntry[] {
  const entries: ReplayHistoryEntry[] = [];

  for (let index = timeline.length - 1; index >= 0; index -= 1) {
    const entry = timeline[index];
    if (!entry) {
      continue;
    }

    const decoded = decodeReplayEntry(entry);
    if (!decoded) {
      continue;
    }

    entries.push({
      key: `${entry.key}-${decoded.kind}`,
      recordedAtMS: entry.recordedAtMS,
      payload: decoded.payload,
      payloadText: formatStorageValue(decoded.payload),
      sourceText: decoded.sourceText,
      callID: decoded.callID
    });
  }

  return entries;
}

function decodeReplayEntry(
  entry: TimelineEntry
): { kind: "result" | "event"; payload: Record<string, unknown>; sourceText: string; callID: string | null } | null {
  const envelope = entry.envelope;

  if (envelope.name === "chrome.api.result" && envelope.t === "event" && isRecord(envelope.data)) {
    const payload = envelope.data.data;
    if (!matchesRuntimeProbePayload(payload)) {
      return null;
    }

    return {
      kind: "result",
      payload: { ...payload },
      sourceText: "runtime result payload",
      callID: typeof envelope.data.call_id === "string" ? envelope.data.call_id : null
    };
  }

  if (envelope.name === "chrome.api.event" && envelope.t === "event" && isRecord(envelope.data)) {
    if (envelope.data.namespace !== "runtime" || envelope.data.event !== "onMessage") {
      return null;
    }
    if (!Array.isArray(envelope.data.args) || envelope.data.args.length === 0) {
      return null;
    }

    const payload = envelope.data.args[0];
    if (!matchesRuntimeProbePayload(payload)) {
      return null;
    }

    return {
      kind: "event",
      payload: { ...payload },
      sourceText: "runtime.onMessage payload",
      callID: null
    };
  }

  return null;
}

function matchesRuntimeProbePayload(value: unknown): value is Record<string, unknown> {
  if (!isRecord(value)) {
    return false;
  }

  return value.kind === runtimeProbeKind && value.probe_id === runtimeProbeID;
}

function isRecord(value: unknown): value is Record<string, any> {
  return typeof value === "object" && value !== null;
}

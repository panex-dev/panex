import type { ChromeAPICall, ChromeAPIEvent, ChromeAPIResult } from "@panex/protocol";

import { formatStorageValue } from "./storage";
import type { TimelineEntry } from "./timeline";

export interface ChromeAPIActivityEntry {
  key: string;
  recordedAtMS: number;
  title: string;
  status: "success" | "failure" | "unsupported" | "event" | "pending";
  detail: string;
  payloadText: string;
  latencyMS: number | null;
  callID: string | null;
}

const defaultActivityLimit = 8;

export function summarizeChromeAPIActivity(
  timeline: TimelineEntry[],
  limit = defaultActivityLimit
): ChromeAPIActivityEntry[] {
  const callsByID = new Map<string, { entry: TimelineEntry; call: ChromeAPICall }>();

  for (const entry of timeline) {
    const call = decodeChromeAPICall(entry);
    if (!call) {
      continue;
    }

    callsByID.set(call.call.call_id, { entry, call: call.call });
  }

  const entries: ChromeAPIActivityEntry[] = [];
  const consumedCalls = new Set<string>();

  for (const entry of timeline) {
    const event = decodeChromeAPIEvent(entry);
    if (event) {
      entries.push({
        key: `${entry.key}-event`,
        recordedAtMS: entry.recordedAtMS,
        title: `${event.event.namespace}.${event.event.event}`,
        status: "event",
        detail: "observed simulator event",
        payloadText: formatStorageValue(event.event.args ?? []),
        latencyMS: null,
        callID: null
      });
      continue;
    }

    const result = decodeChromeAPIResult(entry);
    if (result) {
      const pairedCall = callsByID.get(result.result.call_id) ?? null;
      if (pairedCall) {
        consumedCalls.add(result.result.call_id);
      }
      entries.push(
        summarizeResultEntry(entry, result.result, pairedCall?.entry.recordedAtMS ?? null, pairedCall?.call ?? null)
      );
      continue;
    }
  }

  for (const entry of timeline) {
    const call = decodeChromeAPICall(entry);
    if (!call || consumedCalls.has(call.call.call_id)) {
      continue;
    }

    entries.push({
      key: `${entry.key}-call`,
      recordedAtMS: entry.recordedAtMS,
      title: `${call.call.namespace}.${call.call.method}`,
      status: "pending",
      detail: "awaiting simulator result",
      payloadText: formatStorageValue(call.call.args ?? []),
      latencyMS: null,
      callID: call.call.call_id
    });
  }

  return entries.sort((left, right) => right.recordedAtMS - left.recordedAtMS).slice(0, limit);
}

function summarizeResultEntry(
  entry: TimelineEntry,
  result: ChromeAPIResult,
  callRecordedAtMS: number | null,
  call: ChromeAPICall | null
): ChromeAPIActivityEntry {
  const unsupported = !result.success && typeof result.error === "string" && result.error.startsWith("unsupported ");
  const status: ChromeAPIActivityEntry["status"] = result.success
    ? "success"
    : unsupported
      ? "unsupported"
      : "failure";

  const title = call ? `${call.namespace}.${call.method}` : `result ${result.call_id}`;
  const payload =
    typeof result.data !== "undefined"
      ? result.data
      : typeof result.error === "string"
        ? { error: result.error }
        : null;

  return {
    key: `${entry.key}-result`,
    recordedAtMS: entry.recordedAtMS,
    title,
    status,
    detail: result.success
      ? "simulator result"
      : unsupported
        ? "unsupported simulator call"
        : "failed simulator call",
    payloadText: payload === null ? "[]" : formatStorageValue(payload),
    latencyMS: typeof callRecordedAtMS === "number" ? Math.max(entry.recordedAtMS - callRecordedAtMS, 0) : null,
    callID: result.call_id
  };
}

function decodeChromeAPICall(entry: TimelineEntry): { call: ChromeAPICall } | null {
  const envelope = entry.envelope;
  if (envelope.name !== "chrome.api.call" || envelope.t !== "command" || !isRecord(envelope.data)) {
    return null;
  }

  if (
    typeof envelope.data.call_id !== "string" ||
    typeof envelope.data.namespace !== "string" ||
    typeof envelope.data.method !== "string"
  ) {
    return null;
  }

  return {
    call: {
      call_id: envelope.data.call_id,
      namespace: envelope.data.namespace,
      method: envelope.data.method,
      args: Array.isArray(envelope.data.args) ? envelope.data.args : undefined
    }
  };
}

function decodeChromeAPIResult(entry: TimelineEntry): { result: ChromeAPIResult } | null {
  const envelope = entry.envelope;
  if (envelope.name !== "chrome.api.result" || envelope.t !== "event" || !isRecord(envelope.data)) {
    return null;
  }

  if (typeof envelope.data.call_id !== "string" || typeof envelope.data.success !== "boolean") {
    return null;
  }

  return {
    result: {
      call_id: envelope.data.call_id,
      success: envelope.data.success,
      data: envelope.data.data,
      error: typeof envelope.data.error === "string" ? envelope.data.error : undefined
    }
  };
}

function decodeChromeAPIEvent(entry: TimelineEntry): { event: ChromeAPIEvent } | null {
  const envelope = entry.envelope;
  if (envelope.name !== "chrome.api.event" || envelope.t !== "event" || !isRecord(envelope.data)) {
    return null;
  }

  if (typeof envelope.data.namespace !== "string" || typeof envelope.data.event !== "string") {
    return null;
  }

  return {
    event: {
      namespace: envelope.data.namespace,
      event: envelope.data.event,
      args: Array.isArray(envelope.data.args) ? envelope.data.args : undefined
    }
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

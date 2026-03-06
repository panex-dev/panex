import type {
  ChromeAPIResult,
  StorageSnapshot
} from "@panex/protocol";

import type { ConnectionStatus } from "./connection";
import { formatStorageValue, type StorageArea } from "./storage";
import type { TimelineEntry } from "./timeline";

export interface WorkbenchStorageAreaSummary {
  area: string;
  keys: number;
}

export interface WorkbenchStoragePresetDefinition {
  id: string;
  label: string;
  description: string;
  area: StorageArea;
  key: string;
  value: unknown;
}

export type WorkbenchStoragePresetState = "missing" | "customized" | "applied";

export interface WorkbenchStoragePresetSummary extends WorkbenchStoragePresetDefinition {
  state: WorkbenchStoragePresetState;
  actionLabel: "apply" | "update" | "remove";
  currentValueText: string | null;
}

export interface WorkbenchRuntimeProbeDefinition {
  id: string;
  label: string;
  description: string;
  payload: Record<string, unknown>;
}

export interface WorkbenchRuntimeProbeSummary extends WorkbenchRuntimeProbeDefinition {
  payloadText: string;
  lastResultText: string | null;
  lastEventText: string | null;
  lastActivityAtMS: number | null;
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
  storagePresets: WorkbenchStoragePresetSummary[];
  runtimeProbe: WorkbenchRuntimeProbeSummary;
  timeline: WorkbenchTimelineSummary;
}

const preferredAreaOrder = new Map<string, number>([
  ["local", 0],
  ["sync", 1],
  ["session", 2]
]);
const missingStorageValue = Symbol("missing-storage-value");

const workbenchStoragePresets: readonly WorkbenchStoragePresetDefinition[] = [
  {
    id: "local-feature-flag",
    label: "Seed local feature flag",
    description: "Writes a reversible demo flag under panex.workbench.* in local storage.",
    area: "local",
    key: "panex.workbench.featureFlag",
    value: {
      enabled: true,
      rollout: 1,
      source: "workbench"
    }
  },
  {
    id: "sync-layout-profile",
    label: "Seed sync layout profile",
    description: "Writes a named workspace profile under panex.workbench.* in sync storage.",
    area: "sync",
    key: "panex.workbench.layoutProfile",
    value: {
      density: "compact",
      theme: "sunrise"
    }
  },
  {
    id: "session-query-draft",
    label: "Seed session query draft",
    description: "Writes an ephemeral query draft under panex.workbench.* in session storage.",
    area: "session",
    key: "panex.workbench.queryDraft",
    value: {
      pinnedTab: "workbench",
      query: "name:storage.diff"
    }
  }
] as const;

const runtimeProbeDefinition: WorkbenchRuntimeProbeDefinition = {
  id: "runtime-ping",
  label: "Send runtime ping",
  description: "Dispatches a namespaced runtime.sendMessage probe and watches the echoed result + onMessage event.",
  payload: {
    kind: "panex.workbench.runtime-probe",
    probe_id: "runtime-ping",
    topic: "ping",
    source: "workbench"
  }
};

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
    storagePresets: summarizeStoragePresets(args.storage),
    runtimeProbe: summarizeRuntimeProbe(args.timeline),
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

export function summarizeStoragePresets(
  storage: StorageSnapshot[]
): WorkbenchStoragePresetSummary[] {
  return workbenchStoragePresets.map((preset) => {
    const currentValue = findStorageValue(storage, preset.area, preset.key);
    const state = summarizePresetState(currentValue, preset.value);

    return {
      ...preset,
      state,
      actionLabel: state === "applied" ? "remove" : state === "customized" ? "update" : "apply",
      currentValueText:
        currentValue === missingStorageValue ? null : formatStorageValue(currentValue)
    };
  });
}

export function summarizeRuntimeProbe(
  timeline: TimelineEntry[]
): WorkbenchRuntimeProbeSummary {
  const lastResult = findLatestRuntimeProbeResult(timeline);
  const lastEvent = findLatestRuntimeProbeEvent(timeline);
  const lastActivityAtMS =
    typeof lastResult?.recordedAtMS === "number" && typeof lastEvent?.recordedAtMS === "number"
      ? Math.max(lastResult.recordedAtMS, lastEvent.recordedAtMS)
      : lastResult?.recordedAtMS ?? lastEvent?.recordedAtMS ?? null;

  return {
    ...runtimeProbeDefinition,
    payloadText: formatStorageValue(runtimeProbeDefinition.payload),
    lastResultText: lastResult ? formatRuntimeResult(lastResult.result) : null,
    lastEventText: lastEvent ? formatStorageValue(lastEvent.message) : null,
    lastActivityAtMS
  };
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

function findStorageValue(
  storage: StorageSnapshot[],
  area: StorageArea,
  key: string
): unknown | typeof missingStorageValue {
  for (const snapshot of storage) {
    if (snapshot.area !== area) {
      continue;
    }

    if (Object.prototype.hasOwnProperty.call(snapshot.items, key)) {
      return snapshot.items[key];
    }
  }

  return missingStorageValue;
}

function summarizePresetState(
  currentValue: unknown | typeof missingStorageValue,
  expectedValue: unknown
): WorkbenchStoragePresetState {
  if (currentValue === missingStorageValue) {
    return "missing";
  }

  return stableValueText(currentValue) === stableValueText(expectedValue)
    ? "applied"
    : "customized";
}

function stableValueText(value: unknown, seen = new WeakSet<object>()): string {
  if (value === null) {
    return "null";
  }

  switch (typeof value) {
    case "string":
      return JSON.stringify(value);
    case "number":
    case "boolean":
      return String(value);
    case "undefined":
      return "undefined";
    case "bigint":
      return `${value}n`;
    case "function":
    case "symbol":
      return String(value);
    default:
      break;
  }

  if (Array.isArray(value)) {
    return `[${value.map((entry) => stableValueText(entry, seen)).join(",")}]`;
  }

  if (typeof value !== "object") {
    return String(value);
  }

  if (seen.has(value)) {
    return '"[circular]"';
  }

  seen.add(value);
  const record = value as Record<string, unknown>;
  const serialized = `{${Object.keys(record)
    .sort((left, right) => left.localeCompare(right))
    .map((key) => `${JSON.stringify(key)}:${stableValueText(record[key], seen)}`)
    .join(",")}}`;
  seen.delete(value);
  return serialized;
}

function findLatestRuntimeProbeResult(
  timeline: TimelineEntry[]
): { recordedAtMS: number; result: ChromeAPIResult } | null {
  for (let index = timeline.length - 1; index >= 0; index -= 1) {
    const entry = timeline[index];
    if (!entry) {
      continue;
    }

    const result = decodeRuntimeProbeResult(entry);
    if (result) {
      return { recordedAtMS: entry.recordedAtMS, result };
    }
  }

  return null;
}

function findLatestRuntimeProbeEvent(
  timeline: TimelineEntry[]
): { recordedAtMS: number; message: unknown } | null {
  for (let index = timeline.length - 1; index >= 0; index -= 1) {
    const entry = timeline[index];
    if (!entry) {
      continue;
    }

    const message = decodeRuntimeProbeEvent(entry);
    if (typeof message !== "undefined") {
      return { recordedAtMS: entry.recordedAtMS, message };
    }
  }

  return null;
}

function decodeRuntimeProbeResult(entry: TimelineEntry): ChromeAPIResult | null {
  const envelope = entry.envelope;
  if (envelope.name !== "chrome.api.result" || envelope.t !== "event" || !isRecord(envelope.data)) {
    return null;
  }

  const success = typeof envelope.data.success === "boolean" ? envelope.data.success : null;
  if (success === null) {
    return null;
  }

  if (!matchesRuntimeProbePayload(envelope.data.data)) {
    return null;
  }

  return {
    call_id: typeof envelope.data.call_id === "string" ? envelope.data.call_id : "",
    success,
    data: envelope.data.data,
    error: typeof envelope.data.error === "string" ? envelope.data.error : undefined
  };
}

function decodeRuntimeProbeEvent(entry: TimelineEntry): unknown {
  const envelope = entry.envelope;
  if (envelope.name !== "chrome.api.event" || envelope.t !== "event" || !isRecord(envelope.data)) {
    return undefined;
  }

  if (envelope.data.namespace !== "runtime" || envelope.data.event !== "onMessage") {
    return undefined;
  }

  if (!Array.isArray(envelope.data.args) || envelope.data.args.length === 0) {
    return undefined;
  }

  const message = envelope.data.args[0];
  return matchesRuntimeProbePayload(message) ? message : undefined;
}

function matchesRuntimeProbePayload(value: unknown): value is Record<string, unknown> {
  if (!isRecord(value)) {
    return false;
  }

  return (
    value.kind === runtimeProbeDefinition.payload.kind &&
    value.probe_id === runtimeProbeDefinition.payload.probe_id
  );
}

function formatRuntimeResult(result: ChromeAPIResult): string {
  if (!result.success) {
    return result.error ? `failure: ${result.error}` : "failure";
  }

  return result.data ? `success: ${formatStorageValue(result.data)}` : "success";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

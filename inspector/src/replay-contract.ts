import type { TimelineEntry } from "./timeline";

export type ReplayFamilyID = "runtime-probe";

export interface ReplayFamilyContract {
  id: ReplayFamilyID;
  label: string;
  payloadLabel: string;
}

export interface ReplayObservation {
  familyID: ReplayFamilyID;
  familyLabel: string;
  observedAs: "result" | "event";
  payload: Record<string, unknown>;
  sourceText: string;
  latestSourceText: string;
  callID: string | null;
}

const runtimeProbeContract = {
  id: "runtime-probe",
  label: "runtime probe",
  payloadLabel: "runtime probe payloads"
} as const satisfies ReplayFamilyContract;

const runtimeProbePayload = {
  kind: "panex.workbench.runtime-probe",
  probeID: "runtime-ping"
} as const;

export const replayFamilies = [runtimeProbeContract] as const;

export function findLatestReplayObservation(timeline: TimelineEntry[]): ReplayObservation | null {
  for (let index = timeline.length - 1; index >= 0; index -= 1) {
    const entry = timeline[index];
    if (!entry) {
      continue;
    }

    const observation = decodeReplayObservation(entry);
    if (observation) {
      return observation;
    }
  }

  return null;
}

export function decodeReplayObservation(entry: TimelineEntry): ReplayObservation | null {
  const envelope = entry.envelope;

  if (envelope.name === "chrome.api.result" && envelope.t === "event" && isRecord(envelope.data)) {
    const payload = envelope.data.data;
    if (!isReplayPayload(payload)) {
      return null;
    }

    return {
      familyID: runtimeProbeContract.id,
      familyLabel: runtimeProbeContract.label,
      observedAs: "result",
      payload: { ...payload },
      sourceText: "runtime result payload",
      latestSourceText: "latest runtime result payload",
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
    if (!isReplayPayload(payload)) {
      return null;
    }

    return {
      familyID: runtimeProbeContract.id,
      familyLabel: runtimeProbeContract.label,
      observedAs: "event",
      payload: { ...payload },
      sourceText: "runtime.onMessage payload",
      latestSourceText: "latest runtime.onMessage payload",
      callID: null
    };
  }

  return null;
}

export function isReplayPayload(value: unknown): value is Record<string, unknown> {
  if (!isRecord(value)) {
    return false;
  }

  return value.kind === runtimeProbePayload.kind && value.probe_id === runtimeProbePayload.probeID;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

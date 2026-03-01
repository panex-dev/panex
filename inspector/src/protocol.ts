export const PROTOCOL_VERSION = 1;

export type EnvelopeType = "lifecycle" | "event" | "command";

export type EnvelopeName =
  | "hello"
  | "welcome"
  | "build.complete"
  | "context.log"
  | "command.reload"
  | "query.events"
  | "query.events.result";

export interface Source {
  role: "daemon" | "dev-agent" | "inspector";
  id: string;
}

export interface Envelope<TData = unknown> {
  v: number;
  t: EnvelopeType;
  name: EnvelopeName;
  src: Source;
  data: TData;
}

export interface Hello {
  protocol_version: number;
  capabilities?: string[];
}

export interface QueryEvents {
  limit?: number;
}

export interface EventSnapshot {
  id: number;
  recorded_at_ms: number;
  envelope: Envelope;
}

export interface QueryEventsResult {
  events: EventSnapshot[];
}

export function isEnvelope(value: unknown): value is Envelope {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const candidate = value as Partial<Envelope>;
  return (
    typeof candidate.v === "number" &&
    typeof candidate.t === "string" &&
    typeof candidate.name === "string" &&
    typeof candidate.src === "object" &&
    candidate.src !== null &&
    typeof (candidate.src as Source).role === "string" &&
    typeof (candidate.src as Source).id === "string"
  );
}

export function isQueryEventsResult(envelope: Envelope): envelope is Envelope<QueryEventsResult> {
  return envelope.t === "event" && envelope.name === "query.events.result";
}

export function isWelcome(envelope: Envelope): boolean {
  return envelope.t === "lifecycle" && envelope.name === "welcome";
}

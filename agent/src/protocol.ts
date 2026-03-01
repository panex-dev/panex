export const PROTOCOL_VERSION = 1;

export type EnvelopeType = "lifecycle" | "event" | "command";

export type EnvelopeName =
  | "hello"
  | "welcome"
  | "build.complete"
  | "context.log"
  | "command.reload";

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

export interface CommandReload {
  reason?: string;
  build_id?: string;
}

export function isEnvelope(value: unknown): value is Envelope {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  // Validate only envelope shape at transport boundary; per-message payload
  // validation belongs to specific handlers to keep protocol evolution flexible.
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

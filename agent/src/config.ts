export interface AgentConfig {
  wsUrl: string;
  token: string;
  agentId: string;
  extensionId: string;
  diagnosticLogging: boolean;
}

const loopbackHosts = new Set(["127.0.0.1", "localhost"]);

export const defaultConfig: AgentConfig = {
  wsUrl: "ws://127.0.0.1:4317/ws",
  token: "dev-token",
  agentId: "dev-agent-1",
  extensionId: "default",
  diagnosticLogging: false
};

export async function loadConfig(): Promise<AgentConfig> {
  const raw = await chrome.storage.local.get(["panex"]);
  const value = raw.panex as Partial<AgentConfig> | undefined;

  return {
    wsUrl: normalizeDaemonWebSocketURL(value?.wsUrl, defaultConfig.wsUrl),
    token: nonEmpty(value?.token, defaultConfig.token),
    agentId: nonEmpty(value?.agentId, defaultConfig.agentId),
    extensionId: nonEmpty(value?.extensionId, defaultConfig.extensionId),
    diagnosticLogging: normalizeBoolean(value?.diagnosticLogging)
  };
}

export function buildDaemonURL(wsUrl: string): string {
  const url = new URL(wsUrl);
  url.searchParams.delete("token");
  return url.toString();
}

function nonEmpty(value: string | undefined, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

function normalizeBoolean(value: unknown): boolean {
  return value === true;
}

function normalizeDaemonWebSocketURL(value: string | undefined, fallback: string): string {
  const trimmed = nonEmpty(value, "");
  if (trimmed === "") {
    return fallback;
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return fallback;
  }

  if (
    parsed.protocol !== "ws:" ||
    !loopbackHosts.has(parsed.hostname) ||
    parsed.pathname !== "/ws" ||
    parsed.username !== "" ||
    parsed.password !== ""
  ) {
    return fallback;
  }

  parsed.hash = "";
  parsed.searchParams.delete("token");
  return parsed.toString();
}

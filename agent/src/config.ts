export interface AgentConfig {
  wsUrl: string;
  token: string;
  agentId: string;
}

const loopbackHosts = new Set(["127.0.0.1", "localhost"]);

export const defaultConfig: AgentConfig = {
  wsUrl: "ws://127.0.0.1:4317/ws",
  token: "dev-token",
  agentId: "dev-agent-1"
};

export async function loadConfig(): Promise<AgentConfig> {
  const raw = await chrome.storage.local.get(["panex"]);
  const value = raw.panex as Partial<AgentConfig> | undefined;

  return {
    wsUrl: normalizeDaemonWebSocketURL(value?.wsUrl, defaultConfig.wsUrl),
    token: nonEmpty(value?.token, defaultConfig.token),
    agentId: nonEmpty(value?.agentId, defaultConfig.agentId)
  };
}

export function buildDaemonURL(wsUrl: string, token: string): string {
  const url = new URL(wsUrl);
  // Keep auth in query params for parity with the daemon's current handshake gate.
  url.searchParams.set("token", token);
  return url.toString();
}

function nonEmpty(value: string | undefined, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
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
  return parsed.toString();
}

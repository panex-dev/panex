export interface AgentConfig {
  wsUrl: string;
  token: string;
  agentId: string;
}

export const defaultConfig: AgentConfig = {
  wsUrl: "ws://localhost:4317/ws",
  token: "dev-token",
  agentId: "dev-agent-1"
};

export async function loadConfig(): Promise<AgentConfig> {
  const raw = await chrome.storage.local.get(["panex"]);
  const value = raw.panex as Partial<AgentConfig> | undefined;

  return {
    wsUrl: nonEmpty(value?.wsUrl, defaultConfig.wsUrl),
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

import { buildDaemonURL, nonEmpty, normalizeDaemonWebSocketURL } from "@panex/protocol";

export { buildDaemonURL };

export interface AgentConfig {
  wsUrl: string;
  token: string;
  agentId: string;
  extensionId: string;
  diagnosticLogging: boolean;
}

export const defaultConfig: AgentConfig = {
  wsUrl: "ws://127.0.0.1:4317/ws",
  token: "",
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

function normalizeBoolean(value: unknown): boolean {
  return value === true;
}

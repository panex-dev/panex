import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";

import { buildDaemonURL, defaultConfig, loadConfig } from "../src/config";

const previousChrome = (globalThis as Record<string, unknown>).chrome;

afterEach(() => {
  if (previousChrome === undefined) {
    delete (globalThis as Record<string, unknown>).chrome;
    return;
  }

  (globalThis as Record<string, unknown>).chrome = previousChrome;
});

function setChromeStorage(value: unknown): void {
  (globalThis as Record<string, unknown>).chrome = {
    storage: {
      local: {
        get: async () => ({ panex: value })
      }
    }
  };
}

describe("config loading", () => {
  it("uses 127.0.0.1 for the default daemon websocket URL", () => {
    assert.equal(defaultConfig.wsUrl, "ws://127.0.0.1:4317/ws");
  });

  it("falls back to defaults when storage is empty or invalid", async () => {
    setChromeStorage({
      wsUrl: " ",
      token: "",
      agentId: 42,
      diagnosticLogging: "yes"
    });

    const config = await loadConfig();

    assert.deepEqual(config, defaultConfig);
  });

  it("uses stored values when they are non-empty strings", async () => {
    setChromeStorage({
      wsUrl: "ws://127.0.0.1:9999/ws",
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: true
    });

    const config = await loadConfig();

    assert.deepEqual(config, {
      wsUrl: "ws://127.0.0.1:9999/ws",
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: true
    });
  });

  it("accepts loopback localhost overrides on the daemon port contract", async () => {
    setChromeStorage({
      wsUrl: "ws://localhost:4317/ws?client=agent&token=leak",
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: false
    });

    const config = await loadConfig();

    assert.deepEqual(config, {
      wsUrl: "ws://localhost:4317/ws?client=agent",
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: false
    });
  });

  it("falls back when the stored websocket URL points off loopback or to the wrong path", async () => {
    setChromeStorage({
      wsUrl: "ws://evil.example:4317/not-ws",
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: true
    });

    const config = await loadConfig();

    assert.deepEqual(config, {
      wsUrl: defaultConfig.wsUrl,
      token: "secret",
      agentId: "agent-a",
      diagnosticLogging: true
    });
  });

  it("enables diagnostic logging only for an explicit boolean true", async () => {
    setChromeStorage({
      diagnosticLogging: true
    });

    assert.equal((await loadConfig()).diagnosticLogging, true);

    setChromeStorage({
      diagnosticLogging: "true"
    });

    assert.equal((await loadConfig()).diagnosticLogging, false);
  });
});

describe("daemon URL construction", () => {
  it("strips token query parameters and preserves unrelated params", () => {
    const url = buildDaemonURL("ws://127.0.0.1:4317/ws?foo=1&token=old");
    const parsed = new URL(url);

    assert.equal(parsed.searchParams.get("foo"), "1");
    assert.equal(parsed.searchParams.get("token"), null);
  });
});

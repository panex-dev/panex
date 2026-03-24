import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { createTabsNamespace } from "../src/tabs";
import type { ChromeSimTransport } from "../src/transport";

describe("tabs namespace", () => {
  it("routes tabs.query through transport and normalizes returned tabs", async () => {
    const calls: Array<{ namespace: string; method: string; args: unknown[] | undefined }> = [];
    const transport: ChromeSimTransport = {
      call(namespace, method, args) {
        calls.push({ namespace, method, args });
        return Promise.resolve([
          {
            id: 1,
            windowId: 1,
            active: true,
            currentWindow: true,
            url: "https://example.com",
            title: "Example"
          }
        ]);
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      },
      subscribeStorageDiff() {
        return () => {};
      }
    };

    const tabs = createTabsNamespace(transport);
    const result = await tabs.query({ active: true });
    assert.deepEqual(result, [
      {
        id: 1,
        windowId: 1,
        active: true,
        currentWindow: true,
        url: "https://example.com",
        title: "Example"
      }
    ]);
    assert.deepEqual(calls, [{ namespace: "tabs", method: "query", args: [{ active: true }] }]);
  });

  it("returns empty list when payload is not a tab array", async () => {
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve({ bad: true });
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      },
      subscribeStorageDiff() {
        return () => {};
      }
    };

    const tabs = createTabsNamespace(transport);
    assert.deepEqual(await tabs.query(), []);
  });
});

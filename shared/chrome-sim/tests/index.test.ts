import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { installChromeSim } from "../src/index";
import type { ChromeSimTransport } from "../src/transport";

describe("installChromeSim", () => {
  it("returns null when window is unavailable", () => {
    assert.equal(installChromeSim(), null);
  });

  it("installs storage namespaces on window.chrome", () => {
    const originalWindow = (globalThis as any).window;
    const fakeWindow: Record<string, unknown> = {};
    (globalThis as any).window = fakeWindow;

    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve({});
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      }
    };

    try {
      const installed = installChromeSim({ transport });
      assert.equal(installed, transport);

      const chrome = (fakeWindow as any).chrome;
      assert.equal(typeof chrome.storage.local.get, "function");
      assert.equal(typeof chrome.storage.sync.get, "function");
      assert.equal(typeof chrome.storage.session.get, "function");
    } finally {
      (globalThis as any).window = originalWindow;
    }
  });
});

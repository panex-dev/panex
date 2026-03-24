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
    const fakeWindow: Record<string, unknown> = {
      location: { search: "?extension_id=ext-query" }
    };
    (globalThis as any).window = fakeWindow;

    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve({});
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

    try {
      const installed = installChromeSim({ transport });
      assert.equal(installed, transport);

      const chrome = (fakeWindow as any).chrome;
      assert.equal(typeof chrome.storage.local.get, "function");
      assert.equal(typeof chrome.storage.sync.get, "function");
      assert.equal(typeof chrome.storage.session.get, "function");
      assert.equal(typeof chrome.storage.onChanged.addListener, "function");
      assert.equal(typeof chrome.runtime.sendMessage, "function");
      assert.equal(typeof chrome.tabs.query, "function");
      assert.equal(chrome.runtime.id, "ext-query");
    } finally {
      (globalThis as any).window = originalWindow;
    }
  });

  it("prefers explicit extensionID over query bootstrap value", () => {
    const originalWindow = (globalThis as any).window;
    const fakeWindow: Record<string, unknown> = {
      location: { search: "?extension_id=ext-query" }
    };
    (globalThis as any).window = fakeWindow;

    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve({});
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

    try {
      installChromeSim({ transport, extensionID: "ext-explicit" });
      const chrome = (fakeWindow as any).chrome;
      assert.equal(chrome.runtime.id, "ext-explicit");
      assert.equal(typeof chrome.tabs.query, "function");
    } finally {
      (globalThis as any).window = originalWindow;
    }
  });

  it("uses script bootstrap extension_id when query is absent", () => {
    const originalWindow = (globalThis as any).window;
    const fakeWindow: Record<string, unknown> = {
      location: { search: "" },
      document: {
        createElement() {
          return {
            type: "",
            src: "",
            dataset: {}
          };
        },
        currentScript: {
          type: "module",
          src: "/chrome-sim.js",
          dataset: {
            panexChromeSim: "1",
            panexExtensionId: "ext-from-script"
          }
        }
      }
    };
    (globalThis as any).window = fakeWindow;

    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve({});
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

    try {
      installChromeSim({ transport });
      const chrome = (fakeWindow as any).chrome;
      assert.equal(chrome.runtime.id, "ext-from-script");
    } finally {
      (globalThis as any).window = originalWindow;
    }
  });
});

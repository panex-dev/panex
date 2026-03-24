import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { injectChromeSimEntrypoint, resolveChromeSimBootstrapValues } from "../src/bootstrap";

describe("chrome-sim bootstrap helpers", () => {
  it("resolves query params first, then script dataset, then global values", () => {
    const values = resolveChromeSimBootstrapValues({
      location: {
        search: "?ws=ws://query/ws&token=query-token"
      },
      __PANEX_DAEMON_URL__: "ws://global/ws",
      __PANEX_DAEMON_TOKEN__: "global-token",
      __PANEX_EXTENSION_ID__: "global-ext",
      document: {
        createElement() {
          throw new Error("not used");
        },
        currentScript: {
          type: "module",
          src: "/chrome-sim.js",
          dataset: {
            panexChromeSim: "1",
            panexWs: "ws://script/ws",
            panexToken: "script-token",
            panexExtensionId: "script-ext"
          }
        }
      }
    });

    assert.equal(values.daemonURL, "ws://query/ws");
    assert.equal(values.authToken, "query-token");
    assert.equal(values.extensionID, "script-ext");
  });

  it("does not read token from script dataset (only from globals and query params)", () => {
    const values = resolveChromeSimBootstrapValues({
      __PANEX_DAEMON_TOKEN__: "global-token",
      document: {
        createElement() {
          throw new Error("not used");
        },
        currentScript: {
          type: "module",
          src: "/chrome-sim.js",
          dataset: {
            panexChromeSim: "1",
            panexWs: "ws://script/ws",
            panexToken: "script-token-should-be-ignored",
            panexExtensionId: "script-ext"
          }
        }
      }
    });

    assert.equal(values.authToken, "global-token");
  });

  it("injects module script with bootstrap dataset", () => {
    const appended: Array<{ type: string; src: string; dataset: Record<string, string | undefined> }> = [];
    const fakeDocument = {
      createElement() {
        return {
          type: "",
          src: "",
          dataset: {}
        };
      },
      head: {
        appendChild(node: { type: string; src: string; dataset: Record<string, string | undefined> }) {
          appended.push(node);
        }
      }
    };

    const script = injectChromeSimEntrypoint(fakeDocument, {
      moduleURL: "/chrome-sim-entry.js",
      daemonURL: "ws://127.0.0.1:4317/ws",
      authToken: "dev-token",
      extensionID: "ext-123"
    });

    assert.equal(script?.type, "module");
    assert.equal(script?.src, "/chrome-sim-entry.js");
    assert.equal(script?.dataset.panexChromeSim, "1");
    assert.equal(script?.dataset.panexWs, "ws://127.0.0.1:4317/ws");
    assert.equal(script?.dataset.panexToken, undefined);
    assert.equal(script?.dataset.panexExtensionId, "ext-123");
    assert.equal(appended.length, 1);
  });
});

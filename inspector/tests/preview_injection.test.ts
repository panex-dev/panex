import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { DEFAULT_DAEMON_WEBSOCKET_URL } from "@panex/protocol";

import { chromeSimScriptTag, injectChromeSimIntoHTML } from "../scripts/preview_injection";

describe("preview html chrome-sim injection", () => {
  it("inserts a chrome-sim module script before </head>", () => {
    const html = "<html><head><title>X</title></head><body></body></html>";

    const injected = injectChromeSimIntoHTML(html, {
      daemonURL: DEFAULT_DAEMON_WEBSOCKET_URL,
      authToken: "dev-token",
      extensionID: "ext-1",
      moduleURL: "./chrome-sim.js"
    });

    assert.match(injected, /data-panex-chrome-sim="1"/);
    assert.match(injected, /data-panex-ws="ws:\/\/127\.0\.0\.1:4317\/ws"/);
    assert.doesNotMatch(injected, /data-panex-token/);
    assert.match(injected, /data-panex-extension-id="ext-1"/);
    assert.match(injected, /src="\.\/chrome-sim\.js"/);
    assert.ok(injected.indexOf("data-panex-chrome-sim") < injected.toLowerCase().indexOf("</head>"));
  });

  it("does not duplicate when html already contains panex chrome-sim marker", () => {
    const html = "<html><head><script data-panex-chrome-sim=\"1\"></script></head><body></body></html>";

    const injected = injectChromeSimIntoHTML(html, {
      daemonURL: DEFAULT_DAEMON_WEBSOCKET_URL,
      moduleURL: "./chrome-sim.js"
    });

    assert.equal(injected, html);
  });

  it("fails when html has no closing head tag", () => {
    assert.throws(
      () =>
        injectChromeSimIntoHTML("<html><body></body></html>", {
          daemonURL: DEFAULT_DAEMON_WEBSOCKET_URL,
          moduleURL: "./chrome-sim.js"
        }),
      /missing a <\/head>/i
    );
  });

  it("renders a deterministic script tag with escaped attributes and no token", () => {
    const tag = chromeSimScriptTag({
      daemonURL: "ws://localhost:4317/ws?x=1&y=2",
      authToken: "\"quoted\"",
      moduleURL: "./chrome-sim.js"
    });

    assert.match(tag, /^<script /);
    assert.match(tag, /type="module"/);
    assert.doesNotMatch(tag, /data-panex-token/);
    assert.match(tag, /data-panex-ws="ws:\/\/localhost:4317\/ws\?x=1&amp;y=2"/);
  });
});

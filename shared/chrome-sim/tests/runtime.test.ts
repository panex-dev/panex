import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { Envelope } from "@panex/protocol";
import { createRuntimeNamespace } from "../src/runtime";
import type { ChromeSimTransport } from "../src/transport";

describe("runtime namespace", () => {
  it("routes sendMessage through transport runtime namespace", async () => {
    const calls: Array<{ namespace: string; method: string; args: unknown[] | undefined }> = [];
    const transport: ChromeSimTransport = {
      call(namespace, method, args) {
        calls.push({ namespace, method, args });
        return Promise.resolve({ ok: true });
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

    const runtime = createRuntimeNamespace(transport, "ext-1");
    const result = await runtime.sendMessage({ hello: "world" });
    assert.deepEqual(result, { ok: true });
    assert.equal(runtime.id, "ext-1");
    assert.deepEqual(calls, [
      {
        namespace: "runtime",
        method: "sendMessage",
        args: [{ hello: "world" }]
      }
    ]);
  });

  it("dispatches runtime.onMessage listeners from chrome.api.event payloads", () => {
    let eventHandler: ((event: Envelope) => void) | undefined;
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve(undefined);
      },
      close() {},
      status: () => "open",
      subscribeEvents(handler) {
        eventHandler = handler as (event: Envelope) => void;
        return () => {
          eventHandler = undefined;
        };
      },
      subscribeStorageDiff() {
        return () => {};
      }
    };

    const runtime = createRuntimeNamespace(transport, "ext-2");
    const received: unknown[] = [];
    const listener = (message: unknown) => {
      received.push(message);
    };

    runtime.onMessage.addListener(listener);
    assert.equal(runtime.onMessage.hasListener(listener), true);

    eventHandler?.({
      v: 1,
      t: "event",
      name: "chrome.api.event",
      src: { role: "daemon", id: "daemon-1" },
      data: {
        namespace: "runtime",
        event: "onMessage",
        args: [{ type: "ping" }]
      }
    });

    assert.deepEqual(received, [{ type: "ping" }]);

    runtime.onMessage.removeListener(listener);
    assert.equal(runtime.onMessage.hasListener(listener), false);
  });
});

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { createStorageArea } from "../src/storage";
import type { ChromeSimTransport } from "../src/transport";

describe("storage area adapter", () => {
  it("routes get/set/remove/clear/getBytesInUse through transport call", async () => {
    const calls: Array<{ namespace: string; method: string; args: unknown[] | undefined }> = [];

    const transport: ChromeSimTransport = {
      call(namespace, method, args) {
        calls.push({ namespace, method, args });
        if (method === "get") {
          return Promise.resolve({ feature: "on" });
        }
        if (method === "getBytesInUse") {
          return Promise.resolve(17);
        }
        return Promise.resolve(undefined);
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      }
    };

    const local = createStorageArea("local", transport);
    assert.deepEqual(await local.get(), { feature: "on" });
    assert.deepEqual(await local.get(["feature"]), { feature: "on" });
    await local.set({ feature: "on" });
    await local.remove(["feature"]);
    await local.clear();
    assert.equal(await local.getBytesInUse(["feature"]), 17);

    assert.deepEqual(calls, [
      { namespace: "storage.local", method: "get", args: [] },
      { namespace: "storage.local", method: "get", args: [["feature"]] },
      { namespace: "storage.local", method: "set", args: [{ feature: "on" }] },
      { namespace: "storage.local", method: "remove", args: [["feature"]] },
      { namespace: "storage.local", method: "clear", args: undefined },
      { namespace: "storage.local", method: "getBytesInUse", args: [["feature"]] }
    ]);
  });

  it("normalizes non-object get results to empty records", async () => {
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve("not-an-object");
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      }
    };

    const local = createStorageArea("local", transport);
    assert.deepEqual(await local.get(), {});
  });

  it("fails getBytesInUse for non-numeric results", async () => {
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve("NaN");
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      }
    };

    const local = createStorageArea("local", transport);
    await assert.rejects(async () => local.getBytesInUse(), /expected numeric getBytesInUse result/);
  });
});

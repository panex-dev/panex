import assert from "node:assert/strict";
import { describe, it } from "node:test";

import type { Envelope, StorageDiff } from "@panex/protocol";
import { createStorageArea, createStorageOnChanged } from "../src/storage";
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
      },
      subscribeStorageDiff() {
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
      },
      subscribeStorageDiff() {
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
      },
      subscribeStorageDiff() {
        return () => {};
      }
    };

    const local = createStorageArea("local", transport);
    await assert.rejects(async () => local.getBytesInUse(), /expected numeric getBytesInUse result/);
  });
});

describe("storage onChanged", () => {
  it("fires listeners when storage.diff arrives via transport", () => {
    let diffHandler: ((event: Envelope<StorageDiff>) => void) | undefined;
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve(undefined);
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      },
      subscribeStorageDiff(handler) {
        diffHandler = handler;
        return () => {
          diffHandler = undefined;
        };
      }
    };

    const onChanged = createStorageOnChanged(transport);
    const received: Array<{ changes: Record<string, unknown>; area: string }> = [];
    const listener = (changes: Record<string, unknown>, area: string) => {
      received.push({ changes, area });
    };

    onChanged.addListener(listener);
    assert.equal(onChanged.hasListener(listener), true);

    diffHandler?.({
      v: 1,
      t: "event",
      name: "storage.diff",
      src: { role: "daemon", id: "daemon-1" },
      data: {
        area: "local",
        changes: [
          { key: "theme", old_value: "light", new_value: "dark" },
          { key: "count", new_value: 1 }
        ]
      }
    });

    assert.equal(received.length, 1);
    assert.equal(received[0].area, "local");
    assert.deepEqual(received[0].changes, {
      theme: { oldValue: "light", newValue: "dark" },
      count: { newValue: 1 }
    });

    onChanged.removeListener(listener);
    assert.equal(onChanged.hasListener(listener), false);
  });

  it("does not fire listeners for empty change arrays", () => {
    let diffHandler: ((event: Envelope<StorageDiff>) => void) | undefined;
    const transport: ChromeSimTransport = {
      call() {
        return Promise.resolve(undefined);
      },
      close() {},
      status: () => "open",
      subscribeEvents() {
        return () => {};
      },
      subscribeStorageDiff(handler) {
        diffHandler = handler;
        return () => {
          diffHandler = undefined;
        };
      }
    };

    const onChanged = createStorageOnChanged(transport);
    let called = false;
    onChanged.addListener(() => {
      called = true;
    });

    diffHandler?.({
      v: 1,
      t: "event",
      name: "storage.diff",
      src: { role: "daemon", id: "daemon-1" },
      data: {
        area: "local",
        changes: []
      }
    });

    assert.equal(called, false);
  });
});

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  MAX_WEBSOCKET_MESSAGE_BYTES,
  readWebSocketMessageData
} from "../src/index";

describe("readWebSocketMessageData", () => {
  it("returns bytes for supported websocket payloads within the limit", () => {
    const raw = new Uint8Array([1, 2, 3]).buffer;

    const result = readWebSocketMessageData(raw);

    assert.equal(result.kind, "bytes");
    if (result.kind !== "bytes") {
      return;
    }
    assert.deepEqual(Array.from(result.bytes), [1, 2, 3]);
  });

  it("accepts array buffer views", () => {
    const raw = new Uint16Array([1, 2, 3]);

    const result = readWebSocketMessageData(raw);

    assert.equal(result.kind, "bytes");
    if (result.kind !== "bytes") {
      return;
    }
    assert.equal(result.bytes.byteLength, raw.byteLength);
  });

  it("reports oversized payloads", () => {
    const raw = new Uint8Array(MAX_WEBSOCKET_MESSAGE_BYTES + 1);

    const result = readWebSocketMessageData(raw);

    assert.deepEqual(result, {
      kind: "too_large",
      size: MAX_WEBSOCKET_MESSAGE_BYTES + 1
    });
  });

  it("rejects unsupported websocket payload shapes", () => {
    assert.deepEqual(readWebSocketMessageData("nope"), { kind: "unsupported" });
  });
});

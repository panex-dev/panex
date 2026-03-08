import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { resolveConnectionParamsFromSearch } from "../src/connection";

describe("resolveConnectionParamsFromSearch", () => {
  it("uses loopback defaults when no overrides are present", () => {
    assert.deepEqual(resolveConnectionParamsFromSearch(""), {
      wsURL: "ws://127.0.0.1:4317/ws",
      token: "dev-token"
    });
  });

  it("accepts top-level loopback websocket overrides", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch("?ws=ws://localhost:9999/ws&token=secret"),
      {
        wsURL: "ws://localhost:9999/ws",
        token: "secret"
      }
    );
  });

  it("falls back when the websocket override leaves the daemon contract", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch("?ws=ws://evil.example:4317/not-ws&token=secret"),
      {
        wsURL: "ws://127.0.0.1:4317/ws",
        token: "secret"
      }
    );
  });

  it("ignores query-param overrides when the inspector is embedded", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws://localhost:9999/ws&token=secret",
        true
      ),
      {
        wsURL: "ws://127.0.0.1:4317/ws",
        token: "dev-token"
      }
    );
  });
});

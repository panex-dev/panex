import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { buildTimelineQuery, resolveConnectionParamsFromSearch } from "../src/connection";

describe("resolveConnectionParamsFromSearch", () => {
  it("uses loopback defaults when no overrides are present", () => {
    assert.deepEqual(resolveConnectionParamsFromSearch(""), {
      wsURL: "ws://127.0.0.1:4317/ws",
      token: ""
    });
  });

  it("accepts top-level loopback websocket overrides", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws%3A%2F%2Flocalhost%3A9999%2Fws%3Fclient%3Dinspector%26token%3Dleak&token=secret"
      ),
      {
        wsURL: "ws://localhost:9999/ws?client=inspector",
        token: "secret"
      }
    );
  });

  it("falls back when the websocket override leaves the daemon contract", () => {
    assert.deepEqual(
      resolveConnectionParamsFromSearch(
        "?ws=ws%3A%2F%2Fevil.example%3A4317%2Fnot-ws%3Ftoken%3Dleak&token=secret"
      ),
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
        token: ""
      }
    );
  });
});

describe("buildTimelineQuery", () => {
  it("builds the initial recent-history request without a cursor", () => {
    const query = buildTimelineQuery(500);
    assert.deepEqual(query, {
      v: 1,
      t: "command",
      name: "query.events",
      src: {
        role: "inspector",
        id: query.src.id
      },
      data: { limit: 500 }
    });
  });

  it("adds before_id when loading older history", () => {
    const query = buildTimelineQuery(250, 42);
    assert.equal(query.data.limit, 250);
    assert.equal(query.data.before_id, 42);
  });
});

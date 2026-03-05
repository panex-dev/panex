import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { reconnectCeilingMS, reconnectDelay, reconnectFloorMS } from "../src/reconnect";

describe("reconnectDelay", () => {
  it("starts at floor", () => {
    assert.equal(reconnectDelay(0), reconnectFloorMS);
  });

  it("grows exponentially and caps at ceiling", () => {
    assert.equal(reconnectDelay(1), reconnectFloorMS * 2);
    assert.equal(reconnectDelay(10), reconnectCeilingMS);
  });
});

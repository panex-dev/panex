import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { reconnectCeilingMS, reconnectDelay, reconnectFloorMS } from "../src/reconnect";

describe("reconnectDelay", () => {
  it("starts at floor delay", () => {
    assert.equal(reconnectDelay(0), reconnectFloorMS);
  });

  it("doubles with each attempt until ceiling", () => {
    assert.equal(reconnectDelay(1), reconnectFloorMS * 2);
    assert.equal(reconnectDelay(2), reconnectFloorMS * 4);
  });

  it("caps at ceiling", () => {
    assert.equal(reconnectDelay(10), reconnectCeilingMS);
  });

  it("normalizes negative attempt values", () => {
    assert.equal(reconnectDelay(-3), reconnectFloorMS);
  });
});

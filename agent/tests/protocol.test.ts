import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { isEnvelope } from "../src/protocol";

describe("isEnvelope", () => {
  it("accepts protocol-shaped envelopes", () => {
    const value = {
      v: 1,
      t: "command",
      name: "command.reload",
      src: { role: "daemon", id: "daemon-1" },
      data: { reason: "build complete" }
    };

    assert.equal(isEnvelope(value), true);
  });

  it("rejects invalid payloads", () => {
    assert.equal(isEnvelope(null), false);
    assert.equal(
      isEnvelope({
        v: 1,
        t: "command",
        name: "command.reload",
        src: { role: "daemon" },
        data: {}
      }),
      false
    );
  });
});

import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { buildTabAccessibility } from "../src/accessibility";

describe("buildTabAccessibility", () => {
  it("returns stable ids for the active tab and marks it selected", () => {
    assert.deepEqual(buildTabAccessibility("timeline", "timeline"), {
      tabID: "inspector-tab-timeline",
      panelID: "inspector-panel-timeline",
      selected: true
    });
  });

  it("marks inactive tabs as not selected while preserving tab/panel ids", () => {
    assert.deepEqual(buildTabAccessibility("timeline", "storage"), {
      tabID: "inspector-tab-storage",
      panelID: "inspector-panel-storage",
      selected: false
    });
  });
});

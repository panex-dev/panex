import type { InspectorTab } from "./router";

export interface TabAccessibility {
  tabID: string;
  panelID: string;
  selected: boolean;
}

export function buildTabAccessibility(activeTab: InspectorTab, tab: InspectorTab): TabAccessibility {
  return {
    tabID: `inspector-tab-${tab}`,
    panelID: `inspector-panel-${tab}`,
    selected: activeTab === tab
  };
}

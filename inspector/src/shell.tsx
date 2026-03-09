import h from "solid-js/h";
import type { Accessor, JSX } from "solid-js";

import { buildTabAccessibility } from "./accessibility";
import type { InspectorTab } from "./router";

export interface ShellTabSpec {
  id: InspectorTab;
  label: string;
  disabled?: boolean;
}

interface ShellProps {
  activeTab: Accessor<InspectorTab>;
  tabs: readonly ShellTabSpec[];
  onTabSelect: (tab: InspectorTab) => void;
  sidebar: () => JSX.Element;
  content: () => JSX.Element;
}

export function Shell(props: ShellProps): JSX.Element {
  return (
    <main class="layout shell-layout">
      <header class="topbar">
        <h1>Panex Inspector</h1>
      </header>

      <section class="shell-grid">
        <aside class="shell-sidebar">{props.sidebar()}</aside>

        <div class="shell-main">
          <nav class="tab-bar" role="tablist" aria-label="Inspector Tabs">
            {props.tabs.map((tab) => {
              const a11y = buildTabAccessibility(props.activeTab(), tab.id);

              return (
                <button
                  id={a11y.tabID}
                  type="button"
                  role="tab"
                  class={`tab-button${a11y.selected ? " active" : ""}`}
                  aria-selected={a11y.selected ? "true" : "false"}
                  aria-controls={a11y.panelID}
                  tabindex={a11y.selected ? 0 : -1}
                  disabled={tab.disabled ?? false}
                  onClick={() => {
                    if (!tab.disabled) {
                      props.onTabSelect(tab.id);
                    }
                  }}
                >
                  {tab.label}
                </button>
              );
            })}
          </nav>

          <section
            id={buildTabAccessibility(props.activeTab(), props.activeTab()).panelID}
            class="tab-content"
            role="tabpanel"
            aria-labelledby={buildTabAccessibility(props.activeTab(), props.activeTab()).tabID}
            tabindex={0}
          >
            {props.content()}
          </section>
        </div>
      </section>
    </main>
  );
}

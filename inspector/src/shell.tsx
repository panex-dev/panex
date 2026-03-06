import h from "solid-js/h";
import type { Accessor, JSX } from "solid-js";

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
          <nav class="tab-bar" aria-label="Inspector Tabs">
            {props.tabs.map((tab) => (
              <button
                type="button"
                class={`tab-button${props.activeTab() === tab.id ? " active" : ""}`}
                aria-current={props.activeTab() === tab.id ? "page" : "false"}
                disabled={tab.disabled ?? false}
                onClick={() => {
                  if (!tab.disabled) {
                    props.onTabSelect(tab.id);
                  }
                }}
              >
                {tab.label}
              </button>
            ))}
          </nav>

          <section class="tab-content">{props.content()}</section>
        </div>
      </section>
    </main>
  );
}

import html from "solid-js/html";
import type { Accessor } from "solid-js";

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
  sidebar: () => unknown;
  content: () => unknown;
}

export function Shell(props: ShellProps) {
  const tabButtons = () =>
    props.tabs.map((tab) =>
      html`<button
        type="button"
        class=${() => `tab-button${props.activeTab() === tab.id ? " active" : ""}`}
        aria-current=${() => (props.activeTab() === tab.id ? "page" : "false")}
        disabled=${tab.disabled ?? false}
        onClick=${() => {
          if (!tab.disabled) {
            props.onTabSelect(tab.id);
          }
        }}
      >
        ${tab.label}
      </button>`
    );

  return html`<main class="layout shell-layout">
    <header class="topbar">
      <h1>Panex Inspector</h1>
    </header>

    <section class="shell-grid">
      <aside class="shell-sidebar">${props.sidebar}</aside>

      <div class="shell-main">
        <nav class="tab-bar" aria-label="Inspector Tabs">${tabButtons}</nav>
        <section class="tab-content">${props.content}</section>
      </div>
    </section>
  </main>`;
}

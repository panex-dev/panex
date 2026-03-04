import html from "solid-js/html";
import { ErrorBoundary } from "solid-js";
import { render } from "solid-js/web";

import "./styles.css";

import { ConnectionProvider, useConnection } from "./connection";
import { createInspectorRouter, type InspectorTab } from "./router";
import { Sidebar } from "./sidebar";
import { Shell, type ShellTabSpec } from "./shell";
import { StorageTab } from "./tabs/storage";
import { TimelineTab } from "./tabs/timeline";

const appRoot = document.getElementById("app");
if (!appRoot) {
  throw new Error("missing app root");
}

const tabs: readonly ShellTabSpec[] = [
  { id: "timeline", label: "Timeline" },
  { id: "storage", label: "Storage" },
  { id: "workbench", label: "Workbench", disabled: true },
  { id: "replay", label: "Replay", disabled: true }
];

function InspectorApp() {
  const connection = useConnection();
  const router = createInspectorRouter("timeline");

  const renderContent = () => {
    const activeTab = router.activeTab();
    switch (activeTab) {
      case "timeline":
        return TimelineTab({ timeline: connection.timeline });
      case "storage":
        return StorageTab();
      case "workbench":
      case "replay":
        return disabledTab(activeTab);
      default:
        return TimelineTab({ timeline: connection.timeline });
    }
  };

  return Shell({
    activeTab: router.activeTab,
    tabs,
    onTabSelect: router.navigate,
    sidebar: () =>
      Sidebar({
        status: connection.status,
        socketURL: connection.socketURL,
        lastError: connection.lastError
      }),
    content: renderContent
  });
}

function disabledTab(tab: Exclude<InspectorTab, "timeline" | "storage">) {
  return html`<section class="panel placeholder-panel">
    <div class="panel-header">
      <h2>${tab}</h2>
      <p>disabled</p>
    </div>
    <div class="placeholder-body">
      <p>${tab} is planned but not enabled in this milestone.</p>
      <p>Use Timeline and Storage tabs for the current development loop.</p>
    </div>
  </section>`;
}

function renderFallback(error: unknown, reset: () => void) {
  return html`<main class="layout">
    <header class="topbar">
      <h1>Panex Inspector</h1>
      <p class="error">render failure: ${String(error)}</p>
      <button class="filter-reset" type="button" onClick=${reset}>retry</button>
    </header>
  </main>`;
}

render(
  () =>
    ErrorBoundary({
      fallback: renderFallback,
      get children() {
        return ConnectionProvider({
          get children() {
            return InspectorApp();
          }
        });
      }
    }),
  appRoot
);

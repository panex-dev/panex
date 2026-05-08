import h from "solid-js/h";
import { ErrorBoundary, type JSX } from "solid-js";
import { render } from "solid-js/web";

import "./styles.css";

import { ConnectionProvider, useConnection } from "./connection";
import { createInspectorRouter } from "./router";
import { Sidebar } from "./sidebar";
import { Shell, type ShellTabSpec } from "./shell";
import { ProbeHistoryTab } from "./tabs/probe-history";
import { StorageTab } from "./tabs/storage";
import { TimelineTab } from "./tabs/timeline";
import { WorkbenchTab } from "./tabs/workbench";

const appRoot = document.getElementById("app");
if (!appRoot) {
  throw new Error("missing app root");
}

const tabs: readonly ShellTabSpec[] = [
  { id: "timeline", label: "Timeline" },
  { id: "storage", label: "Storage" },
  { id: "workbench", label: "Workbench" },
  { id: "probe-history", label: "Probe History" }
];

function InspectorApp(): JSX.Element {
  const connection = useConnection();
  const router = createInspectorRouter("timeline");

  const renderContent = () => {
    const activeTab = router.activeTab();
    switch (activeTab) {
      case "timeline":
        return TimelineTab({
          timeline: connection.timeline,
          canLoadOlderTimeline: connection.canLoadOlderTimeline,
          loadingOlderTimeline: connection.loadingOlderTimeline,
          loadingLatestTimeline: connection.loadingLatestTimeline,
          trimmedOlderTimelineCount: connection.trimmedOlderTimelineCount,
          trimmedNewerTimelineCount: connection.trimmedNewerTimelineCount,
          loadOlderTimeline: connection.loadOlderTimeline,
          jumpToLatestTimeline: connection.jumpToLatestTimeline
        });
      case "storage":
        return StorageTab({
          status: connection.status,
          storage: connection.storage,
          storageHighlights: connection.storageHighlights,
          refreshStorage: connection.refreshStorage,
          setStorageItem: connection.setStorageItem,
          removeStorageItem: connection.removeStorageItem,
          clearStorageArea: connection.clearStorageArea
        });
      case "workbench":
        return WorkbenchTab({
          status: connection.status,
          bridgeSession: connection.bridgeSession,
          socketURL: connection.socketURL,
          lastError: connection.lastError,
          storage: connection.storage,
          timeline: connection.timeline,
          setStorageItem: connection.setStorageItem,
          removeStorageItem: connection.removeStorageItem,
          sendRuntimeMessage: connection.sendRuntimeMessage
        });
      case "probe-history":
        return ProbeHistoryTab({
          status: connection.status,
          socketURL: connection.socketURL,
          lastError: connection.lastError,
          timeline: connection.timeline,
          sendRuntimeMessage: connection.sendRuntimeMessage
        });
      default:
        return TimelineTab({
          timeline: connection.timeline,
          canLoadOlderTimeline: connection.canLoadOlderTimeline,
          loadingOlderTimeline: connection.loadingOlderTimeline,
          loadingLatestTimeline: connection.loadingLatestTimeline,
          trimmedOlderTimelineCount: connection.trimmedOlderTimelineCount,
          trimmedNewerTimelineCount: connection.trimmedNewerTimelineCount,
          loadOlderTimeline: connection.loadOlderTimeline,
          jumpToLatestTimeline: connection.jumpToLatestTimeline
        });
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

function renderFallback(error: unknown, reset: () => void) {
  return (
    <main class="layout">
      <header class="topbar">
        <h1>Panex Inspector</h1>
        <p class="error" role="alert">
          render failure: {String(error)}
        </p>
        <button class="filter-reset" type="button" onClick={reset}>
          retry
        </button>
      </header>
    </main>
  );
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

import { createSignal, onCleanup, type Accessor } from "solid-js";

export type InspectorTab = "timeline" | "storage" | "workbench" | "probe-history";

export interface InspectorRouter {
  activeTab: Accessor<InspectorTab>;
  navigate: (tab: InspectorTab) => void;
}

const knownTabs = new Set<InspectorTab>(["timeline", "storage", "workbench", "probe-history"]);

export function createInspectorRouter(defaultTab: InspectorTab = "timeline"): InspectorRouter {
  const [activeTab, setActiveTab] = createSignal<InspectorTab>(readHash(defaultTab));

  const onHashChange = () => {
    setActiveTab(readHash(defaultTab));
  };

  if (typeof window !== "undefined") {
    if (window.location.hash.length === 0) {
      window.location.hash = `#${defaultTab}`;
      setActiveTab(defaultTab);
    }
    window.addEventListener("hashchange", onHashChange);
  }

  onCleanup(() => {
    if (typeof window !== "undefined") {
      window.removeEventListener("hashchange", onHashChange);
    }
  });

  return {
    activeTab,
    navigate: (tab) => {
      if (typeof window === "undefined") {
        setActiveTab(tab);
        return;
      }

      if (window.location.hash !== `#${tab}`) {
        window.location.hash = `#${tab}`;
      }
      setActiveTab(tab);
    }
  };
}

function readHash(fallback: InspectorTab): InspectorTab {
  if (typeof window === "undefined") {
    return fallback;
  }

  const raw = window.location.hash.replace(/^#/, "").trim().toLowerCase();
  if (knownTabs.has(raw as InspectorTab)) {
    return raw as InspectorTab;
  }
  return fallback;
}

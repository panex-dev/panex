import { decode, encode } from "@msgpack/msgpack";
import {
  PROTOCOL_VERSION,
  isEnvelope,
  isQueryEventsResult,
  isWelcome,
  type Envelope,
  type Hello,
  type QueryEvents,
  type QueryEventsResult
} from "../../shared/protocol/src/index";
import html from "solid-js/html";
import { ErrorBoundary, createEffect, createMemo, createSignal, onCleanup } from "solid-js";
import { render } from "solid-js/web";

import "./styles.css";

import {
  defaultTimelineFilter,
  defaultTimelineLimit,
  filterEntries,
  formatTime,
  fromLiveEnvelope,
  fromSnapshot,
  mergeEntries,
  summarizeEnvelope,
  type TimelineEntry
} from "./timeline";
import { reconnectDelay } from "./reconnect";

const appRoot = document.getElementById("app");
if (!appRoot) {
  throw new Error("missing app root");
}

const inspectorID = `inspector-${safeClientID()}`;
const filterStorageKey = "panex.inspector.filters.v1";

function App() {
  const initialFilter = loadFilterPreferences();
  const [connection, setConnection] = createSignal("connecting");
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [lastError, setLastError] = createSignal<string | null>(null);
  const [search, setSearch] = createSignal(initialFilter.search);
  const [messageType, setMessageType] = createSignal(initialFilter.messageType);
  const [sourceRole, setSourceRole] = createSignal(initialFilter.sourceRole);

  let listRef: HTMLDivElement | undefined;
  let socket: WebSocket | null = null;
  let reconnectTimer: number | undefined;
  let reconnectAttempt = 0;
  let stopped = false;

  const filteredTimeline = createMemo(() =>
    filterEntries(timeline(), {
      search: search(),
      messageType: messageType(),
      sourceRole: sourceRole()
    })
  );

  createEffect(() => {
    // Keep the list tail visible while streaming live events.
    if (!listRef) {
      return;
    }
    filteredTimeline().length;
    queueMicrotask(() => {
      if (listRef) {
        listRef.scrollTop = listRef.scrollHeight;
      }
    });
  });
  createEffect(() => {
    saveFilterPreferences({
      search: search(),
      messageType: messageType(),
      sourceRole: sourceRole()
    });
  });

  const { wsURL, token } = resolveConnectionParams();
  const socketURL = buildDaemonURL(wsURL, token);

  const connect = () => {
    if (stopped) {
      return;
    }

    const next = new WebSocket(socketURL);
    next.binaryType = "arraybuffer";
    socket = next;

    next.addEventListener("open", () => {
      if (socket !== next) {
        return;
      }

      reconnectAttempt = 0;
      setConnection("open");
      setLastError(null);

      const hello: Envelope<Hello> = {
        v: PROTOCOL_VERSION,
        t: "lifecycle",
        name: "hello",
        src: { role: "inspector", id: inspectorID },
        data: {
          protocol_version: PROTOCOL_VERSION,
          capabilities: ["query.events"]
        }
      };

      next.send(encode(hello));
    });

    next.addEventListener("message", (event) => {
      if (!(event.data instanceof ArrayBuffer)) {
        return;
      }

      let decoded: unknown;
      try {
        decoded = decode(new Uint8Array(event.data));
      } catch {
        return;
      }
      if (!isEnvelope(decoded)) {
        return;
      }

      if (isWelcome(decoded)) {
        const query: Envelope<QueryEvents> = {
          v: PROTOCOL_VERSION,
          t: "command",
          name: "query.events",
          src: { role: "inspector", id: inspectorID },
          data: { limit: defaultTimelineLimit }
        };
        next.send(encode(query));
        return;
      }

      if (isQueryEventsResult(decoded)) {
        applyQueryResult(decoded.data, setTimeline);
        return;
      }

      setTimeline((existing) =>
        mergeEntries(existing, [fromLiveEnvelope(decoded)], defaultTimelineLimit)
      );
    });

    next.addEventListener("close", () => {
      if (socket !== next) {
        return;
      }
      if (stopped) {
        setConnection("closed");
        return;
      }

      setConnection("reconnecting");
      const delay = reconnectDelay(reconnectAttempt);
      reconnectAttempt += 1;

      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = undefined;
        connect();
      }, delay);
    });

    next.addEventListener("error", () => {
      if (socket !== next) {
        return;
      }

      setLastError("websocket transport error");
      next.close();
    });
  };

  connect();

  onCleanup(() => {
    stopped = true;
    if (typeof reconnectTimer === "number") {
      window.clearTimeout(reconnectTimer);
    }
    socket?.close();
    setConnection("closed");
  });

  const errorBlock = () => {
    const value = lastError();
    if (!value) {
      return null;
    }

    return html`<p class="error">${value}</p>`;
  };

  const eventCards = () =>
    filteredTimeline().map((entry) => {
      return html`<article class="event-card">
        <div class="event-meta">
          <span>${formatTime(entry.recordedAtMS)}</span>
          <span>${entry.envelope.name}</span>
          <span>${entry.envelope.t}</span>
        </div>
        <div class="event-source">${entry.envelope.src.role}:${entry.envelope.src.id}</div>
        <p>${summarizeEnvelope(entry.envelope)}</p>
      </article>`;
    });
  const resetFilters = () => {
    setSearch(defaultTimelineFilter.search);
    setMessageType(defaultTimelineFilter.messageType);
    setSourceRole(defaultTimelineFilter.sourceRole);
  };

  return html`<main class="layout">
    <header class="topbar">
      <h1>Panex Inspector</h1>
      <p>Connection: ${connection}</p>
      <p class="subtle">${socketURL}</p>
      ${errorBlock}
    </header>

    <section class="panel">
      <div class="panel-header">
        <h2>Event Timeline</h2>
        <p>${() => `${filteredTimeline().length}/${timeline().length} events`}</p>
      </div>

      <div class="filters">
        <label class="filter-control">
          <span>Search</span>
          <input
            type="search"
            value=${search}
            placeholder="name:command.reload src:daemon build-42"
            onInput=${(event: Event) => {
              setSearch((event.currentTarget as HTMLInputElement).value);
            }}
          />
        </label>

        <label class="filter-control">
          <span>Type</span>
          <select
            value=${messageType}
            onChange=${(event: Event) => {
              setMessageType(
                (event.currentTarget as HTMLSelectElement).value as "all" | "lifecycle" | "event" | "command"
              );
            }}
          >
            <option value="all">all</option>
            <option value="lifecycle">lifecycle</option>
            <option value="event">event</option>
            <option value="command">command</option>
          </select>
        </label>

        <label class="filter-control">
          <span>Source</span>
          <select
            value=${sourceRole}
            onChange=${(event: Event) => {
              setSourceRole((event.currentTarget as HTMLSelectElement).value as "all" | "daemon" | "dev-agent" | "inspector");
            }}
          >
            <option value="all">all</option>
            <option value="daemon">daemon</option>
            <option value="dev-agent">dev-agent</option>
            <option value="inspector">inspector</option>
          </select>
        </label>

        <button class="filter-reset" type="button" onClick=${resetFilters}>reset</button>
      </div>
      <p class="filter-hint">
        operators: <code>name:</code> <code>src:</code> <code>type:</code> (combine with free text)
      </p>

      <div
        class="timeline"
        ref=${(element: Element) => {
          listRef = element as HTMLDivElement;
        }}
      >
        ${eventCards}
      </div>
    </section>
  </main>`;
}

function applyQueryResult(
  payload: QueryEventsResult,
  setTimeline: (updater: (existing: TimelineEntry[]) => TimelineEntry[]) => void
): void {
  if (!Array.isArray(payload.events)) {
    return;
  }

  const snapshots = payload.events.filter((event): event is QueryEventsResult["events"][number] => {
    return (
      typeof event === "object" &&
      event !== null &&
      typeof event.id === "number" &&
      typeof event.recorded_at_ms === "number" &&
      isEnvelope(event.envelope)
    );
  });

  setTimeline((existing) =>
    mergeEntries(
      existing,
      snapshots.map((snapshot) => fromSnapshot(snapshot)),
      defaultTimelineLimit
    )
  );
}

function resolveConnectionParams(): { wsURL: string; token: string } {
  const params = new URLSearchParams(window.location.search);
  return {
    wsURL: nonEmpty(params.get("ws"), "ws://127.0.0.1:4317/ws"),
    token: nonEmpty(params.get("token"), "dev-token")
  };
}

function buildDaemonURL(wsURL: string, token: string): string {
  const url = new URL(wsURL);
  url.searchParams.set("token", token);
  return url.toString();
}

function safeClientID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  return `${Date.now()}`;
}

function nonEmpty(value: string | null, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

function loadFilterPreferences() {
  if (typeof window === "undefined") {
    return defaultTimelineFilter;
  }

  try {
    const raw = window.localStorage.getItem(filterStorageKey);
    if (!raw) {
      return defaultTimelineFilter;
    }

    const parsed = JSON.parse(raw) as Partial<typeof defaultTimelineFilter>;
    return {
      search: typeof parsed.search === "string" ? parsed.search : defaultTimelineFilter.search,
      messageType: isMessageType(parsed.messageType) ? parsed.messageType : defaultTimelineFilter.messageType,
      sourceRole: isSourceRole(parsed.sourceRole) ? parsed.sourceRole : defaultTimelineFilter.sourceRole
    };
  } catch {
    return defaultTimelineFilter;
  }
}

function saveFilterPreferences(filter: typeof defaultTimelineFilter): void {
  if (typeof window === "undefined") {
    return;
  }

  try {
    window.localStorage.setItem(filterStorageKey, JSON.stringify(filter));
  } catch {
    // Ignore storage failures so private browsing limits do not break the inspector.
  }
}

function isMessageType(value: unknown): value is typeof defaultTimelineFilter.messageType {
  return value === "all" || value === "lifecycle" || value === "event" || value === "command";
}

function isSourceRole(value: unknown): value is typeof defaultTimelineFilter.sourceRole {
  return value === "all" || value === "daemon" || value === "dev-agent" || value === "inspector";
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
        return App();
      }
    }),
  appRoot
);

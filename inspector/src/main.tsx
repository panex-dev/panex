import { decode, encode } from "@msgpack/msgpack";
import html from "solid-js/html";
import { createEffect, createSignal, onCleanup } from "solid-js";
import { render } from "solid-js/web";

import "./styles.css";

import {
  PROTOCOL_VERSION,
  isEnvelope,
  isQueryEventsResult,
  isWelcome,
  type Envelope,
  type Hello,
  type QueryEvents,
  type QueryEventsResult
} from "./protocol";
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

const appRoot = document.getElementById("app");
if (!appRoot) {
  throw new Error("missing app root");
}

const inspectorID = `inspector-${safeClientID()}`;

function App() {
  const [connection, setConnection] = createSignal("connecting");
  const [timeline, setTimeline] = createSignal<TimelineEntry[]>([]);
  const [socketURL, setSocketURL] = createSignal("");
  const [lastError, setLastError] = createSignal<string | null>(null);
  const [search, setSearch] = createSignal(defaultTimelineFilter.search);
  const [messageType, setMessageType] = createSignal(defaultTimelineFilter.messageType);
  const [sourceRole, setSourceRole] = createSignal(defaultTimelineFilter.sourceRole);

  let listRef: HTMLDivElement | undefined;
  let socket: WebSocket | null = null;
  const filteredTimeline = () =>
    filterEntries(timeline(), {
      search: search(),
      messageType: messageType(),
      sourceRole: sourceRole()
    });

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

  const { wsURL, token } = resolveConnectionParams();
  setSocketURL(buildDaemonURL(wsURL, token));

  socket = new WebSocket(socketURL());
  socket.binaryType = "arraybuffer";

  socket.addEventListener("open", () => {
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

    socket?.send(encode(hello));
  });

  socket.addEventListener("message", (event) => {
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
        data: { limit: 200 }
      };
      socket?.send(encode(query));
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

  socket.addEventListener("close", () => {
    setConnection("closed");
  });

  socket.addEventListener("error", () => {
    setLastError("websocket transport error");
    socket?.close();
  });

  onCleanup(() => {
    socket?.close();
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
            placeholder="name, source, payload"
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
      </div>

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

render(App, appRoot);

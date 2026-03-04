import html from "solid-js/html";
import { createEffect, createMemo, createSignal, type Accessor } from "solid-js";

import {
  defaultTimelineFilter,
  filterEntries,
  formatTime,
  summarizeEnvelope,
  type TimelineEntry
} from "../timeline";

interface TimelineTabProps {
  timeline: Accessor<TimelineEntry[]>;
}

const filterStorageKey = "panex.inspector.filters.v1";

export function TimelineTab(props: TimelineTabProps) {
  const initialFilter = loadFilterPreferences();
  const [search, setSearch] = createSignal(initialFilter.search);
  const [messageType, setMessageType] = createSignal(initialFilter.messageType);
  const [sourceRole, setSourceRole] = createSignal(initialFilter.sourceRole);

  let listRef: HTMLDivElement | undefined;

  const filteredTimeline = createMemo(() =>
    filterEntries(props.timeline(), {
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

  return html`<section class="panel">
    <div class="panel-header">
      <h2>Event Timeline</h2>
      <p>${() => `${filteredTimeline().length}/${props.timeline().length} events`}</p>
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
              (event.currentTarget as HTMLSelectElement).value as
                | "all"
                | "lifecycle"
                | "event"
                | "command"
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
            setSourceRole(
              (event.currentTarget as HTMLSelectElement).value as
                | "all"
                | "daemon"
                | "dev-agent"
                | "inspector"
            );
          }}
        >
          <option value="all">all</option>
          <option value="daemon">daemon</option>
          <option value="dev-agent">dev-agent</option>
          <option value="inspector">inspector</option>
        </select>
      </label>

      <button class="filter-reset" type="button" onClick=${resetFilters}>
        reset
      </button>
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
  </section>`;
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

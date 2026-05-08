import h from "solid-js/h";
import { createEffect, createMemo, createSignal, type Accessor, type JSX } from "solid-js";

import type { BridgeSession } from "../connection";
import {
  defaultTimelineFilter,
  defaultTimelineLimit,
  defaultTimelineRenderWindow,
  filterEntries,
  formatTime,
  hiddenOlderTimelineCount,
  renderTimelineWindow,
  summarizeEnvelope,
  type TimelineEntry
} from "../timeline";

interface TimelineTabProps {
  bridgeSession: Accessor<BridgeSession | null>;
  timeline: Accessor<TimelineEntry[]>;
  canLoadOlderTimeline: Accessor<boolean>;
  loadingOlderTimeline: Accessor<boolean>;
  loadingLatestTimeline: Accessor<boolean>;
  trimmedOlderTimelineCount: Accessor<number>;
  trimmedNewerTimelineCount: Accessor<number>;
  loadOlderTimeline: () => boolean;
  jumpToLatestTimeline: () => boolean;
}

const filterStorageKey = "panex.inspector.filters.v1";

export function TimelineTab(props: TimelineTabProps): JSX.Element {
  const initialFilter = loadFilterPreferences();
  const [search, setSearch] = createSignal(initialFilter.search);
  const [messageType, setMessageType] = createSignal(initialFilter.messageType);
  const [sourceRole, setSourceRole] = createSignal(initialFilter.sourceRole);
  const [visibleCount, setVisibleCount] = createSignal(defaultTimelineRenderWindow);
  const [followLatest, setFollowLatest] = createSignal(true);

  let listRef: HTMLDivElement | undefined;

  const filteredTimeline = createMemo(() =>
    filterEntries(props.timeline(), {
      search: search(),
      messageType: messageType(),
      sourceRole: sourceRole()
    })
  );
  const renderedTimeline = createMemo(() =>
    renderTimelineWindow(filteredTimeline(), visibleCount())
  );
  const hiddenOlderCount = createMemo(() =>
    hiddenOlderTimelineCount(filteredTimeline(), visibleCount())
  );
  const canQueryTimeline = createMemo(() =>
    props.bridgeSession()?.capabilitiesSupported.includes("query.events") ?? false
  );

  createEffect(() => {
    if (!listRef || !followLatest()) {
      return;
    }

    renderedTimeline().length;
    queueMicrotask(() => {
      if (listRef && followLatest()) {
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

  const showOlderLoaded = () => {
    setFollowLatest(false);
    setVisibleCount((current) => Math.min(filteredTimeline().length, current + defaultTimelineRenderWindow));
  };

  const loadOlderTimeline = () => {
    if (!props.loadOlderTimeline()) {
      return;
    }

    setFollowLatest(false);
    setVisibleCount((current) => current + defaultTimelineLimit);
  };

  const jumpToNewest = () => {
    setVisibleCount(defaultTimelineRenderWindow);
    setFollowLatest(true);
    props.jumpToLatestTimeline();
  };

  const resetFilters = () => {
    setSearch(defaultTimelineFilter.search);
    setMessageType(defaultTimelineFilter.messageType);
    setSourceRole(defaultTimelineFilter.sourceRole);
    jumpToNewest();
  };

  return (
    <section class="panel">
      <div class="panel-header">
        <h2>Event Timeline</h2>
        <div class="panel-actions">
          <p>
            {`${renderedTimeline().length}/${filteredTimeline().length} shown · ${props.timeline().length} active window`}
            {props.trimmedOlderTimelineCount() > 0
              ? ` · older trimmed ${props.trimmedOlderTimelineCount()}`
              : ""}
            {props.trimmedNewerTimelineCount() > 0
              ? ` · newer available ${props.trimmedNewerTimelineCount()}`
              : ""}
          </p>
          {hiddenOlderCount() > 0 ? (
            <button
              class="filter-reset"
              type="button"
              onClick={showOlderLoaded}
            >
              {`show older loaded (${hiddenOlderCount()})`}
            </button>
          ) : null}
          {props.canLoadOlderTimeline() ? (
            <button
              class="filter-reset"
              type="button"
              disabled={props.loadingOlderTimeline()}
              onClick={loadOlderTimeline}
            >
              {props.loadingOlderTimeline() ? "loading older..." : "load older"}
            </button>
          ) : null}
          {!followLatest() || props.trimmedNewerTimelineCount() > 0 ? (
            <button
              class="filter-reset"
              type="button"
              disabled={props.loadingLatestTimeline()}
              onClick={jumpToNewest}
            >
              {props.loadingLatestTimeline() ? "loading newest..." : "jump to newest"}
            </button>
          ) : null}
        </div>
      </div>

      <div class="filters">
        <label class="filter-control">
          <span>Search</span>
          <input
            type="search"
            value={search()}
            placeholder="name:command.reload src:daemon build-42"
            aria-describedby="timeline-filter-hint"
            onInput={(event) => {
              setSearch(event.currentTarget.value);
            }}
          />
        </label>

        <label class="filter-control">
          <span>Type</span>
          <select
            value={messageType()}
            onChange={(event) => {
              setMessageType(
                event.currentTarget.value as "all" | "lifecycle" | "event" | "command"
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
            value={sourceRole()}
            onChange={(event) => {
              setSourceRole(
                event.currentTarget.value as "all" | "daemon" | "dev-agent" | "inspector"
              );
            }}
          >
            <option value="all">all</option>
            <option value="daemon">daemon</option>
            <option value="dev-agent">dev-agent</option>
            <option value="inspector">inspector</option>
          </select>
        </label>

        <button class="filter-reset" type="button" onClick={resetFilters}>
          reset
        </button>
      </div>

      <p id="timeline-filter-hint" class="filter-hint">
        operators: <code>name:</code> <code>src:</code> <code>type:</code> (combine with free text)
      </p>
      {props.bridgeSession() && !canQueryTimeline() ? (
        <p class="subtle" role="status" aria-live="polite">
          Persisted timeline queries were not negotiated for this session. Live bridge events can
          still appear here when the daemon broadcasts them.
        </p>
      ) : null}

      <div
        class="timeline"
        ref={(element) => {
          listRef = element;
        }}
      >
        {renderedTimeline().map((entry) => (
          <article class="event-card">
            <div class="event-meta">
              <span>{formatTime(entry.recordedAtMS)}</span>
              <span>{entry.envelope.name}</span>
              <span>{entry.envelope.t}</span>
            </div>
            <div class="event-source">
              {entry.envelope.src.role}:{entry.envelope.src.id}
            </div>
            <p>{summarizeEnvelope(entry.envelope)}</p>
          </article>
        ))}
      </div>
    </section>
  );
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
      messageType: isMessageType(parsed.messageType)
        ? parsed.messageType
        : defaultTimelineFilter.messageType,
      sourceRole: isSourceRole(parsed.sourceRole)
        ? parsed.sourceRole
        : defaultTimelineFilter.sourceRole
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

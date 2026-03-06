import html from "solid-js/html";
import { createMemo, type Accessor } from "solid-js";

import type { StorageSnapshot } from "@panex/protocol";

import type { ConnectionStatus } from "../connection";
import type { TimelineEntry } from "../timeline";
import { buildWorkbenchModel } from "../workbench";

interface WorkbenchTabProps {
  status: Accessor<ConnectionStatus>;
  socketURL: Accessor<string>;
  lastError: Accessor<string | null>;
  storage: Accessor<StorageSnapshot[]>;
  timeline: Accessor<TimelineEntry[]>;
}

const plannedTools = [
  "Preset storage mutations",
  "Runtime message probes",
  "Replay controls"
] as const;

export function WorkbenchTab(props: WorkbenchTabProps) {
  const model = createMemo(() =>
    buildWorkbenchModel({
      status: props.status(),
      socketURL: props.socketURL(),
      lastError: props.lastError(),
      storage: props.storage(),
      timeline: props.timeline()
    })
  );

  const storageAreaRows = () => {
    if (model().storageAreas.length === 0) {
      return html`<li class="workbench-list-empty">No storage snapshots loaded yet.</li>`;
    }

    return model().storageAreas.map((entry) => {
      return html`<li>
        <span>${entry.area}</span>
        <strong>${entry.keys} keys</strong>
      </li>`;
    });
  };

  return html`<section class="panel workbench-panel">
    <div class="panel-header">
      <h2>Workbench</h2>
      <p>read-only overview</p>
    </div>

    <div class="workbench-intro">
      <p>
        Workbench is enabled as an operator cockpit in this milestone. It surfaces live state from the
        existing inspector connection without introducing new daemon or protocol actions yet.
      </p>
    </div>

    <div class="workbench-grid">
      <article class="workbench-card">
        <p class="workbench-eyebrow">Connection</p>
        <strong class="workbench-metric">${() => model().status}</strong>
        <p class="subtle">${() => model().socketURL}</p>
        ${() => (model().lastError ? html`<p class="error">${model().lastError}</p>` : null)}
      </article>

      <article class="workbench-card">
        <p class="workbench-eyebrow">Timeline</p>
        <strong class="workbench-metric">${() => `${model().timeline.totalEvents} events`}</strong>
        <p class="subtle">
          ${() => `${model().timeline.persistedEvents} persisted · ${model().timeline.liveEvents} live`}
        </p>
        <p>
          ${() =>
            model().timeline.latestEventName
              ? `${model().timeline.latestEventName} from ${model().timeline.latestEventSource}`
              : "No timeline events yet."}
        </p>
      </article>

      <article class="workbench-card">
        <p class="workbench-eyebrow">Storage</p>
        <strong class="workbench-metric">${() => `${model().totalStorageKeys} keys`}</strong>
        <ul class="workbench-list">${storageAreaRows}</ul>
      </article>

      <article class="workbench-card">
        <p class="workbench-eyebrow">Planned tools</p>
        <ul class="workbench-list">
          ${plannedTools.map((label) => html`<li><span>${label}</span><strong>planned</strong></li>`)}
        </ul>
        <p class="subtle">No mutating actions are exposed from Workbench in this milestone.</p>
      </article>
    </div>
  </section>`;
}

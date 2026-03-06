import html from "solid-js/html";
import { createMemo, createSignal, type Accessor } from "solid-js";

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
  setStorageItem: (area: string, key: string, value: unknown) => boolean;
  removeStorageItem: (area: string, key: string) => boolean;
  sendRuntimeMessage: (message: unknown) => boolean;
}

const plannedTools = ["Replay controls"] as const;

export function WorkbenchTab(props: WorkbenchTabProps) {
  const [presetMessage, setPresetMessage] = createSignal<string | null>(null);
  const [runtimeMessage, setRuntimeMessage] = createSignal<string | null>(null);
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

  const presetRows = () =>
    model().storagePresets.map((preset) => {
      return html`<li class="workbench-preset">
        <div class="workbench-preset-copy">
          <div class="workbench-preset-heading">
            <strong>${preset.label}</strong>
            <span class=${`workbench-pill workbench-pill-${preset.state}`}>${preset.state}</span>
          </div>
          <p class="subtle">${preset.description}</p>
          <p class="subtle">
            <code>${preset.area}</code> / <code>${preset.key}</code>
          </p>
          <p class="subtle">
            ${preset.currentValueText
              ? html`Current: <code>${preset.currentValueText}</code>`
              : "Current: not set"}
          </p>
        </div>

        <div class="workbench-preset-actions">
          <button
            class="filter-reset"
            type="button"
            disabled=${() => props.status() !== "open"}
            onClick=${() => {
              const sent =
                preset.actionLabel === "remove"
                  ? props.removeStorageItem(preset.area, preset.key)
                  : props.setStorageItem(preset.area, preset.key, preset.value);

              setPresetMessage(
                sent
                  ? `Sent storage.${preset.actionLabel === "remove" ? "remove" : "set"} for ${preset.key}.`
                  : `Unable to send ${preset.actionLabel} for ${preset.key} while websocket is closed.`
              );
            }}
          >
            ${preset.actionLabel}
          </button>
        </div>
      </li>`;
    });

  const runtimeProbe = () => model().runtimeProbe;

  return html`<section class="panel workbench-panel">
    <div class="panel-header">
      <h2>Workbench</h2>
      <p>overview + live tools</p>
    </div>

    <div class="workbench-intro">
      <p>
        Workbench now exposes constrained storage and runtime actions on top of existing transport.
        Both tools stay namespaced and reversible so the surface can grow without widening backend
        contracts prematurely.
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
        <p class="workbench-eyebrow">Runtime probe</p>
        <strong class="workbench-metric">1 live probe</strong>
        <p class="subtle">${() => runtimeProbe().description}</p>
        <p class="subtle">Payload: <code>${() => runtimeProbe().payloadText}</code></p>
        <button
          class="filter-reset"
          type="button"
          disabled=${() => props.status() !== "open"}
          onClick=${() => {
            const sent = props.sendRuntimeMessage(runtimeProbe().payload);
            setRuntimeMessage(
              sent
                ? `Sent runtime.sendMessage for ${runtimeProbe().id}.`
                : `Unable to send runtime probe for ${runtimeProbe().id} while websocket is closed.`
            );
          }}
        >
          ${() => runtimeProbe().label}
        </button>
        ${() => (runtimeMessage() ? html`<p class="subtle">${runtimeMessage()}</p>` : null)}
        <p class="subtle">
          ${() =>
            runtimeProbe().lastResultText
              ? `Last result: ${runtimeProbe().lastResultText}`
              : "Last result: none yet"}
        </p>
        <p class="subtle">
          ${() =>
            runtimeProbe().lastEventText
              ? `Last onMessage payload: ${runtimeProbe().lastEventText}`
              : "Last onMessage payload: none yet"}
        </p>
      </article>

      <article class="workbench-card workbench-card-wide">
        <p class="workbench-eyebrow">Storage presets</p>
        <strong class="workbench-metric">3 reversible actions</strong>
        <p class="subtle">
          These presets only touch namespaced demo keys and automatically switch between apply, update,
          and remove based on the current storage snapshot.
        </p>
        ${() =>
          presetMessage() ? html`<p class="subtle">${presetMessage()}</p>` : null}
        ${() =>
          props.status() === "open"
            ? null
            : html`<p class="subtle">Presets become available after the daemon websocket opens.</p>`}
        <ul class="workbench-preset-list">${presetRows}</ul>
      </article>

      <article class="workbench-card">
        <p class="workbench-eyebrow">Roadmap</p>
        <strong class="workbench-metric">2 live tools</strong>
        <ul class="workbench-list">
          ${plannedTools.map((label) => html`<li><span>${label}</span><strong>planned</strong></li>`)}
        </ul>
        <p class="subtle">Storage presets and the runtime ping probe are live. The remaining tools stay intentionally deferred.</p>
      </article>
    </div>
  </section>`;
}

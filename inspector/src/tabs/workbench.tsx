import h from "solid-js/h";
import { createMemo, createSignal, type Accessor, type JSX } from "solid-js";

import type { StorageSnapshot } from "@panex/protocol";

import { summarizeChromeAPIActivity } from "../activity-log";
import type { BridgeSession, ConnectionStatus } from "../connection";
import type { TimelineEntry } from "../timeline";
import { formatTime } from "../timeline";
import { buildWorkbenchModel } from "../workbench";

interface WorkbenchTabProps {
  status: Accessor<ConnectionStatus>;
  bridgeSession: Accessor<BridgeSession | null>;
  socketURL: Accessor<string>;
  lastError: Accessor<string | null>;
  storage: Accessor<StorageSnapshot[]>;
  timeline: Accessor<TimelineEntry[]>;
  setStorageItem: (area: string, key: string, value: unknown) => boolean;
  removeStorageItem: (area: string, key: string) => boolean;
  sendRuntimeMessage: (message: unknown) => boolean;
}

const plannedTools = [] as const;

export function WorkbenchTab(props: WorkbenchTabProps): JSX.Element {
  const [presetMessage, setPresetMessage] = createSignal<string | null>(null);
  const [runtimeMessage, setRuntimeMessage] = createSignal<string | null>(null);
  const [replayMessage, setReplayMessage] = createSignal<string | null>(null);
  const model = createMemo(() =>
    buildWorkbenchModel({
      status: props.status(),
      bridgeSession: props.bridgeSession(),
      socketURL: props.socketURL(),
      lastError: props.lastError(),
      storage: props.storage(),
      timeline: props.timeline()
    })
  );

  const runtimeProbe = () => model().runtimeProbe;
  const activity = createMemo(() => summarizeChromeAPIActivity(props.timeline()));

  return (
    <section class="panel workbench-panel">
      <div class="panel-header">
        <h2>Workbench</h2>
        <p>overview + live tools</p>
      </div>

      <div class="workbench-intro">
        <p>
          Diagnostics panel for transport, storage, and runtime probing.
          This is not a preview surface — there is no iframe rendering or Vite integration.
          All tools stay namespaced and reversible so the surface can grow without widening backend
          contracts prematurely.
        </p>
      </div>

      <div class="workbench-grid">
        <article class="workbench-card">
          <p class="workbench-eyebrow">Connection</p>
          <strong class="workbench-metric">{model().status}</strong>
          <p class="subtle">{model().socketURL}</p>
          {(() => {
            const bridgeSession = model().bridgeSession;
            if (!bridgeSession) {
              return <p class="subtle">Handshake metadata will appear after hello.ack completes.</p>;
            }

            return (
              <>
                <p class="subtle">
                  {`daemon ${bridgeSession.daemonVersion} · session ${bridgeSession.sessionID}`}
                </p>
                <p class="subtle">
                  {bridgeSession.extensionID
                    ? `extension ${bridgeSession.extensionID}`
                    : "extension not negotiated for this client"}
                </p>
                <p class="subtle">
                  {bridgeSession.capabilitiesSupported.length
                    ? `capabilities: ${bridgeSession.capabilitiesSupported.join(", ")}`
                    : "capabilities: none negotiated"}
                </p>
              </>
            );
          })()}
          {model().lastError ? <p class="error">{model().lastError}</p> : null}
        </article>

        <article class="workbench-card">
          <p class="workbench-eyebrow">Timeline</p>
          <strong class="workbench-metric">{`${model().timeline.totalEvents} events`}</strong>
          <p class="subtle">{`${model().timeline.persistedEvents} persisted · ${model().timeline.liveEvents} live`}</p>
          <p>
            {model().timeline.latestEventName
              ? `${model().timeline.latestEventName} from ${model().timeline.latestEventSource}`
              : "No timeline events yet."}
          </p>
        </article>

        <article class="workbench-card">
          <p class="workbench-eyebrow">Storage</p>
          <strong class="workbench-metric">{`${model().totalStorageKeys} keys`}</strong>
          <ul class="workbench-list">
            {model().storageAreas.length === 0 ? (
              <li class="workbench-list-empty">No storage snapshots loaded yet.</li>
            ) : (
              model().storageAreas.map((entry) => (
                <li>
                  <span>{entry.area}</span>
                  <strong>{entry.keys} keys</strong>
                </li>
              ))
            )}
          </ul>
        </article>

        <article class="workbench-card">
          <p class="workbench-eyebrow">Runtime probe</p>
          <strong class="workbench-metric">1 live probe</strong>
          <p class="subtle">{runtimeProbe().description}</p>
          <p class="subtle">
            Payload: <code>{runtimeProbe().payloadText}</code>
          </p>
          <button
            class="filter-reset"
            type="button"
            disabled={props.status() !== "open"}
            onClick={() => {
              const sent = props.sendRuntimeMessage(runtimeProbe().payload);
              setRuntimeMessage(
                sent
                  ? `Sent runtime.sendMessage for ${runtimeProbe().id}.`
                  : `Unable to send runtime probe for ${runtimeProbe().id} while websocket is closed.`
              );
            }}
          >
            {runtimeProbe().label}
          </button>
          {runtimeMessage() ? (
            <p class="subtle" role="status" aria-live="polite">
              {runtimeMessage()}
            </p>
          ) : null}
          <p class="subtle">
            {runtimeProbe().lastResultText
              ? `Last result: ${runtimeProbe().lastResultText}`
              : "Last result: none yet"}
          </p>
          <p class="subtle">
            {runtimeProbe().lastEventText
              ? `Last onMessage payload: ${runtimeProbe().lastEventText}`
              : "Last onMessage payload: none yet"}
          </p>
          <p class="subtle">
            {runtimeProbe().replaySourceText
              ? `Replay source: ${runtimeProbe().replaySourceText}`
              : "Replay source: unavailable until the timeline captures a runtime probe."}
          </p>
          <button
            class="filter-reset"
            type="button"
            disabled={props.status() !== "open" || runtimeProbe().replayPayload === null}
            onClick={() => {
              const replayPayload = runtimeProbe().replayPayload;
              if (!replayPayload) {
                setReplayMessage("No replayable runtime payload has been observed yet.");
                return;
              }

              const sent = props.sendRuntimeMessage(replayPayload);
              setReplayMessage(
                sent
                  ? `Replayed runtime.sendMessage from ${runtimeProbe().replaySourceText}.`
                  : "Unable to replay the last runtime payload while websocket is closed."
              );
            }}
          >
            replay last runtime payload
          </button>
          {replayMessage() ? (
            <p class="subtle" role="status" aria-live="polite">
              {replayMessage()}
            </p>
          ) : null}
        </article>

        <article class="workbench-card workbench-card-wide">
          <p class="workbench-eyebrow">Chrome API activity</p>
          <strong class="workbench-metric">{`${activity().length} recent entries`}</strong>
          <p class="subtle">
            Focused simulator traffic only: paired calls/results plus observed runtime events from
            the existing timeline history.
          </p>
          {activity().length === 0 ? (
            <p class="subtle">
              No chrome API activity has been observed yet. Use the runtime probe or other
              simulator-backed actions to populate this log.
            </p>
          ) : (
            <ul class="workbench-activity-list">
              {activity().map((entry) => (
                <li class="workbench-activity-item">
                  <div class="workbench-activity-copy">
                    <div class="workbench-preset-heading">
                      <strong>{entry.title}</strong>
                      <span class={`workbench-pill workbench-pill-${entry.status}`}>{entry.status}</span>
                    </div>
                    <p class="subtle">
                      {entry.callID ? `Call ${entry.callID}` : "Event observation"} ·{" "}
                      {formatTime(entry.recordedAtMS)}
                      {entry.latencyMS === null ? "" : ` · ${entry.latencyMS}ms`}
                    </p>
                    <p class="subtle">{entry.detail}</p>
                    <pre class="replay-payload">{entry.payloadText}</pre>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </article>

        <article class="workbench-card workbench-card-wide">
          <p class="workbench-eyebrow">Storage presets</p>
          <strong class="workbench-metric">3 reversible actions</strong>
          <p class="subtle">
            These presets only touch namespaced demo keys and automatically switch between apply,
            update, and remove based on the current storage snapshot.
          </p>
          {presetMessage() ? (
            <p class="subtle" role="status" aria-live="polite">
              {presetMessage()}
            </p>
          ) : null}
          {props.status() === "open" ? null : (
            <p class="subtle" role="status" aria-live="polite">
              Presets become available after the daemon websocket opens.
            </p>
          )}
          <ul class="workbench-preset-list">
            {model().storagePresets.map((preset) => (
              <li class="workbench-preset">
                <div class="workbench-preset-copy">
                  <div class="workbench-preset-heading">
                    <strong>{preset.label}</strong>
                    <span class={`workbench-pill workbench-pill-${preset.state}`}>{preset.state}</span>
                  </div>
                  <p class="subtle">{preset.description}</p>
                  <p class="subtle">
                    <code>{preset.area}</code> / <code>{preset.key}</code>
                  </p>
                  <p class="subtle">
                    {preset.currentValueText ? (
                      <span>
                        Current: <code>{preset.currentValueText}</code>
                      </span>
                    ) : (
                      "Current: not set"
                    )}
                  </p>
                </div>

                <div class="workbench-preset-actions">
                  <button
                    class="filter-reset"
                    type="button"
                    disabled={props.status() !== "open"}
                    onClick={() => {
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
                    {preset.actionLabel}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </article>

        <article class="workbench-card">
          <p class="workbench-eyebrow">Roadmap</p>
          <strong class="workbench-metric">3 live controls</strong>
          {plannedTools.length === 0 ? (
            <p class="subtle">
              Replay controls now span the Workbench quick action and the focused Replay tab.
              Broader replay families remain intentionally deferred until they have their own
              explicit contract.
            </p>
          ) : (
            <ul class="workbench-list">
              {plannedTools.map((label) => (
                <li>
                  <span>{label}</span>
                  <strong>planned</strong>
                </li>
              ))}
            </ul>
          )}
          <p class="subtle">
            Storage presets, the runtime ping probe, and history-driven replay of observed runtime
            payloads are live.
          </p>
        </article>
      </div>
    </section>
  );
}

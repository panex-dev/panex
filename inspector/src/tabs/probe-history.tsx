import h from "solid-js/h";
import { createMemo, createSignal, type Accessor, type JSX } from "solid-js";

import type { ConnectionStatus } from "../connection";
import { summarizeReplayHistory } from "../replay";
import { replayFamilies } from "../replay-contract";
import { formatTime, type TimelineEntry } from "../timeline";

interface ReplayTabProps {
  status: Accessor<ConnectionStatus>;
  socketURL: Accessor<string>;
  lastError: Accessor<string | null>;
  timeline: Accessor<TimelineEntry[]>;
  sendRuntimeMessage: (message: unknown) => boolean;
}

export function ProbeHistoryTab(props: ReplayTabProps): JSX.Element {
  const [feedback, setFeedback] = createSignal<string | null>(null);
  const entries = createMemo(() => summarizeReplayHistory(props.timeline()));
  const replayFamily = replayFamilies[0];

  return (
    <section class="panel replay-panel">
      <div class="panel-header">
        <h2>Probe History</h2>
        <p>{`${entries().length} replayable ${replayFamily.payloadLabel}`}</p>
      </div>

      <div class="workbench-intro">
        <p>
          Replay is still constrained to the {replayFamily.label} family. This tab
          surfaces the history directly so operators can replay what the system actually saw, not a
          speculative local draft.
        </p>
      </div>

      <div class="replay-summary">
        <p class="subtle">connection: {props.status()}</p>
        <p class="subtle">{props.socketURL()}</p>
        {props.lastError() ? (
          <p class="error" role="status" aria-live="polite">
            {props.lastError()}
          </p>
        ) : null}
        {feedback() ? (
          <p class="subtle" role="status" aria-live="polite">
            {feedback()}
          </p>
        ) : null}
      </div>

      <div class="placeholder-body">
        {entries().length === 0 ? (
          <p class="subtle">
            No replayable {replayFamily.payloadLabel} have been observed yet. Use Workbench to send the
            runtime probe first, then return here to replay specific observed payloads from history.
          </p>
        ) : (
          <ul class="replay-list">
            {entries().map((entry) => (
              <li class="replay-item">
                <div class="replay-item-copy">
                  <div class="workbench-preset-heading">
                    <strong>{formatTime(entry.recordedAtMS)}</strong>
                    <span class="workbench-pill workbench-pill-applied">{entry.sourceText}</span>
                  </div>
                  <p class="subtle">
                    {entry.callID ? `Observed from ${entry.sourceText} (${entry.callID}).` : `Observed from ${entry.sourceText}.`}
                  </p>
                  <pre class="replay-payload">{entry.payloadText}</pre>
                </div>

                <div class="workbench-preset-actions">
                  <button
                    class="filter-reset"
                    type="button"
                    disabled={props.status() !== "open"}
                    onClick={() => {
                      const sent = props.sendRuntimeMessage(entry.payload);
                      setFeedback(
                        sent
                          ? `Replayed payload observed at ${formatTime(entry.recordedAtMS)} from ${entry.sourceText}.`
                          : `Unable to replay payload observed at ${formatTime(entry.recordedAtMS)} while websocket is closed.`
                      );
                    }}
                  >
                    replay payload
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

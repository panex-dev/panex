import html from "solid-js/html";
import { createEffect, createMemo, createSignal, type Accessor } from "solid-js";

import type { QueryStorage, StorageSnapshot } from "../../../shared/protocol/src/index";
import type { ConnectionStatus } from "../connection";
import {
  flattenStorageSnapshots,
  isStorageAreaFilter,
  type StorageAreaFilter
} from "../storage";

interface StorageTabProps {
  status: Accessor<ConnectionStatus>;
  storage: Accessor<StorageSnapshot[]>;
  storageHighlights: Accessor<Set<string>>;
  refreshStorage: (area?: QueryStorage["area"]) => boolean;
}

const storageAreas: readonly StorageAreaFilter[] = ["all", "local", "sync", "session"];

export function StorageTab(props: StorageTabProps) {
  const [area, setArea] = createSignal<StorageAreaFilter>("all");
  const [requestError, setRequestError] = createSignal<string | null>(null);

  const rows = createMemo(() => flattenStorageSnapshots(props.storage(), area()));
  const selectedAreaLabel = createMemo(() => (area() === "all" ? "all areas" : area()));

  createEffect(() => {
    if (props.status() !== "open") {
      return;
    }

    const selectedArea = area();
    const sent = props.refreshStorage(selectedArea === "all" ? undefined : selectedArea);
    setRequestError(sent ? null : "unable to query storage while websocket is closed");
  });

  const rowEntries = () => {
    if (rows().length === 0) {
      return html`<tr>
        <td colspan="3" class="storage-empty">No storage keys found for ${selectedAreaLabel()}.</td>
      </tr>`;
    }

    return rows().map((row) => {
      return html`<tr class=${props.storageHighlights().has(row.rowID) ? "storage-row-highlight" : ""}>
        <td><code>${row.area}</code></td>
        <td><code>${row.key}</code></td>
        <td><span class="storage-value">${truncateStorageValue(row.valueText)}</span></td>
      </tr>`;
    });
  };

  const statusHint = () => {
    if (props.status() === "open") {
      return requestError() ? html`<p class="error">${requestError()}</p>` : null;
    }

    return html`<p class="subtle">Storage queries run after the daemon websocket is open.</p>`;
  };

  return html`<section class="panel placeholder-panel">
    <div class="panel-header">
      <h2>Storage</h2>
      <p>${() => `${rows().length} keys · ${selectedAreaLabel()}`}</p>
    </div>

    <div class="filters storage-filters">
      <label class="filter-control">
        <span>Area</span>
        <select
          value=${area}
          onChange=${(event: Event) => {
            const next = (event.currentTarget as HTMLSelectElement).value;
            if (isStorageAreaFilter(next)) {
              setArea(next);
            }
          }}
        >
          ${storageAreas.map((entry) => html`<option value=${entry}>${entry}</option>`)}
        </select>
      </label>

      <button
        class="filter-reset"
        type="button"
        onClick=${() => {
          const selectedArea = area();
          const sent = props.refreshStorage(selectedArea === "all" ? undefined : selectedArea);
          setRequestError(sent ? null : "unable to query storage while websocket is closed");
        }}
      >
        refresh
      </button>
    </div>

    <div class="placeholder-body">
      <p class="filter-hint">Snapshot source: <code>query.storage.result</code></p>
      ${statusHint}

      <div class="storage-table-wrap">
        <table class="storage-table">
          <thead>
            <tr>
              <th scope="col">area</th>
              <th scope="col">key</th>
              <th scope="col">value</th>
            </tr>
          </thead>
          <tbody>
            ${rowEntries}
          </tbody>
        </table>
      </div>
    </div>
  </section>`;
}

function truncateStorageValue(value: string, maxLength = 240): string {
  if (value.length <= maxLength) {
    return value;
  }
  return `${value.slice(0, maxLength - 3)}...`;
}

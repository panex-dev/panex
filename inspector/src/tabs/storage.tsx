import {
  QUERY_STORAGE_MESSAGE_NAME,
  STORAGE_CLEAR_MESSAGE_NAME,
  STORAGE_REMOVE_MESSAGE_NAME,
  STORAGE_SET_MESSAGE_NAME,
  type QueryStorage,
  type StorageSnapshot
} from "@panex/protocol";

import h from "solid-js/h";
import { createEffect, createMemo, createSignal, type Accessor, type JSX } from "solid-js";
import type { BridgeSession, ConnectionStatus } from "../connection";
import {
  flattenStorageSnapshots,
  isStorageAreaFilter,
  type StorageArea,
  type StorageAreaFilter
} from "../storage";

interface StorageTabProps {
  status: Accessor<ConnectionStatus>;
  bridgeSession: Accessor<BridgeSession | null>;
  storage: Accessor<StorageSnapshot[]>;
  storageHighlights: Accessor<Set<string>>;
  refreshStorage: (area?: QueryStorage["area"]) => boolean;
  setStorageItem: (area: string, key: string, value: unknown) => boolean;
  removeStorageItem: (area: string, key: string) => boolean;
  clearStorageArea: (area: string) => boolean;
}

const storageAreas: readonly StorageAreaFilter[] = ["all", "local", "sync", "session"];
const mutableStorageAreas: readonly StorageArea[] = ["local", "sync", "session"];

export function StorageTab(props: StorageTabProps): JSX.Element {
  const [area, setArea] = createSignal<StorageAreaFilter>("all");
  const [requestError, setRequestError] = createSignal<string | null>(null);
  const [mutationError, setMutationError] = createSignal<string | null>(null);
  const [mutationArea, setMutationArea] = createSignal<StorageArea>("local");
  const [mutationKey, setMutationKey] = createSignal("");
  const [mutationValue, setMutationValue] = createSignal("");

  const rows = createMemo(() => flattenStorageSnapshots(props.storage(), area()));
  const selectedAreaLabel = createMemo(() => (area() === "all" ? "all areas" : area()));
  const canQueryStorage = createMemo(() =>
    props.bridgeSession()?.capabilitiesSupported.includes(QUERY_STORAGE_MESSAGE_NAME) ?? false
  );
  const canSetStorage = createMemo(() =>
    props.bridgeSession()?.capabilitiesSupported.includes(STORAGE_SET_MESSAGE_NAME) ?? false
  );
  const canRemoveStorage = createMemo(() =>
    props.bridgeSession()?.capabilitiesSupported.includes(STORAGE_REMOVE_MESSAGE_NAME) ?? false
  );
  const canClearStorage = createMemo(() =>
    props.bridgeSession()?.capabilitiesSupported.includes(STORAGE_CLEAR_MESSAGE_NAME) ?? false
  );

  createEffect(() => {
    if (props.status() !== "open" || !canQueryStorage()) {
      return;
    }

    const selectedArea = area();
    const sent = props.refreshStorage(selectedArea === "all" ? undefined : selectedArea);
    setRequestError(sent ? null : "unable to query storage while websocket is closed");
  });

  return (
    <section class="panel placeholder-panel">
      <div class="panel-header">
        <h2>Storage</h2>
        <p>{`${rows().length} keys · ${selectedAreaLabel()}`}</p>
      </div>

      <div class="filters storage-filters">
        <label class="filter-control">
          <span>Area</span>
          <select
            value={area()}
            onChange={(event) => {
              const next = event.currentTarget.value;
              if (isStorageAreaFilter(next)) {
                setArea(next);
              }
            }}
          >
            {storageAreas.map((entry) => (
              <option value={entry}>{entry}</option>
            ))}
          </select>
        </label>

        <button
          class="filter-reset"
          type="button"
          disabled={props.status() !== "open" || !canQueryStorage()}
          onClick={() => {
            const selectedArea = area();
            const sent = props.refreshStorage(selectedArea === "all" ? undefined : selectedArea);
            setRequestError(sent ? null : "unable to query storage while websocket is closed");
          }}
        >
          refresh
        </button>
      </div>

      <div class="storage-mutation-grid">
        <label class="filter-control">
          <span>Mutation area</span>
          <select
            value={mutationArea()}
            onChange={(event) => {
              const next = event.currentTarget.value as StorageArea;
              if (next === "local" || next === "sync" || next === "session") {
                setMutationArea(next);
              }
            }}
          >
            {mutableStorageAreas.map((entry) => (
              <option value={entry}>{entry}</option>
            ))}
          </select>
        </label>

        <label class="filter-control">
          <span>Key</span>
          <input
            type="text"
            value={mutationKey()}
            placeholder="feature.flag"
            onInput={(event) => {
              setMutationKey(event.currentTarget.value);
            }}
          />
        </label>

        <label class="filter-control">
          <span>Value (JSON or plain text)</span>
          <input
            type="text"
            value={mutationValue()}
            placeholder='{"enabled":true}'
            onInput={(event) => {
              setMutationValue(event.currentTarget.value);
            }}
          />
        </label>

        <div class="storage-mutation-actions">
          <button
            class="filter-reset"
            type="button"
            disabled={props.status() !== "open" || !canSetStorage()}
            onClick={() => {
              const key = mutationKey().trim();
              if (key.length === 0) {
                setMutationError("storage.set requires a non-empty key");
                return;
              }

              const sent = props.setStorageItem(
                mutationArea(),
                key,
                parseStorageMutationValue(mutationValue())
              );
              setMutationError(
                sent
                  ? null
                  : "unable to send storage.set while websocket is closed or payload is invalid"
              );
            }}
          >
            set
          </button>

          <button
            class="filter-reset"
            type="button"
            disabled={props.status() !== "open" || !canRemoveStorage()}
            onClick={() => {
              const key = mutationKey().trim();
              if (key.length === 0) {
                setMutationError("storage.remove requires a non-empty key");
                return;
              }

              const sent = props.removeStorageItem(mutationArea(), key);
              setMutationError(
                sent
                  ? null
                  : "unable to send storage.remove while websocket is closed or payload is invalid"
              );
            }}
          >
            remove
          </button>

          <button
            class="filter-reset"
            type="button"
            disabled={props.status() !== "open" || !canClearStorage()}
            onClick={() => {
              const sent = props.clearStorageArea(mutationArea());
              setMutationError(
                sent
                  ? null
                  : "unable to send storage.clear while websocket is closed or payload is invalid"
              );
            }}
          >
            clear area
          </button>
        </div>
      </div>

      <div class="placeholder-body">
        <p id="storage-filter-hint" class="filter-hint">
          Snapshot source: <code>query.storage.result</code> + live diffs from{" "}
          <code>storage.diff</code>
        </p>

        {props.status() === "open" ? (
          <div aria-live="polite" aria-atomic="true">
            {props.bridgeSession() && !canQueryStorage() ? (
              <p class="subtle">Storage snapshots were not negotiated for this session.</p>
            ) : null}
            {props.bridgeSession() && (!canSetStorage() || !canRemoveStorage() || !canClearStorage()) ? (
              <p class="subtle">
                Storage mutations only enable commands negotiated during hello.ack.
              </p>
            ) : null}
            {requestError() ? <p class="error">{requestError()}</p> : null}
            {mutationError() ? <p class="error">{mutationError()}</p> : null}
          </div>
        ) : (
          <p class="subtle" role="status" aria-live="polite">
            Storage queries run after the daemon websocket is open.
          </p>
        )}

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
              {rows().length === 0 ? (
                <tr>
                  <td colSpan="3" class="storage-empty">
                    No storage keys found for {selectedAreaLabel()}.
                  </td>
                </tr>
              ) : (
                rows().map((row) => (
                  <tr
                    class={props.storageHighlights().has(row.rowID) ? "storage-row-highlight" : ""}
                  >
                    <td>
                      <code>{row.area}</code>
                    </td>
                    <td>
                      <code>{row.key}</code>
                    </td>
                    <td>
                      <span class="storage-value">{truncateStorageValue(row.valueText)}</span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function parseStorageMutationValue(rawValue: string): unknown {
  const trimmed = rawValue.trim();
  if (trimmed.length === 0) {
    return "";
  }

  try {
    return JSON.parse(trimmed);
  } catch {
    return rawValue;
  }
}

function truncateStorageValue(value: string, maxLength = 240): string {
  if (value.length <= maxLength) {
    return value;
  }
  return `${value.slice(0, maxLength - 3)}...`;
}

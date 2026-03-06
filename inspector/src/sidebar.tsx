import h from "solid-js/h";
import type { Accessor, JSX } from "solid-js";

import type { ConnectionStatus } from "./connection";

interface SidebarProps {
  status: Accessor<ConnectionStatus>;
  socketURL: Accessor<string>;
  lastError: Accessor<string | null>;
}

export function Sidebar(props: SidebarProps): JSX.Element {
  return (
    <section class="sidebar-panel">
      <h2>Connection</h2>
      <p>
        status: <strong>{props.status()}</strong>
      </p>
      <p class="subtle">{props.socketURL()}</p>
      {props.lastError() ? <p class="error">{props.lastError()}</p> : null}

      <div class="sidebar-actions">
        <button class="filter-reset" type="button" disabled>
          reload (soon)
        </button>
        <button class="filter-reset" type="button" disabled>
          reinject (soon)
        </button>
      </div>
    </section>
  );
}

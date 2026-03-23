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
    <section class="sidebar-panel" aria-labelledby="inspector-connection-heading">
      <h2 id="inspector-connection-heading">Connection</h2>
      <p aria-live="polite" aria-atomic="true">
        status: <strong>{props.status()}</strong>
      </p>
      <p class="subtle">{props.socketURL()}</p>
      {props.lastError() ? (
        <p class="error" role="status" aria-live="polite">
          {props.lastError()}
        </p>
      ) : null}
    </section>
  );
}

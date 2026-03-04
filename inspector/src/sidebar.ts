import html from "solid-js/html";
import type { Accessor } from "solid-js";

import type { ConnectionStatus } from "./connection";

interface SidebarProps {
  status: Accessor<ConnectionStatus>;
  socketURL: Accessor<string>;
  lastError: Accessor<string | null>;
}

export function Sidebar(props: SidebarProps) {
  const error = () => {
    const value = props.lastError();
    if (!value) {
      return null;
    }
    return html`<p class="error">${value}</p>`;
  };

  return html`<section class="sidebar-panel">
    <h2>Connection</h2>
    <p>status: <strong>${props.status}</strong></p>
    <p class="subtle">${props.socketURL}</p>
    ${error}

    <div class="sidebar-actions">
      <button class="filter-reset" type="button" disabled>reload (soon)</button>
      <button class="filter-reset" type="button" disabled>reinject (soon)</button>
    </div>
  </section>`;
}

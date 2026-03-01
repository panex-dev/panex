import type { CommandReload, Envelope } from "./protocol";

export function isReloadCommand(envelope: Envelope): envelope is Envelope<CommandReload> {
  return envelope.name === "command.reload" && envelope.t === "command";
}

export function handleReloadCommand(envelope: Envelope, runtimeReload: () => void): boolean {
  if (!isReloadCommand(envelope)) {
    return false;
  }

  runtimeReload();
  return true;
}

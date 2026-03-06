import {
  isReloadCommand,
  type CommandReload,
  type Envelope
} from "@panex/protocol";

export { isReloadCommand };

export function handleReloadCommand(envelope: Envelope, runtimeReload: () => void): boolean {
  if (!isReloadCommand(envelope)) {
    return false;
  }

  runtimeReload();
  return true;
}

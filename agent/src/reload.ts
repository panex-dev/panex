import {
  isReloadCommand,
  type CommandReload,
  type Envelope
} from "@panex/protocol";

export { isReloadCommand };

export function handleReloadCommand(
  envelope: Envelope,
  extensionID: string,
  runtimeReload: () => void
): boolean {
  if (!isReloadCommand(envelope)) {
    return false;
  }
  if (!matchesTargetExtension(envelope.data, extensionID)) {
    return false;
  }

  runtimeReload();
  return true;
}

function matchesTargetExtension(data: CommandReload, extensionID: string): boolean {
  if (typeof data.extension_id !== "string" || data.extension_id.trim().length === 0) {
    return true
  }

  return data.extension_id === extensionID;
}

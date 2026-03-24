export interface ChromeSimBootstrapValues {
  daemonURL?: string;
  authToken?: string;
  extensionID?: string;
}

export interface ChromeSimEntrypointInjectionOptions {
  authToken?: string;
  daemonURL: string;
  extensionID?: string;
  moduleURL: string;
}

export interface ScriptLike {
  dataset: Record<string, string | undefined>;
  src: string;
  type: string;
}

export interface DocumentLike {
  createElement(tagName: "script"): ScriptLike;
  head?: {
    appendChild(node: ScriptLike): void;
  } | null;
  querySelector?(selector: string): ScriptLike | null;
  currentScript?: ScriptLike | null;
}

export interface WindowLike {
  document?: DocumentLike;
  location?: {
    search?: string;
  };
  __PANEX_DAEMON_TOKEN__?: string;
  __PANEX_DAEMON_URL__?: string;
  __PANEX_EXTENSION_ID__?: string;
}

export function resolveChromeSimBootstrapValues(win: WindowLike | undefined): ChromeSimBootstrapValues {
  if (!win) {
    return {};
  }

  const globalValues = {
    daemonURL: normalizeString(win.__PANEX_DAEMON_URL__),
    authToken: normalizeString(win.__PANEX_DAEMON_TOKEN__),
    extensionID: normalizeString(win.__PANEX_EXTENSION_ID__)
  };

  const scriptValues = resolveScriptBootstrapValues(win.document);
  const queryValues = resolveQueryBootstrapValues(win.location?.search);

  return {
    daemonURL: queryValues.daemonURL ?? scriptValues.daemonURL ?? globalValues.daemonURL,
    authToken: queryValues.authToken ?? scriptValues.authToken ?? globalValues.authToken,
    extensionID: queryValues.extensionID ?? scriptValues.extensionID ?? globalValues.extensionID
  };
}

export function injectChromeSimEntrypoint(
  doc: DocumentLike | undefined,
  options: ChromeSimEntrypointInjectionOptions
): ScriptLike | null {
  if (!doc || !doc.head) {
    return null;
  }

  const script = doc.createElement("script");
  script.type = "module";
  script.src = options.moduleURL;
  script.dataset.panexChromeSim = "1";
  script.dataset.panexWs = options.daemonURL;

  const extensionID = normalizeString(options.extensionID);
  if (extensionID) {
    script.dataset.panexExtensionId = extensionID;
  }

  doc.head.appendChild(script);
  return script;
}

function resolveScriptBootstrapValues(doc: DocumentLike | undefined): ChromeSimBootstrapValues {
  if (!doc) {
    return {};
  }

  const candidate = scriptCandidate(doc);
  if (!candidate || !isRecord(candidate.dataset)) {
    return {};
  }

  return {
    daemonURL: normalizeString(candidate.dataset.panexWs),
    extensionID: normalizeString(candidate.dataset.panexExtensionId)
  };
}

function scriptCandidate(doc: DocumentLike): ScriptLike | null {
  const current = doc.currentScript;
  if (current && hasChromeSimDataset(current)) {
    return current;
  }

  if (typeof doc.querySelector === "function") {
    return doc.querySelector("script[data-panex-chrome-sim]");
  }

  return null;
}

function hasChromeSimDataset(script: ScriptLike): boolean {
  if (!isRecord(script.dataset)) {
    return false;
  }

  return normalizeString(script.dataset.panexChromeSim) !== undefined;
}

function resolveQueryBootstrapValues(search: string | undefined): ChromeSimBootstrapValues {
  if (typeof search !== "string" || search.trim().length === 0) {
    return {};
  }

  const params = new URLSearchParams(search);
  return {
    daemonURL: normalizeString(params.get("ws")),
    authToken: normalizeString(params.get("token")),
    extensionID: normalizeString(params.get("extension_id"))
  };
}

function normalizeString(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

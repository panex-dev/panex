import {
  injectChromeSimEntrypoint,
  type ChromeSimEntrypointInjectionOptions,
  type ScriptLike
} from "../../shared/chrome-sim/src/bootstrap";

const closingHeadPattern = /<\/head>/i;
const existingInjectionPattern = /<script[^>]*data-panex-chrome-sim[^>]*>/i;

export function injectChromeSimIntoHTML(
  html: string,
  options: ChromeSimEntrypointInjectionOptions
): string {
  if (existingInjectionPattern.test(html)) {
    return html;
  }

  const insertionIndex = html.search(closingHeadPattern);
  if (insertionIndex < 0) {
    throw new Error("preview html is missing a </head> tag");
  }

  const scriptTag = chromeSimScriptTag(options);
  return `${html.slice(0, insertionIndex)}  ${scriptTag}\n${html.slice(insertionIndex)}`;
}

export function chromeSimScriptTag(options: ChromeSimEntrypointInjectionOptions): string {
  let appended: ScriptLike | null = null;
  const documentLike = {
    createElement() {
      return {
        dataset: {},
        src: "",
        type: ""
      };
    },
    head: {
      appendChild(node: ScriptLike) {
        appended = node;
      }
    }
  };

  const script = injectChromeSimEntrypoint(documentLike, options);
  if (!script || !appended) {
    throw new Error("failed to create chrome-sim entrypoint script");
  }

  return renderScriptTag(script);
}

function renderScriptTag(script: ScriptLike): string {
  const attrs: string[] = [];
  attrs.push(`type="${escapeAttr(script.type)}"`);
  attrs.push(`src="${escapeAttr(script.src)}"`);

  const dataKeys = Object.keys(script.dataset).sort();
  for (const key of dataKeys) {
    const value = script.dataset[key];
    if (typeof value !== "string" || value.length === 0) {
      continue;
    }
    attrs.push(`${datasetKeyToAttribute(key)}="${escapeAttr(value)}"`);
  }

  return `<script ${attrs.join(" ")}></script>`;
}

function datasetKeyToAttribute(key: string): string {
  return `data-${key.replace(/[A-Z]/g, (char) => `-${char.toLowerCase()}`)}`;
}

function escapeAttr(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("\"", "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

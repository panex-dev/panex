export type UnsupportedHandler = (path: string) => void;

export interface NamespaceRegistry {
  register(namespace: string, value: unknown): void;
  buildProxy(): Record<string, unknown>;
}

export function createNamespaceRegistry(onUnsupported?: UnsupportedHandler): NamespaceRegistry {
  const entries = new Map<string, unknown>();

  const register = (namespace: string, value: unknown) => {
    const normalized = namespace.trim();
    if (normalized.length === 0) {
      throw new Error("namespace is required");
    }
    entries.set(normalized, value);
  };

  const buildProxy = () => {
    const root = Object.create(null) as Record<string, unknown>;

    for (const [namespace, value] of entries.entries()) {
      const parts = namespace.split(".");
      let cursor: Record<string, unknown> = root;
      for (let index = 0; index < parts.length; index += 1) {
        const part = parts[index];
        const isLast = index === parts.length - 1;
        if (isLast) {
          cursor[part] = value;
          continue;
        }

        const existing = cursor[part];
        if (typeof existing === "object" && existing !== null) {
          cursor = existing as Record<string, unknown>;
          continue;
        }

        const next = Object.create(null) as Record<string, unknown>;
        cursor[part] = next;
        cursor = next;
      }
    }

    return new Proxy(root, {
      get(target, property, receiver) {
        if (typeof property === "string" && !(property in target)) {
          onUnsupported?.(property);
          return undefined;
        }
        return Reflect.get(target, property, receiver);
      }
    });
  };

  return { register, buildProxy };
}

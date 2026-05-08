import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { decode, encode } from "@msgpack/msgpack";

import { MAX_WEBSOCKET_MESSAGE_BYTES, PROTOCOL_VERSION, type Envelope } from "@panex/protocol";
import { createChromeSimTransport, type TransportSocket } from "../src/transport";

describe("chrome-sim transport", () => {
  it("defaults browser-facing transport connections to 127.0.0.1", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    assert.equal(sockets.length, 1);
    assert.equal(sockets[0]?.url, "ws://127.0.0.1:4317/ws");
    sockets[0]?.open();
    sockets[0]?.messageEnvelope(buildHelloAckEnvelope("sess-default"));
    await waitFor(() => sockets[0]!.sent.length >= 2);
    sockets[0]?.messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, {}));
    await pending;
    transport.close();
  });

  it("handshakes and resolves calls using correlated chrome.api.result envelopes", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      authToken: "dev-token",
      callIDFactory: () => "call-1",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get", ["theme"]);
    assert.equal(sockets.length, 1);
    const socket = sockets[0];
    socket.open();

    await waitFor(() => socket.sent.length >= 1);
    const hello = decodeEnvelope(socket.sent[0]);
    const helloData = hello.data as { auth_token?: string; extension_id?: string };
    assert.equal(hello.name, "hello");
    assert.equal(hello.t, "lifecycle");
    assert.equal(hello.src.role, "chrome-sim");
    assert.equal(helloData.auth_token, "dev-token");
    assert.equal(helloData.extension_id, "panex.simulated.extension");
    assert.deepEqual((hello.data as { capabilities_requested?: string[] }).capabilities_requested, [
      "chrome.api.call",
      "chrome.api.event",
      "storage.diff"
    ]);

    socket.messageEnvelope(buildHelloAckEnvelope("sess-1"));
    await waitFor(() => socket.sent.length >= 2);

    const call = decodeEnvelope(socket.sent[1]);
    assert.equal(call.name, "chrome.api.call");
    assert.equal(call.t, "command");
    assert.equal(call.src.role, "chrome-sim");
    assert.deepEqual(call.data, {
      call_id: "call-1",
      namespace: "storage.local",
      method: "get",
      args: ["theme"]
    });

    socket.messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, { theme: "dark" }));
    const result = await pending;
    assert.deepEqual(result, { theme: "dark" });
    assert.equal(transport.status(), "open");
    transport.close();
  });

  it("rejects pending calls on timeout", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      callTimeoutMS: 15,
      callIDFactory: () => "timeout-call",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get", ["missing"]);
    sockets[0].open();
    sockets[0].messageEnvelope(buildHelloAckEnvelope("sess-timeout"));

    await assert.rejects(async () => pending, /timed out/);
    transport.close();
  });

  it("rejects calls when chrome.api.result reports success=false", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      callIDFactory: () => "call-err",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "remove", ["x"]);
    sockets[0].open();
    sockets[0].messageEnvelope(buildHelloAckEnvelope("sess-err"));
    await waitFor(() => sockets[0].sent.length >= 2);

    sockets[0].messageEnvelope(
      buildChromeAPIResultEnvelope("call-err", false, undefined, "simulated failure")
    );
    await assert.rejects(async () => pending, /simulated failure/);
    transport.close();
  });

  it("rejects calls when hello.ack does not negotiate chrome.api.call", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    sockets[0].open();
    sockets[0].messageEnvelope(
      buildHelloAckEnvelope("sess-no-call", {
        capabilities_supported: ["chrome.api.event", "storage.diff"]
      })
    );

    await assert.rejects(async () => pending, /did not negotiate chrome\.api\.call/);
    assert.equal(sockets[0].sent.length, 1);
    transport.close();
  });

  it("reconnects after socket close and can process subsequent calls", async () => {
    const sockets: FakeSocket[] = [];
    let seq = 0;
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      callIDFactory: () => `call-${++seq}`,
      reconnectFloorMS: 1,
      reconnectCeilingMS: 2,
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const firstPending = transport.call("storage.local", "get");
    sockets[0].open();
    sockets[0].messageEnvelope(buildHelloAckEnvelope("sess-a"));
    await waitFor(() => sockets[0].sent.length >= 2);
    sockets[0].messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, { key: "a" }));
    assert.deepEqual(await firstPending, { key: "a" });

    sockets[0].close();
    await waitFor(() => sockets.length >= 2);
    sockets[1].open();
    sockets[1].messageEnvelope(buildHelloAckEnvelope("sess-b"));

    const secondPending = transport.call("storage.local", "get", ["key"]);
    await waitFor(() => sockets[1].sent.length >= 2);
    sockets[1].messageEnvelope(buildChromeAPIResultEnvelope("call-2", true, { key: "b" }));
    assert.deepEqual(await secondPending, { key: "b" });
    transport.close();
  });

  it("emits chrome.api.event envelopes to subscribers", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const received: Envelope[] = [];
    const unsubscribe = transport.subscribeEvents((event) => {
      received.push(event);
    });

    const pending = transport.call("storage.local", "get");
    sockets[0].open();
    sockets[0].messageEnvelope(buildHelloAckEnvelope("sess-events"));
    await waitFor(() => sockets[0].sent.length >= 2);
    sockets[0].messageEnvelope(buildChromeAPIEventEnvelope("storage.local", "changed", []));
    sockets[0].messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, {}));
    await pending;

    assert.equal(received.length, 1);
    assert.equal(received[0].name, "chrome.api.event");
    unsubscribe();
    transport.close();
  });

  it("rejects when daemon reports a mismatched protocol version", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      handshakeTimeoutMS: 100,
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    const socket = sockets[0];
    socket.open();

    const mismatchedAck: Envelope = {
      v: PROTOCOL_VERSION,
      t: "lifecycle",
      name: "hello.ack",
      src: { role: "daemon", id: "daemon-1" },
      data: {
        protocol_version: 99,
        daemon_version: "test",
        session_id: "sess-mismatch",
        auth_ok: true,
        capabilities_supported: ["chrome.api.call", "chrome.api.event"]
      }
    };

    socket.messageEnvelope(mismatchedAck);

    await assert.rejects(async () => pending, /protocol version mismatch/);
    transport.close();
  });

  it("closes the socket when an inbound websocket frame exceeds the browser-side limit", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      handshakeTimeoutMS: 25,
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    const socket = sockets[0];
    socket.open();
    socket.messageRaw(new Uint8Array(MAX_WEBSOCKET_MESSAGE_BYTES + 1));

    await assert.rejects(async () => pending, /closed before hello\.ack/);
    assert.deepEqual(socket.closeCalls, [{ code: 1009, reason: "message exceeds limit" }]);
    transport.close();
  });

  it("strips token query params from the websocket URL and keeps auth in hello", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws?foo=1&token=leak",
      authToken: "secret-token",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    const socket = sockets[0];
    assert.equal(socket.url, "ws://127.0.0.1:4317/ws?foo=1");
    socket.open();
    await waitFor(() => socket.sent.length >= 1);

    const hello = decodeEnvelope(socket.sent[0]);
    const helloData = hello.data as { auth_token?: string; extension_id?: string };
    assert.equal(helloData.auth_token, "secret-token");
    assert.equal(helloData.extension_id, "panex.simulated.extension");

    socket.messageEnvelope(buildHelloAckEnvelope("sess-auth"));
    await waitFor(() => socket.sent.length >= 2);
    socket.messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, {}));
    await pending;
    transport.close();
  });

  it("includes the configured extension id in the hello payload", async () => {
    const sockets: FakeSocket[] = [];
    const transport = createChromeSimTransport({
      daemonURL: "ws://127.0.0.1:4317/ws",
      extensionID: "popup",
      webSocketFactory: (url) => {
        const socket = new FakeSocket(url);
        sockets.push(socket);
        return socket;
      }
    });

    const pending = transport.call("storage.local", "get");
    const socket = sockets[0];
    socket.open();
    await waitFor(() => socket.sent.length >= 1);

    const hello = decodeEnvelope(socket.sent[0]);
    const helloData = hello.data as { extension_id?: string };
    assert.equal(helloData.extension_id, "popup");

    socket.messageEnvelope(buildHelloAckEnvelope("sess-popup"));
    await waitFor(() => socket.sent.length >= 2);
    socket.messageEnvelope(buildChromeAPIResultEnvelope("call-1", true, {}));
    await pending;
    transport.close();
  });
});

class FakeSocket implements TransportSocket {
  readonly url: string;
  readyState = 0;
  binaryType?: string;
  sent: Uint8Array[] = [];
  closeCalls: Array<{ code?: number; reason?: string }> = [];
  private listeners: Record<string, Array<(event: any) => void>> = {
    open: [],
    message: [],
    error: [],
    close: []
  };

  constructor(url: string) {
    this.url = url;
  }

  addEventListener(type: "open" | "message" | "error" | "close", listener: (event: any) => void) {
    this.listeners[type].push(listener);
  }

  send(data: Uint8Array) {
    if (this.readyState !== 1) {
      throw new Error("socket is not open");
    }
    this.sent.push(data);
  }

  close(code?: number, reason?: string) {
    this.closeCalls.push({ code, reason });
    if (this.readyState === 3) {
      return;
    }
    this.readyState = 3;
    this.emit("close", {});
  }

  open() {
    this.readyState = 1;
    this.emit("open", {});
  }

  messageEnvelope(envelope: Envelope) {
    this.emit("message", { data: encode(envelope) });
  }

  messageRaw(data: Uint8Array) {
    this.emit("message", { data });
  }

  private emit(type: "open" | "message" | "error" | "close", event: any) {
    for (const handler of this.listeners[type]) {
      handler(event);
    }
  }
}

function decodeEnvelope(raw: Uint8Array): Envelope {
  const value = decode(raw) as Envelope;
  return value;
}

function buildHelloAckEnvelope(
  sessionID: string,
  overrides: Partial<{
    protocol_version: number;
    daemon_version: string;
    session_id: string;
    auth_ok: boolean;
    capabilities_supported: string[];
  }> = {}
): Envelope {
  return {
    v: PROTOCOL_VERSION,
    t: "lifecycle",
    name: "hello.ack",
    src: { role: "daemon", id: "daemon-1" },
    data: {
      protocol_version: PROTOCOL_VERSION,
      daemon_version: "test",
      session_id: sessionID,
      auth_ok: true,
      capabilities_supported: ["chrome.api.call", "chrome.api.event", "storage.diff"],
      ...overrides
    }
  };
}

function buildChromeAPIResultEnvelope(
  callID: string,
  success: boolean,
  data?: unknown,
  error?: string
): Envelope {
  return {
    v: PROTOCOL_VERSION,
    t: "event",
    name: "chrome.api.result",
    src: { role: "daemon", id: "daemon-1" },
    data: {
      call_id: callID,
      success,
      data,
      error
    }
  };
}

function buildChromeAPIEventEnvelope(namespace: string, event: string, args: unknown[]): Envelope {
  return {
    v: PROTOCOL_VERSION,
    t: "event",
    name: "chrome.api.event",
    src: { role: "daemon", id: "daemon-1" },
    data: {
      namespace,
      event,
      args
    }
  };
}

async function waitFor(predicate: () => boolean, timeoutMS = 200): Promise<void> {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMS) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 1));
  }
  throw new Error("condition wait timed out");
}

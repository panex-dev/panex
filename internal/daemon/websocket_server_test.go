package daemon

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/panex-dev/panex/internal/protocol"
)

func TestWebSocketAuthRejectsMissingToken(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	_, resp, err := dialer.Dial(server.wsURL+"/ws", nil)
	if err == nil {
		t.Fatal("expected unauthorized handshake error, got nil")
	}
	if resp == nil {
		t.Fatal("expected HTTP response for failed handshake")
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestWebSocketHandshakeSendsHelloAckAndTracksConnection(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	helloAckEnv := mustHandshake(t, conn)
	if helloAckEnv.Name != protocol.MessageHelloAck {
		t.Fatalf("unexpected message name: got %q, want %q", helloAckEnv.Name, protocol.MessageHelloAck)
	}
	if helloAckEnv.T != protocol.TypeLifecycle {
		t.Fatalf("unexpected message type: got %q, want %q", helloAckEnv.T, protocol.TypeLifecycle)
	}
	if helloAckEnv.Src.Role != protocol.SourceDaemon {
		t.Fatalf("unexpected source role: got %q, want %q", helloAckEnv.Src.Role, protocol.SourceDaemon)
	}

	var helloAck protocol.HelloAck
	if err := protocol.DecodePayload(helloAckEnv.Data, &helloAck); err != nil {
		t.Fatalf("DecodePayload(hello.ack) returned error: %v", err)
	}
	if helloAck.ProtocolVersion != protocol.CurrentVersion {
		t.Fatalf("unexpected protocol version: got %d, want %d", helloAck.ProtocolVersion, protocol.CurrentVersion)
	}
	if helloAck.SessionID == "" {
		t.Fatal("expected non-empty session id")
	}
	if !helloAck.AuthOK {
		t.Fatal("expected auth_ok=true")
	}
	if helloAck.DaemonVersion != "test-version" {
		t.Fatalf("unexpected daemon version: got %q, want %q", helloAck.DaemonVersion, "test-version")
	}
	if len(helloAck.CapabilitiesSupported) != 1 || helloAck.CapabilitiesSupported[0] != "command.reload" {
		t.Fatalf("unexpected supported capabilities: %v", helloAck.CapabilitiesSupported)
	}

	waitForConnectionCount(t, server.ws, 1)

	if err := conn.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	waitForConnectionCount(t, server.ws, 0)
}

func TestWebSocketHandshakeNegotiatesCapabilities(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	hello := protocol.NewHello(
		protocol.Source{
			Role: protocol.SourceInspector,
			ID:   "inspector-1",
		},
		protocol.Hello{
			ProtocolVersion:       protocol.CurrentVersion,
			ClientKind:            "inspector",
			ClientVersion:         "dev",
			CapabilitiesRequested: []string{"query.events", "unknown.capability", "query.events"},
		},
	)
	rawHello, err := protocol.Encode(hello)
	if err != nil {
		t.Fatalf("Encode(hello) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawHello); err != nil {
		t.Fatalf("WriteMessage(hello) returned error: %v", err)
	}

	helloAck := mustReadEnvelope(t, conn)
	if helloAck.Name != protocol.MessageHelloAck {
		t.Fatalf("unexpected message name: got %q, want %q", helloAck.Name, protocol.MessageHelloAck)
	}

	var payload protocol.HelloAck
	if err := protocol.DecodePayload(helloAck.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(hello.ack) returned error: %v", err)
	}
	if len(payload.CapabilitiesSupported) != 1 || payload.CapabilitiesSupported[0] != "query.events" {
		t.Fatalf("unexpected supported capabilities: %v", payload.CapabilitiesSupported)
	}
}

func TestWebSocketBroadcastToConnectedClient(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	buildComplete := protocol.NewBuildComplete(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   "daemon-test",
		},
		protocol.BuildComplete{
			BuildID:      "build-1",
			Success:      true,
			DurationMS:   42,
			ChangedFiles: []string{"index.ts"},
		},
	)
	if err := server.ws.Broadcast(buildComplete); err != nil {
		t.Fatalf("Broadcast() returned error: %v", err)
	}

	_, rawMessage, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(broadcast) returned error: %v", err)
	}

	envelope, err := protocol.DecodeEnvelope(rawMessage)
	if err != nil {
		t.Fatalf("DecodeEnvelope(broadcast) returned error: %v", err)
	}
	if envelope.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected message name: got %q, want %q", envelope.Name, protocol.MessageBuildComplete)
	}

	var payload protocol.BuildComplete
	if err := protocol.DecodePayload(envelope.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(broadcast) returned error: %v", err)
	}
	if payload.BuildID != "build-1" {
		t.Fatalf("unexpected build id: got %q, want %q", payload.BuildID, "build-1")
	}
	if len(payload.ChangedFiles) != 1 || payload.ChangedFiles[0] != "index.ts" {
		t.Fatalf("unexpected changed files: %v", payload.ChangedFiles)
	}
}

func TestWebSocketRejectsFirstMessageThatIsNotHello(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	invalidFirst := protocol.NewBuildComplete(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   "daemon-1",
		},
		protocol.BuildComplete{
			BuildID:    "build-1",
			Success:    true,
			DurationMS: 42,
		},
	)
	rawInvalid, err := protocol.Encode(invalidFirst)
	if err != nil {
		t.Fatalf("Encode(invalidFirst) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawInvalid); err != nil {
		t.Fatalf("WriteMessage(invalidFirst) returned error: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close for invalid first message")
	}

	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected websocket.CloseError, got %T (%v)", err, err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("unexpected close code: got %d, want %d", closeErr.Code, websocket.ClosePolicyViolation)
	}

	waitForConnectionCount(t, server.ws, 0)
}

func TestWebSocketQueryEventsReturnsRecentStoredMessages(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	buildComplete := protocol.NewBuildComplete(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-test"},
		protocol.BuildComplete{
			BuildID:      "build-query",
			Success:      true,
			DurationMS:   42,
			ChangedFiles: []string{"index.ts"},
		},
	)
	reload := protocol.NewCommandReload(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-test"},
		protocol.CommandReload{
			Reason:  "build.complete",
			BuildID: "build-query",
		},
	)
	if err := server.ws.Broadcast(buildComplete); err != nil {
		t.Fatalf("Broadcast(build.complete) returned error: %v", err)
	}
	if err := server.ws.Broadcast(reload); err != nil {
		t.Fatalf("Broadcast(command.reload) returned error: %v", err)
	}

	// Drain direct broadcasts so the next read captures query response deterministically.
	_ = mustReadEnvelope(t, conn)
	_ = mustReadEnvelope(t, conn)

	query := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryEvents{Limit: 2},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.events) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.events) returned error: %v", err)
	}

	response := mustReadEnvelope(t, conn)
	if response.Name != protocol.MessageQueryResult {
		t.Fatalf("unexpected response name: got %q, want %q", response.Name, protocol.MessageQueryResult)
	}
	if response.T != protocol.TypeEvent {
		t.Fatalf("unexpected response type: got %q, want %q", response.T, protocol.TypeEvent)
	}

	var payload protocol.QueryEventsResult
	if err := protocol.DecodePayload(response.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.events.result) returned error: %v", err)
	}
	if len(payload.Events) != 2 {
		t.Fatalf("unexpected event count: got %d, want %d", len(payload.Events), 2)
	}
	if payload.Events[0].Envelope.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected first event: got %q, want %q", payload.Events[0].Envelope.Name, protocol.MessageBuildComplete)
	}
	if payload.Events[1].Envelope.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected second event: got %q, want %q", payload.Events[1].Envelope.Name, protocol.MessageCommandReload)
	}
}

func TestWebSocketQueryStorageReturnsAllAreasByDefault(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	response := mustReadEnvelope(t, conn)
	if response.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected response name: got %q, want %q", response.Name, protocol.MessageStorageResult)
	}
	if response.T != protocol.TypeEvent {
		t.Fatalf("unexpected response type: got %q, want %q", response.T, protocol.TypeEvent)
	}

	var payload protocol.QueryStorageResult
	if err := protocol.DecodePayload(response.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}
	if len(payload.Snapshots) != 3 {
		t.Fatalf("unexpected snapshot count: got %d, want %d", len(payload.Snapshots), 3)
	}

	wantAreas := []string{"local", "sync", "session"}
	for index, wantArea := range wantAreas {
		if payload.Snapshots[index].Area != wantArea {
			t.Fatalf("unexpected area at index %d: got %q, want %q", index, payload.Snapshots[index].Area, wantArea)
		}
		if len(payload.Snapshots[index].Items) != 0 {
			t.Fatalf("expected empty items for area %q, got %v", payload.Snapshots[index].Area, payload.Snapshots[index].Items)
		}
	}
}

func TestWebSocketQueryStorageRejectsUnsupportedArea(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "cookies"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage invalid area) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage invalid area) returned error: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close for invalid query.storage area")
	}

	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected websocket.CloseError, got %T (%v)", err, err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("unexpected close code: got %d, want %d", closeErr.Code, websocket.ClosePolicyViolation)
	}
}

type testServer struct {
	ws         *WebSocketServer
	httpServer *httptest.Server
	wsURL      string
	token      string
}

func newTestServer(t *testing.T) testServer {
	t.Helper()

	const token = "test-token"

	ws, err := NewWebSocketServer(WebSocketConfig{
		Port:           18080,
		AuthToken:      token,
		EventStorePath: filepath.Join(t.TempDir(), "events.db"),
		ServerVersion:  "test-version",
		DaemonID:       "daemon-test",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = ws.Close()
	})

	httpServer := httptest.NewServer(ws.Handler())
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	return testServer{
		ws:         ws,
		httpServer: httpServer,
		wsURL:      wsURL,
		token:      token,
	}
}

func dialAuthorizedConnection(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	conn, resp, err := dialer.Dial(wsURL+"/ws?token="+token, nil)
	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
	}
	if err != nil {
		t.Fatalf("Dial(authorized) returned error: %v", err)
	}

	return conn
}

func waitForConnectionCount(t *testing.T, server *WebSocketServer, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := server.ConnectionCount(); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for connection count %d (last=%d)", want, server.ConnectionCount())
}

func mustHandshake(t *testing.T, conn *websocket.Conn) protocol.Envelope {
	t.Helper()

	hello := protocol.NewHello(
		protocol.Source{
			Role: protocol.SourceDevAgent,
			ID:   "agent-1",
		},
		protocol.Hello{
			ProtocolVersion:       protocol.CurrentVersion,
			ClientKind:            "dev-agent",
			ClientVersion:         "dev",
			CapabilitiesRequested: []string{"command.reload"},
		},
	)
	rawHello, err := protocol.Encode(hello)
	if err != nil {
		t.Fatalf("Encode(hello) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawHello); err != nil {
		t.Fatalf("WriteMessage(hello) returned error: %v", err)
	}

	_, rawHelloAck, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(hello.ack) returned error: %v", err)
	}

	helloAckEnv, err := protocol.DecodeEnvelope(rawHelloAck)
	if err != nil {
		t.Fatalf("DecodeEnvelope(hello.ack) returned error: %v", err)
	}

	return helloAckEnv
}

func mustReadEnvelope(t *testing.T, conn *websocket.Conn) protocol.Envelope {
	t.Helper()

	_, rawMessage, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() returned error: %v", err)
	}

	envelope, err := protocol.DecodeEnvelope(rawMessage)
	if err != nil {
		t.Fatalf("DecodeEnvelope() returned error: %v", err)
	}

	return envelope
}

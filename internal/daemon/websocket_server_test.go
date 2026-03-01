package daemon

import (
	"net/http"
	"net/http/httptest"
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

func TestWebSocketHandshakeSendsWelcomeAndTracksConnection(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	welcomeEnv := mustHandshake(t, conn)
	if welcomeEnv.Name != protocol.MessageWelcome {
		t.Fatalf("unexpected message name: got %q, want %q", welcomeEnv.Name, protocol.MessageWelcome)
	}
	if welcomeEnv.T != protocol.TypeLifecycle {
		t.Fatalf("unexpected message type: got %q, want %q", welcomeEnv.T, protocol.TypeLifecycle)
	}
	if welcomeEnv.Src.Role != protocol.SourceDaemon {
		t.Fatalf("unexpected source role: got %q, want %q", welcomeEnv.Src.Role, protocol.SourceDaemon)
	}

	var welcome protocol.Welcome
	if err := protocol.DecodePayload(welcomeEnv.Data, &welcome); err != nil {
		t.Fatalf("DecodePayload(welcome) returned error: %v", err)
	}
	if welcome.ProtocolVersion != protocol.CurrentVersion {
		t.Fatalf("unexpected protocol version: got %d, want %d", welcome.ProtocolVersion, protocol.CurrentVersion)
	}
	if welcome.SessionID == "" {
		t.Fatal("expected non-empty session id")
	}
	if welcome.ServerVersion != "test-version" {
		t.Fatalf("unexpected server version: got %q, want %q", welcome.ServerVersion, "test-version")
	}

	waitForConnectionCount(t, server.ws, 1)

	if err := conn.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	waitForConnectionCount(t, server.ws, 0)
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
		Port:          18080,
		AuthToken:     token,
		ServerVersion: "test-version",
		DaemonID:      "daemon-test",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer() returned error: %v", err)
	}

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
			ProtocolVersion: protocol.CurrentVersion,
			Capabilities:    []string{"reload"},
		},
	)
	rawHello, err := protocol.Encode(hello)
	if err != nil {
		t.Fatalf("Encode(hello) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawHello); err != nil {
		t.Fatalf("WriteMessage(hello) returned error: %v", err)
	}

	_, rawWelcome, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(welcome) returned error: %v", err)
	}

	welcomeEnv, err := protocol.DecodeEnvelope(rawWelcome)
	if err != nil {
		t.Fatalf("DecodeEnvelope(welcome) returned error: %v", err)
	}

	return welcomeEnv
}

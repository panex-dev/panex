package daemon

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/panex-dev/panex/internal/protocol"
)

// TestIntegrationDaemonLifecycle exercises the full daemon lifecycle:
//
//  1. Start daemon with event store
//  2. Agent connects, sends hello, receives hello.ack with capabilities
//  3. Daemon broadcasts build.complete + command.reload to agent
//  4. Inspector connects, sends hello, receives hello.ack
//  5. Inspector queries events, receives timeline with persisted messages
//  6. Daemon mutates storage, inspector receives storage.diff
//  7. Inspector queries storage, receives current snapshot
//  8. Both clients disconnect cleanly
func TestIntegrationDaemonLifecycle(t *testing.T) {
	const token = "integration-token"

	ws, err := NewWebSocketServer(WebSocketConfig{
		Port:           18080,
		AuthToken:      token,
		EventStorePath: filepath.Join(t.TempDir(), "events.db"),
		ServerVersion:  "integration-test",
		DaemonID:       "daemon-integration",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer() returned error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Close() })

	httpServer := httptest.NewServer(ws.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	// --- Step 1: Agent connects and handshakes ---
	agentConn := dial(t, wsURL)
	t.Cleanup(func() { _ = agentConn.Close() })

	agentHelloAck := handshake(t, agentConn, protocol.Source{
		Role: protocol.SourceDevAgent,
		ID:   "agent-integration",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "dev-agent",
		ClientVersion:         "test",
		CapabilitiesRequested: []string{"command.reload"},
	})
	if !agentHelloAck.AuthOK {
		t.Fatal("agent hello.ack: expected auth_ok=true")
	}
	if agentHelloAck.SessionID == "" {
		t.Fatal("agent hello.ack: expected non-empty session_id")
	}

	waitForConnectionCount(t, ws, 1)

	// --- Step 2: Daemon broadcasts build.complete + command.reload ---
	buildComplete := protocol.NewBuildComplete(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-integration"},
		protocol.BuildComplete{
			BuildID:      "build-integration-1",
			Success:      true,
			DurationMS:   50,
			ChangedFiles: []string{"index.ts"},
		},
	)
	if err := ws.Broadcast(context.Background(), buildComplete); err != nil {
		t.Fatalf("Broadcast(build.complete) returned error: %v", err)
	}

	reload := protocol.NewCommandReload(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-integration"},
		protocol.CommandReload{Reason: "build.complete", BuildID: "build-integration-1"},
	)
	if err := ws.Broadcast(context.Background(), reload); err != nil {
		t.Fatalf("Broadcast(command.reload) returned error: %v", err)
	}

	// Agent should receive both broadcasts.
	buildEnv := readEnvelope(t, agentConn)
	if buildEnv.Name != protocol.MessageBuildComplete {
		t.Fatalf("agent: expected build.complete, got %q", buildEnv.Name)
	}
	reloadEnv := readEnvelope(t, agentConn)
	if reloadEnv.Name != protocol.MessageCommandReload {
		t.Fatalf("agent: expected command.reload, got %q", reloadEnv.Name)
	}

	// --- Step 3: Inspector connects and handshakes ---
	inspectorConn := dial(t, wsURL)
	t.Cleanup(func() { _ = inspectorConn.Close() })

	inspectorHelloAck := handshake(t, inspectorConn, protocol.Source{
		Role: protocol.SourceInspector,
		ID:   "inspector-integration",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "inspector",
		ClientVersion:         "test",
		CapabilitiesRequested: []string{"query.events", "query.storage", "storage.diff"},
	})
	if !inspectorHelloAck.AuthOK {
		t.Fatal("inspector hello.ack: expected auth_ok=true")
	}

	waitForConnectionCount(t, ws, 2)

	// --- Step 4: Inspector queries events (should see persisted history) ---
	query := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-integration"},
		protocol.QueryEvents{Limit: 50},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.events) returned error: %v", err)
	}
	if err := inspectorConn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.events) returned error: %v", err)
	}

	queryResult := readEnvelope(t, inspectorConn)
	if queryResult.Name != protocol.MessageQueryResult {
		t.Fatalf("inspector: expected query.events.result, got %q", queryResult.Name)
	}

	var eventsResult protocol.QueryEventsResult
	if err := protocol.DecodePayload(queryResult.Data, &eventsResult); err != nil {
		t.Fatalf("DecodePayload(query.events.result) returned error: %v", err)
	}
	// Should have at least: agent hello, hello.ack, build.complete, command.reload,
	// inspector hello, inspector hello.ack = 6 events minimum.
	if len(eventsResult.Events) < 6 {
		t.Fatalf("expected at least 6 persisted events, got %d", len(eventsResult.Events))
	}

	// Verify event types include the expected messages.
	nameSet := make(map[protocol.MessageName]bool)
	for _, event := range eventsResult.Events {
		nameSet[event.Envelope.Name] = true
	}
	for _, expected := range []protocol.MessageName{
		protocol.MessageHello,
		protocol.MessageHelloAck,
		protocol.MessageBuildComplete,
		protocol.MessageCommandReload,
	} {
		if !nameSet[expected] {
			t.Fatalf("expected %q in persisted events, got names: %v", expected, nameSet)
		}
	}

	// --- Step 5: Daemon mutates storage, inspector receives diff ---
	if err := ws.SetStorageItem(context.Background(), "local", "theme", "dark", "default"); err != nil {
		t.Fatalf("SetStorageItem returned error: %v", err)
	}

	// Both agent and inspector should receive the storage.diff broadcast.
	agentDiff := readEnvelope(t, agentConn)
	if agentDiff.Name != protocol.MessageStorageDiff {
		t.Fatalf("agent: expected storage.diff, got %q", agentDiff.Name)
	}
	inspectorDiff := readEnvelope(t, inspectorConn)
	if inspectorDiff.Name != protocol.MessageStorageDiff {
		t.Fatalf("inspector: expected storage.diff, got %q", inspectorDiff.Name)
	}

	var diffPayload protocol.StorageDiff
	if err := protocol.DecodePayload(inspectorDiff.Data, &diffPayload); err != nil {
		t.Fatalf("DecodePayload(storage.diff) returned error: %v", err)
	}
	if diffPayload.Area != "local" {
		t.Fatalf("unexpected diff area: got %q, want %q", diffPayload.Area, "local")
	}
	if len(diffPayload.Changes) != 1 || diffPayload.Changes[0].Key != "theme" {
		t.Fatalf("unexpected diff changes: %+v", diffPayload.Changes)
	}

	// --- Step 6: Inspector queries storage, receives snapshot ---
	storageQuery := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-integration"},
		protocol.QueryStorage{Area: "local"},
	)
	rawStorageQuery, err := protocol.Encode(storageQuery)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := inspectorConn.WriteMessage(websocket.BinaryMessage, rawStorageQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	storageResult := readEnvelope(t, inspectorConn)
	if storageResult.Name != protocol.MessageStorageResult {
		t.Fatalf("inspector: expected query.storage.result, got %q", storageResult.Name)
	}

	var storagePayload protocol.QueryStorageResult
	if err := protocol.DecodePayload(storageResult.Data, &storagePayload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}
	if len(storagePayload.Snapshots) != 1 {
		t.Fatalf("expected 1 storage snapshot, got %d", len(storagePayload.Snapshots))
	}
	if storagePayload.Snapshots[0].Area != "local" {
		t.Fatalf("unexpected snapshot area: %q", storagePayload.Snapshots[0].Area)
	}
	if storagePayload.Snapshots[0].Items["theme"] != "dark" {
		t.Fatalf("unexpected theme value: %v", storagePayload.Snapshots[0].Items["theme"])
	}

	// --- Step 7: Clean disconnect ---
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done")
	_ = agentConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	_ = inspectorConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))

	waitForConnectionCount(t, ws, 0)
}

func TestIntegrationStoragePersistsAcrossDaemonRestart(t *testing.T) {
	const token = "restart-token"

	storePath := filepath.Join(t.TempDir(), "events.db")
	first, err := NewWebSocketServer(WebSocketConfig{
		Port:           18081,
		AuthToken:      token,
		EventStorePath: storePath,
		ServerVersion:  "integration-test",
		DaemonID:       "daemon-restart-1",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer(first) returned error: %v", err)
	}

	if err := first.SetStorageItem(context.Background(), "local", "theme", "dark", "default"); err != nil {
		t.Fatalf("first.SetStorageItem(local theme) returned error: %v", err)
	}
	if err := first.SetStorageItem(context.Background(), "session", "counter", int64(7), "default"); err != nil {
		t.Fatalf("first.SetStorageItem(session counter) returned error: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first.Close() returned error: %v", err)
	}

	second, err := NewWebSocketServer(WebSocketConfig{
		Port:           18082,
		AuthToken:      token,
		EventStorePath: storePath,
		ServerVersion:  "integration-test",
		DaemonID:       "daemon-restart-2",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer(second) returned error: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	httpServer := httptest.NewServer(second.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	inspectorConn := dial(t, wsURL)
	t.Cleanup(func() { _ = inspectorConn.Close() })

	inspectorHelloAck := handshake(t, inspectorConn, protocol.Source{
		Role: protocol.SourceInspector,
		ID:   "inspector-restart",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "inspector",
		ClientVersion:         "test",
		CapabilitiesRequested: []string{"query.storage", "query.events"},
	})
	if !inspectorHelloAck.AuthOK {
		t.Fatal("inspector hello.ack after restart: expected auth_ok=true")
	}

	storageQuery := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-restart"},
		protocol.QueryStorage{},
	)
	rawStorageQuery, err := protocol.Encode(storageQuery)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := inspectorConn.WriteMessage(websocket.BinaryMessage, rawStorageQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	storageResult := readEnvelope(t, inspectorConn)
	if storageResult.Name != protocol.MessageStorageResult {
		t.Fatalf("expected query.storage.result after restart, got %q", storageResult.Name)
	}

	var storagePayload protocol.QueryStorageResult
	if err := protocol.DecodePayload(storageResult.Data, &storagePayload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) after restart returned error: %v", err)
	}
	if len(storagePayload.Snapshots) != 3 {
		t.Fatalf("expected 3 storage snapshots after restart, got %d", len(storagePayload.Snapshots))
	}
	if got := storagePayload.Snapshots[0].Items["theme"]; got != "dark" {
		t.Fatalf("unexpected persisted local theme after restart: %#v", got)
	}
	if got := storagePayload.Snapshots[2].Items["counter"]; got != int64(7) {
		t.Fatalf("unexpected persisted session counter after restart: %#v", got)
	}

	eventsQuery := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-restart"},
		protocol.QueryEvents{Limit: 10},
	)
	rawEventsQuery, err := protocol.Encode(eventsQuery)
	if err != nil {
		t.Fatalf("Encode(query.events) after restart returned error: %v", err)
	}
	if err := inspectorConn.WriteMessage(websocket.BinaryMessage, rawEventsQuery); err != nil {
		t.Fatalf("WriteMessage(query.events) after restart returned error: %v", err)
	}

	eventsResult := readEnvelope(t, inspectorConn)
	if eventsResult.Name != protocol.MessageQueryResult {
		t.Fatalf("expected query.events.result after restart, got %q", eventsResult.Name)
	}

	var eventsPayload protocol.QueryEventsResult
	if err := protocol.DecodePayload(eventsResult.Data, &eventsPayload); err != nil {
		t.Fatalf("DecodePayload(query.events.result) after restart returned error: %v", err)
	}
	storageDiffCount := 0
	for _, event := range eventsPayload.Events {
		if event.Envelope.Name == protocol.MessageStorageDiff {
			storageDiffCount++
		}
	}
	if storageDiffCount < 2 {
		t.Fatalf("expected persisted storage.diff history after restart, got %d matching events", storageDiffCount)
	}
}

func TestIntegrationTargetedReloadRoutesByExtensionID(t *testing.T) {
	const token = "routing-token"

	ws, err := NewWebSocketServer(WebSocketConfig{
		Port:           18083,
		AuthToken:      token,
		EventStorePath: filepath.Join(t.TempDir(), "events.db"),
		ServerVersion:  "integration-test",
		DaemonID:       "daemon-routing",
	})
	if err != nil {
		t.Fatalf("NewWebSocketServer() returned error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Close() })

	httpServer := httptest.NewServer(ws.Handler())
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	popupAgent := dial(t, wsURL)
	t.Cleanup(func() { _ = popupAgent.Close() })
	handshake(t, popupAgent, protocol.Source{
		Role: protocol.SourceDevAgent,
		ID:   "agent-popup",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "dev-agent",
		ClientVersion:         "test",
		ExtensionID:           "popup",
		CapabilitiesRequested: []string{"command.reload"},
	})
	waitForConnectionCount(t, ws, 1)

	adminAgent := dial(t, wsURL)
	t.Cleanup(func() { _ = adminAgent.Close() })
	handshake(t, adminAgent, protocol.Source{
		Role: protocol.SourceDevAgent,
		ID:   "agent-admin",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "dev-agent",
		ClientVersion:         "test",
		ExtensionID:           "admin",
		CapabilitiesRequested: []string{"command.reload"},
	})
	waitForConnectionCount(t, ws, 2)

	inspectorConn := dial(t, wsURL)
	t.Cleanup(func() { _ = inspectorConn.Close() })
	handshake(t, inspectorConn, protocol.Source{
		Role: protocol.SourceInspector,
		ID:   "inspector-routing",
	}, protocol.Hello{
		ProtocolVersion:       protocol.CurrentVersion,
		AuthToken:             token,
		ClientKind:            "inspector",
		ClientVersion:         "test",
		CapabilitiesRequested: []string{"query.events"},
	})
	waitForConnectionCount(t, ws, 3)

	buildComplete := protocol.NewBuildComplete(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-routing"},
		protocol.BuildComplete{
			BuildID:     "build-popup-1",
			Success:     true,
			DurationMS:  12,
			ExtensionID: "popup",
		},
	)
	if err := ws.Broadcast(context.Background(), buildComplete); err != nil {
		t.Fatalf("Broadcast(build.complete) returned error: %v", err)
	}

	reload := protocol.NewCommandReload(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-routing"},
		protocol.CommandReload{
			Reason:      "build.complete",
			BuildID:     "build-popup-1",
			ExtensionID: "popup",
		},
	)
	if err := ws.Broadcast(context.Background(), reload); err != nil {
		t.Fatalf("Broadcast(command.reload) returned error: %v", err)
	}

	if env := readEnvelope(t, popupAgent); env.Name != protocol.MessageBuildComplete {
		t.Fatalf("popup agent: expected build.complete, got %q", env.Name)
	}
	if env := readEnvelope(t, popupAgent); env.Name != protocol.MessageCommandReload {
		t.Fatalf("popup agent: expected command.reload, got %q", env.Name)
	}
	if env := readEnvelope(t, inspectorConn); env.Name != protocol.MessageBuildComplete {
		t.Fatalf("inspector: expected build.complete, got %q", env.Name)
	}
	if env := readEnvelope(t, inspectorConn); env.Name != protocol.MessageCommandReload {
		t.Fatalf("inspector: expected command.reload, got %q", env.Name)
	}

	assertNoEnvelope(t, adminAgent)
}

// --- Integration test helpers ---

func dial(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	conn, resp, err := dialer.Dial(wsURL+"/ws", nil)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		t.Fatalf("Dial() returned error: %v", err)
	}
	return conn
}

func handshake(t *testing.T, conn *websocket.Conn, src protocol.Source, hello protocol.Hello) protocol.HelloAck {
	t.Helper()
	msg := protocol.NewHello(src, hello)
	raw, err := protocol.Encode(msg)
	if err != nil {
		t.Fatalf("Encode(hello) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, raw); err != nil {
		t.Fatalf("WriteMessage(hello) returned error: %v", err)
	}
	env := readEnvelope(t, conn)
	if env.Name != protocol.MessageHelloAck {
		t.Fatalf("expected hello.ack, got %q", env.Name)
	}
	var ack protocol.HelloAck
	if err := protocol.DecodePayload(env.Data, &ack); err != nil {
		t.Fatalf("DecodePayload(hello.ack) returned error: %v", err)
	}
	return ack
}

func readEnvelope(t *testing.T, conn *websocket.Conn) protocol.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("ReadMessage() returned error: %v", err)
	}
	env, err := protocol.DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("DecodeEnvelope() returned error: %v", err)
	}
	return env
}

func assertNoEnvelope(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{})
	if err == nil {
		t.Fatal("expected no message, but a websocket frame was received")
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) && !strings.Contains(err.Error(), "i/o timeout") {
		t.Fatalf("expected read timeout, got %v", err)
	}
}

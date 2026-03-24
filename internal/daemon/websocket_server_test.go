package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/panex-dev/panex/internal/protocol"
	"github.com/panex-dev/panex/internal/store"
)

func TestWebSocketHandshakeRejectsMissingHelloAuthToken(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

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

	helloAckEnv := mustReadEnvelope(t, conn)
	if helloAckEnv.Name != protocol.MessageHelloAck {
		t.Fatalf("unexpected message name: got %q, want %q", helloAckEnv.Name, protocol.MessageHelloAck)
	}

	var helloAck protocol.HelloAck
	if err := protocol.DecodePayload(helloAckEnv.Data, &helloAck); err != nil {
		t.Fatalf("DecodePayload(hello.ack) returned error: %v", err)
	}
	if helloAck.AuthOK {
		t.Fatal("expected auth_ok=false")
	}
	if helloAck.SessionID != "" {
		t.Fatalf("expected empty session id, got %q", helloAck.SessionID)
	}
	if len(helloAck.CapabilitiesSupported) != 0 {
		t.Fatalf("expected no supported capabilities, got %v", helloAck.CapabilitiesSupported)
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close after unauthorized hello")
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
			AuthToken:             server.token,
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
			BuildID:         "build-1",
			Success:         true,
			DurationMS:      42,
			TriggeringFiles: []string{"index.ts"},
		},
	)
	if err := server.ws.Broadcast(context.Background(), buildComplete); err != nil {
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
	if len(payload.TriggeringFiles) != 1 || payload.TriggeringFiles[0] != "index.ts" {
		t.Fatalf("unexpected triggering files: %v", payload.TriggeringFiles)
	}
}

func TestWebSocketBroadcastUnregistersClosedSession(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	liveSessionID := singleSessionID(t, server.ws)

	server.ws.mu.RLock()
	session := server.ws.sessions[liveSessionID]
	server.ws.mu.RUnlock()
	if session == nil {
		t.Fatalf("expected session %q to be registered", liveSessionID)
	}
	if err := session.close(); err != nil {
		t.Fatalf("session.close() returned error: %v", err)
	}
	waitForConnectionCount(t, server.ws, 0)

	const staleSessionID = "stale-session"
	server.ws.register(staleSessionID, session, sessionMetadata{})
	waitForConnectionCount(t, server.ws, 1)

	buildComplete := protocol.NewBuildComplete(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   "daemon-test",
		},
		protocol.BuildComplete{
			BuildID:         "build-closed",
			Success:         true,
			DurationMS:      42,
			TriggeringFiles: []string{"index.ts"},
		},
	)
	err := server.ws.Broadcast(context.Background(), buildComplete)
	if err == nil {
		t.Fatal("expected Broadcast() to fail for closed session")
	}
	if !strings.Contains(err.Error(), staleSessionID) {
		t.Fatalf("expected Broadcast() error to mention %q, got %v", staleSessionID, err)
	}

	waitForConnectionCount(t, server.ws, 0)
}

func TestWebSocketCloseCancelsInFlightQueryEvents(t *testing.T) {
	eventStore := &blockingRecentEventStore{
		recentStarted:  make(chan struct{}),
		recentCanceled: make(chan struct{}),
	}
	server := newTestServerWithStore(t, eventStore)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	query := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryEvents{Limit: 1},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.events) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.events) returned error: %v", err)
	}

	select {
	case <-eventStore.recentStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-flight query.events call")
	}

	if err := server.ws.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	select {
	case <-eventStore.recentCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-flight query.events cancellation")
	}

	waitForConnectionCount(t, server.ws, 0)
}

func TestWebSocketPingKeepsResponsiveClientConnected(t *testing.T) {
	server := newTestServerWithConfig(t, WebSocketConfig{
		ReadTimeout:  250 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		PingInterval: 50 * time.Millisecond,
	})
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)
	waitForConnectionCount(t, server.ws, 1)

	if err := conn.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	<-readDone
}

func TestWebSocketClosesClientThatStopsRespondingToPing(t *testing.T) {
	server := newTestServerWithConfig(t, WebSocketConfig{
		ReadTimeout:  250 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		PingInterval: 50 * time.Millisecond,
	})
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	conn.SetPingHandler(func(string) error {
		return nil
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)
	waitForConnectionCount(t, server.ws, 0)

	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected connection close after ping timeout")
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
			BuildID:         "build-query",
			Success:         true,
			DurationMS:      42,
			TriggeringFiles: []string{"index.ts"},
		},
	)
	reload := protocol.NewCommandReload(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-test"},
		protocol.CommandReload{
			Reason:  "build.complete",
			BuildID: "build-query",
		},
	)
	if err := server.ws.Broadcast(context.Background(), buildComplete); err != nil {
		t.Fatalf("Broadcast(build.complete) returned error: %v", err)
	}
	if err := server.ws.Broadcast(context.Background(), reload); err != nil {
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
	if !payload.HasMore {
		t.Fatal("expected has_more=true when older handshake history remains")
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

func TestWebSocketQueryEventsSupportsBeforeIDPagination(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	source := protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-test"}
	for _, durationMS := range []int64{10, 20, 30} {
		if err := server.ws.Broadcast(context.Background(), protocol.NewBuildComplete(source, protocol.BuildComplete{
			BuildID:    "build-pagination",
			Success:    true,
			DurationMS: durationMS,
		})); err != nil {
			t.Fatalf("Broadcast(build.complete %d) returned error: %v", durationMS, err)
		}
		_ = mustReadEnvelope(t, conn)
	}

	firstQuery := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryEvents{Limit: 2},
	)
	rawFirstQuery, err := protocol.Encode(firstQuery)
	if err != nil {
		t.Fatalf("Encode(first query.events) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawFirstQuery); err != nil {
		t.Fatalf("WriteMessage(first query.events) returned error: %v", err)
	}

	firstResponse := mustReadEnvelope(t, conn)
	var firstPayload protocol.QueryEventsResult
	if err := protocol.DecodePayload(firstResponse.Data, &firstPayload); err != nil {
		t.Fatalf("DecodePayload(first query.events.result) returned error: %v", err)
	}
	if !firstPayload.HasMore {
		t.Fatal("expected has_more=true on first page")
	}
	if len(firstPayload.Events) != 2 {
		t.Fatalf("unexpected first page event count: got %d, want %d", len(firstPayload.Events), 2)
	}

	secondQuery := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryEvents{Limit: 2, BeforeID: firstPayload.Events[0].ID},
	)
	rawSecondQuery, err := protocol.Encode(secondQuery)
	if err != nil {
		t.Fatalf("Encode(second query.events) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawSecondQuery); err != nil {
		t.Fatalf("WriteMessage(second query.events) returned error: %v", err)
	}

	secondResponse := mustReadEnvelope(t, conn)
	var secondPayload protocol.QueryEventsResult
	if err := protocol.DecodePayload(secondResponse.Data, &secondPayload); err != nil {
		t.Fatalf("DecodePayload(second query.events.result) returned error: %v", err)
	}
	if !secondPayload.HasMore {
		t.Fatal("expected has_more=true on intermediate page")
	}
	if len(secondPayload.Events) != 2 {
		t.Fatalf("unexpected second page event count: got %d, want %d", len(secondPayload.Events), 2)
	}
	for _, event := range secondPayload.Events {
		if event.ID >= firstPayload.Events[0].ID {
			t.Fatalf("expected older event id than %d, got %d", firstPayload.Events[0].ID, event.ID)
		}
	}
	if secondPayload.Events[0].ID == secondPayload.Events[1].ID {
		t.Fatalf("expected distinct ids on second page, got %d twice", secondPayload.Events[0].ID)
	}

	foundOlderBuild := false
	for _, event := range secondPayload.Events {
		if event.Envelope.Name != protocol.MessageBuildComplete {
			continue
		}

		var payload protocol.BuildComplete
		if err := protocol.DecodePayload(event.Envelope.Data, &payload); err != nil {
			t.Fatalf("DecodePayload(older build.complete) returned error: %v", err)
		}
		if payload.DurationMS == 10 {
			foundOlderBuild = true
		}
	}
	if !foundOlderBuild {
		t.Fatal("expected intermediate page to include the oldest build.complete event")
	}

	thirdQuery := protocol.NewQueryEvents(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryEvents{Limit: 2, BeforeID: secondPayload.Events[0].ID},
	)
	rawThirdQuery, err := protocol.Encode(thirdQuery)
	if err != nil {
		t.Fatalf("Encode(third query.events) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawThirdQuery); err != nil {
		t.Fatalf("WriteMessage(third query.events) returned error: %v", err)
	}

	thirdResponse := mustReadEnvelope(t, conn)
	var thirdPayload protocol.QueryEventsResult
	if err := protocol.DecodePayload(thirdResponse.Data, &thirdPayload); err != nil {
		t.Fatalf("DecodePayload(third query.events.result) returned error: %v", err)
	}
	if thirdPayload.HasMore {
		t.Fatal("expected has_more=false on final page")
	}
	if len(thirdPayload.Events) != 1 {
		t.Fatalf("unexpected third page event count: got %d, want %d", len(thirdPayload.Events), 1)
	}
	if thirdPayload.Events[0].ID >= secondPayload.Events[0].ID {
		t.Fatalf("expected final page id older than %d, got %d", secondPayload.Events[0].ID, thirdPayload.Events[0].ID)
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

func TestWebSocketQueryStorageReturnsMutatedStateByArea(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	if err := server.ws.SetStorageItem(context.Background(), "local", "theme", "dark", "default"); err != nil {
		t.Fatalf("SetStorageItem(local theme) returned error: %v", err)
	}
	if err := server.ws.SetStorageItem(context.Background(), "sync", "beta", true, "default"); err != nil {
		t.Fatalf("SetStorageItem(sync beta) returned error: %v", err)
	}

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

	var payload protocol.QueryStorageResult
	if err := protocol.DecodePayload(response.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}

	localSnapshot := snapshotByArea(t, payload.Snapshots, "local")
	if got := localSnapshot.Items["theme"]; got != "dark" {
		t.Fatalf("unexpected local theme: got %#v, want %#v", got, "dark")
	}

	syncSnapshot := snapshotByArea(t, payload.Snapshots, "sync")
	if got := syncSnapshot.Items["beta"]; got != true {
		t.Fatalf("unexpected sync beta: got %#v, want %#v", got, true)
	}

	sessionSnapshot := snapshotByArea(t, payload.Snapshots, "session")
	if len(sessionSnapshot.Items) != 0 {
		t.Fatalf("expected empty session items, got %v", sessionSnapshot.Items)
	}
}

func TestWebSocketStorageMutationBroadcastsDiff(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	if err := server.ws.SetStorageItem(context.Background(), "local", "feature", "on", "default"); err != nil {
		t.Fatalf("SetStorageItem(local feature) returned error: %v", err)
	}

	diffEnvelope := mustReadEnvelope(t, conn)
	if diffEnvelope.Name != protocol.MessageStorageDiff {
		t.Fatalf("unexpected diff response name: got %q, want %q", diffEnvelope.Name, protocol.MessageStorageDiff)
	}

	var diffPayload protocol.StorageDiff
	if err := protocol.DecodePayload(diffEnvelope.Data, &diffPayload); err != nil {
		t.Fatalf("DecodePayload(storage.diff) returned error: %v", err)
	}
	if diffPayload.Area != "local" {
		t.Fatalf("unexpected diff area: got %q, want %q", diffPayload.Area, "local")
	}
	if len(diffPayload.Changes) != 1 {
		t.Fatalf("unexpected diff changes length: got %d, want %d", len(diffPayload.Changes), 1)
	}
	if diffPayload.Changes[0].Key != "feature" {
		t.Fatalf("unexpected diff key: got %q, want %q", diffPayload.Changes[0].Key, "feature")
	}
	if diffPayload.Changes[0].NewValue != "on" {
		t.Fatalf("unexpected diff new value: got %#v, want %#v", diffPayload.Changes[0].NewValue, "on")
	}
}

func TestWebSocketStorageSetCommandAppliesMutationAndBroadcastsDiff(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	setCommand := protocol.NewStorageSet(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.StorageSet{
			Area:  "local",
			Key:   "theme",
			Value: "dark",
		},
	)
	rawSet, err := protocol.Encode(setCommand)
	if err != nil {
		t.Fatalf("Encode(storage.set) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawSet); err != nil {
		t.Fatalf("WriteMessage(storage.set) returned error: %v", err)
	}

	diffEnvelope := mustReadEnvelope(t, conn)
	if diffEnvelope.Name != protocol.MessageStorageDiff {
		t.Fatalf("unexpected response name: got %q, want %q", diffEnvelope.Name, protocol.MessageStorageDiff)
	}

	var diff protocol.StorageDiff
	if err := protocol.DecodePayload(diffEnvelope.Data, &diff); err != nil {
		t.Fatalf("DecodePayload(storage.diff) returned error: %v", err)
	}
	if diff.Area != "local" {
		t.Fatalf("unexpected diff area: got %q, want %q", diff.Area, "local")
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("unexpected diff change count: got %d, want %d", len(diff.Changes), 1)
	}
	if diff.Changes[0].Key != "theme" {
		t.Fatalf("unexpected diff key: got %q, want %q", diff.Changes[0].Key, "theme")
	}
	if diff.Changes[0].NewValue != "dark" {
		t.Fatalf("unexpected diff new value: got %#v, want %#v", diff.Changes[0].NewValue, "dark")
	}

	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "local"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	result := mustReadEnvelope(t, conn)
	if result.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected response name: got %q, want %q", result.Name, protocol.MessageStorageResult)
	}

	var payload protocol.QueryStorageResult
	if err := protocol.DecodePayload(result.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}
	if len(payload.Snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d, want %d", len(payload.Snapshots), 1)
	}
	if got := payload.Snapshots[0].Items["theme"]; got != "dark" {
		t.Fatalf("unexpected local theme value: got %#v, want %#v", got, "dark")
	}
}

func TestWebSocketStorageRemoveCommandAppliesMutationAndBroadcastsDiff(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	if err := server.ws.SetStorageItem(context.Background(), "local", "theme", "dark", "default"); err != nil {
		t.Fatalf("SetStorageItem(local theme) returned error: %v", err)
	}

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	removeCommand := protocol.NewStorageRemove(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.StorageRemove{
			Area: "local",
			Key:  "theme",
		},
	)
	rawRemove, err := protocol.Encode(removeCommand)
	if err != nil {
		t.Fatalf("Encode(storage.remove) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawRemove); err != nil {
		t.Fatalf("WriteMessage(storage.remove) returned error: %v", err)
	}

	diffEnvelope := mustReadEnvelope(t, conn)
	if diffEnvelope.Name != protocol.MessageStorageDiff {
		t.Fatalf("unexpected response name: got %q, want %q", diffEnvelope.Name, protocol.MessageStorageDiff)
	}

	var diff protocol.StorageDiff
	if err := protocol.DecodePayload(diffEnvelope.Data, &diff); err != nil {
		t.Fatalf("DecodePayload(storage.diff) returned error: %v", err)
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("unexpected diff change count: got %d, want %d", len(diff.Changes), 1)
	}
	if diff.Changes[0].Key != "theme" {
		t.Fatalf("unexpected diff key: got %q, want %q", diff.Changes[0].Key, "theme")
	}
	if diff.Changes[0].OldValue != "dark" {
		t.Fatalf("unexpected diff old value: got %#v, want %#v", diff.Changes[0].OldValue, "dark")
	}

	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "local"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	result := mustReadEnvelope(t, conn)
	if result.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected response name: got %q, want %q", result.Name, protocol.MessageStorageResult)
	}

	var payload protocol.QueryStorageResult
	if err := protocol.DecodePayload(result.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}
	if len(payload.Snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d, want %d", len(payload.Snapshots), 1)
	}
	if _, exists := payload.Snapshots[0].Items["theme"]; exists {
		t.Fatalf("expected theme key to be removed, got %v", payload.Snapshots[0].Items)
	}
}

func TestWebSocketStorageClearCommandAppliesMutationAndBroadcastsDiff(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	if err := server.ws.SetStorageItem(context.Background(), "local", "one", 1, "default"); err != nil {
		t.Fatalf("SetStorageItem(local one) returned error: %v", err)
	}
	if err := server.ws.SetStorageItem(context.Background(), "local", "two", 2, "default"); err != nil {
		t.Fatalf("SetStorageItem(local two) returned error: %v", err)
	}

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	clearCommand := protocol.NewStorageClear(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.StorageClear{Area: "local"},
	)
	rawClear, err := protocol.Encode(clearCommand)
	if err != nil {
		t.Fatalf("Encode(storage.clear) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawClear); err != nil {
		t.Fatalf("WriteMessage(storage.clear) returned error: %v", err)
	}

	diffEnvelope := mustReadEnvelope(t, conn)
	if diffEnvelope.Name != protocol.MessageStorageDiff {
		t.Fatalf("unexpected response name: got %q, want %q", diffEnvelope.Name, protocol.MessageStorageDiff)
	}

	var diff protocol.StorageDiff
	if err := protocol.DecodePayload(diffEnvelope.Data, &diff); err != nil {
		t.Fatalf("DecodePayload(storage.diff) returned error: %v", err)
	}
	if len(diff.Changes) != 2 {
		t.Fatalf("unexpected diff change count: got %d, want %d", len(diff.Changes), 2)
	}
	if diff.Changes[0].Key != "one" || diff.Changes[1].Key != "two" {
		t.Fatalf("unexpected cleared keys: %v", diff.Changes)
	}

	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "local"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage) returned error: %v", err)
	}

	result := mustReadEnvelope(t, conn)
	if result.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected response name: got %q, want %q", result.Name, protocol.MessageStorageResult)
	}

	var payload protocol.QueryStorageResult
	if err := protocol.DecodePayload(result.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(query.storage.result) returned error: %v", err)
	}
	if len(payload.Snapshots) != 1 {
		t.Fatalf("unexpected snapshot count: got %d, want %d", len(payload.Snapshots), 1)
	}
	if len(payload.Snapshots[0].Items) != 0 {
		t.Fatalf("expected empty local area after clear, got %v", payload.Snapshots[0].Items)
	}
}

func TestWebSocketChromeAPICallStorageSetGetRemoveClear(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	setCommand := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-set-1",
			Namespace: "storage.local",
			Method:    "set",
			Args:      []any{map[string]any{"theme": "dark", "enabled": true}},
		},
	)
	rawSet, err := protocol.Encode(setCommand)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call set) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawSet); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call set) returned error: %v", err)
	}

	// set() mutates storage (emits diff) and then responds with chrome.api.result.
	_ = mustReadEnvelope(t, conn)
	_ = mustReadEnvelope(t, conn)
	setResultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 4)
	var setResult protocol.ChromeAPIResult
	if err := protocol.DecodePayload(setResultEnvelope.Data, &setResult); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result set) returned error: %v", err)
	}
	if !setResult.Success {
		t.Fatalf("expected set success=true, got false with error %q", setResult.Error)
	}
	if setResult.CallID != "call-set-1" {
		t.Fatalf("unexpected set call_id: got %q, want %q", setResult.CallID, "call-set-1")
	}

	getCommand := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-get-1",
			Namespace: "storage.local",
			Method:    "get",
			Args:      []any{"theme"},
		},
	)
	rawGet, err := protocol.Encode(getCommand)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call get) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawGet); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call get) returned error: %v", err)
	}

	getResultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var getResult protocol.ChromeAPIResult
	if err := protocol.DecodePayload(getResultEnvelope.Data, &getResult); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result get) returned error: %v", err)
	}
	if !getResult.Success {
		t.Fatalf("expected get success=true, got false with error %q", getResult.Error)
	}
	getData := mustMapStringAny(t, getResult.Data)
	if got := getData["theme"]; got != "dark" {
		t.Fatalf("unexpected get result theme: got %#v, want %#v", got, "dark")
	}

	removeCommand := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-remove-1",
			Namespace: "storage.local",
			Method:    "remove",
			Args:      []any{"enabled"},
		},
	)
	rawRemove, err := protocol.Encode(removeCommand)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call remove) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawRemove); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call remove) returned error: %v", err)
	}

	removeDiff := mustReadEnvelopeByName(t, conn, protocol.MessageStorageDiff, 2)
	var removeDiffPayload protocol.StorageDiff
	if err := protocol.DecodePayload(removeDiff.Data, &removeDiffPayload); err != nil {
		t.Fatalf("DecodePayload(storage.diff remove) returned error: %v", err)
	}
	if len(removeDiffPayload.Changes) != 1 || removeDiffPayload.Changes[0].Key != "enabled" {
		t.Fatalf("unexpected remove diff payload: %+v", removeDiffPayload)
	}
	removeResult := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var removeResultPayload protocol.ChromeAPIResult
	if err := protocol.DecodePayload(removeResult.Data, &removeResultPayload); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result remove) returned error: %v", err)
	}
	if !removeResultPayload.Success {
		t.Fatalf("expected remove success=true, got false with error %q", removeResultPayload.Error)
	}

	clearCommand := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-clear-1",
			Namespace: "storage.local",
			Method:    "clear",
		},
	)
	rawClear, err := protocol.Encode(clearCommand)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call clear) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawClear); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call clear) returned error: %v", err)
	}

	clearDiff := mustReadEnvelopeByName(t, conn, protocol.MessageStorageDiff, 2)
	var clearDiffPayload protocol.StorageDiff
	if err := protocol.DecodePayload(clearDiff.Data, &clearDiffPayload); err != nil {
		t.Fatalf("DecodePayload(storage.diff clear) returned error: %v", err)
	}
	if len(clearDiffPayload.Changes) != 1 || clearDiffPayload.Changes[0].Key != "theme" {
		t.Fatalf("unexpected clear diff payload: %+v", clearDiffPayload)
	}
	clearResult := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var clearResultPayload protocol.ChromeAPIResult
	if err := protocol.DecodePayload(clearResult.Data, &clearResultPayload); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result clear) returned error: %v", err)
	}
	if !clearResultPayload.Success {
		t.Fatalf("expected clear success=true, got false with error %q", clearResultPayload.Error)
	}
}

func TestWebSocketChromeAPICallGetBytesInUseAndDefaults(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	if err := server.ws.SetStorageItem(context.Background(), "sync", "theme", "dark", "default"); err != nil {
		t.Fatalf("SetStorageItem(sync theme) returned error: %v", err)
	}

	bytesCommand := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-bytes-1",
			Namespace: "storage.sync",
			Method:    "getBytesInUse",
			Args:      []any{[]any{"theme"}},
		},
	)
	rawBytes, err := protocol.Encode(bytesCommand)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call getBytesInUse) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawBytes); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call getBytesInUse) returned error: %v", err)
	}

	bytesResultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var bytesResult protocol.ChromeAPIResult
	if err := protocol.DecodePayload(bytesResultEnvelope.Data, &bytesResult); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result getBytesInUse) returned error: %v", err)
	}
	if !bytesResult.Success {
		t.Fatalf("expected getBytesInUse success=true, got false with error %q", bytesResult.Error)
	}
	if mustInt64(t, bytesResult.Data) <= 0 {
		t.Fatalf("expected getBytesInUse > 0, got %#v", bytesResult.Data)
	}

	defaultsGet := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-defaults-1",
			Namespace: "storage.sync",
			Method:    "get",
			Args:      []any{map[string]any{"theme": "light", "missing": "fallback"}},
		},
	)
	rawDefaultsGet, err := protocol.Encode(defaultsGet)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call get defaults) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawDefaultsGet); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call get defaults) returned error: %v", err)
	}

	defaultsResultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var defaultsResult protocol.ChromeAPIResult
	if err := protocol.DecodePayload(defaultsResultEnvelope.Data, &defaultsResult); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result get defaults) returned error: %v", err)
	}
	if !defaultsResult.Success {
		t.Fatalf("expected defaults get success=true, got false with error %q", defaultsResult.Error)
	}
	defaultsData := mustMapStringAny(t, defaultsResult.Data)
	if got := defaultsData["theme"]; got != "dark" {
		t.Fatalf("unexpected defaults get theme: got %#v, want %#v", got, "dark")
	}
	if got := defaultsData["missing"]; got != "fallback" {
		t.Fatalf("unexpected defaults get missing: got %#v, want %#v", got, "fallback")
	}
}

func TestWebSocketChromeAPICallRuntimeSendMessageBroadcastsEvent(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-runtime-send-1",
			Namespace: "runtime",
			Method:    "sendMessage",
			Args:      []any{map[string]any{"topic": "ping", "id": 7}},
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call runtime.sendMessage) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call runtime.sendMessage) returned error: %v", err)
	}

	eventEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIEvent, 3)
	var event protocol.ChromeAPIEvent
	if err := protocol.DecodePayload(eventEnvelope.Data, &event); err != nil {
		t.Fatalf("DecodePayload(chrome.api.event runtime.onMessage) returned error: %v", err)
	}
	if event.Namespace != "runtime" || event.Event != "onMessage" {
		t.Fatalf("unexpected runtime event payload: %+v", event)
	}
	if len(event.Args) != 1 {
		t.Fatalf("unexpected runtime event args: got %d, want %d", len(event.Args), 1)
	}
	eventPayload := mustMapStringAny(t, event.Args[0])
	if got := eventPayload["topic"]; got != "ping" {
		t.Fatalf("unexpected runtime event topic: got %#v, want %#v", got, "ping")
	}

	resultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 3)
	var result protocol.ChromeAPIResult
	if err := protocol.DecodePayload(resultEnvelope.Data, &result); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result runtime.sendMessage) returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected runtime.sendMessage success=true, got false with error %q", result.Error)
	}
	if result.CallID != "call-runtime-send-1" {
		t.Fatalf("unexpected runtime.sendMessage call_id: got %q", result.CallID)
	}
	dataPayload := mustMapStringAny(t, result.Data)
	if got := dataPayload["id"]; mustInt64(t, got) != 7 {
		t.Fatalf("unexpected runtime.sendMessage data id: got %#v, want %#v", got, int64(7))
	}
}

func TestWebSocketChromeAPICallRuntimeSendMessageRequiresMessageArg(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-runtime-send-empty",
			Namespace: "runtime",
			Method:    "sendMessage",
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call runtime.sendMessage empty) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call runtime.sendMessage empty) returned error: %v", err)
	}

	resultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var result protocol.ChromeAPIResult
	if err := protocol.DecodePayload(resultEnvelope.Data, &result); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result runtime.sendMessage empty) returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected runtime.sendMessage without args success=false, got true")
	}
	if !strings.Contains(result.Error, "expects a message argument") {
		t.Fatalf("unexpected runtime.sendMessage empty error: %q", result.Error)
	}
}

func TestWebSocketChromeAPICallTabsQueryReturnsFilteredTabs(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-tabs-query-1",
			Namespace: "tabs",
			Method:    "query",
			Args:      []any{map[string]any{"active": true, "currentWindow": true}},
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call tabs.query) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call tabs.query) returned error: %v", err)
	}

	resultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var result protocol.ChromeAPIResult
	if err := protocol.DecodePayload(resultEnvelope.Data, &result); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result tabs.query) returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected tabs.query success=true, got false with error %q", result.Error)
	}
	tabs, ok := result.Data.([]any)
	if !ok {
		t.Fatalf("expected tabs.query data []any, got %T (%#v)", result.Data, result.Data)
	}
	if len(tabs) != 1 {
		t.Fatalf("unexpected tabs.query result count: got %d, want %d", len(tabs), 1)
	}

	tab, ok := tabs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tabs.query[0] map payload, got %T (%#v)", tabs[0], tabs[0])
	}
	if got := mustInt64(t, tab["id"]); got != 1 {
		t.Fatalf("unexpected tabs.query[0].id: got %d, want %d", got, 1)
	}
	if got, ok := tab["active"].(bool); !ok || !got {
		t.Fatalf("expected tabs.query[0].active=true, got %#v", tab["active"])
	}
	if got, ok := tab["currentWindow"].(bool); !ok || !got {
		t.Fatalf("expected tabs.query[0].currentWindow=true, got %#v", tab["currentWindow"])
	}
}

func TestWebSocketChromeAPICallTabsQueryInvalidFilterReturnsFailureResult(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-tabs-query-invalid",
			Namespace: "tabs",
			Method:    "query",
			Args:      []any{map[string]any{"active": "yes"}},
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call tabs.query invalid) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call tabs.query invalid) returned error: %v", err)
	}

	resultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var result protocol.ChromeAPIResult
	if err := protocol.DecodePayload(resultEnvelope.Data, &result); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result tabs.query invalid) returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected tabs.query invalid filter success=false, got true with data %#v", result.Data)
	}
	if !strings.Contains(result.Error, "active filter must be boolean") {
		t.Fatalf("unexpected tabs.query invalid filter error: %q", result.Error)
	}

	// Connection remains open after simulator-level validation errors.
	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "local"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage follow-up tabs invalid) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage follow-up tabs invalid) returned error: %v", err)
	}
	followUp := mustReadEnvelopeByName(t, conn, protocol.MessageStorageResult, 2)
	if followUp.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected follow-up message name: got %q, want %q", followUp.Name, protocol.MessageStorageResult)
	}
}

func TestWebSocketChromeAPICallUnsupportedNamespaceReturnsFailureResult(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    "call-bad-namespace",
			Namespace: "bookmarks",
			Method:    "search",
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call unsupported namespace) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call unsupported namespace) returned error: %v", err)
	}

	resultEnvelope := mustReadEnvelopeByName(t, conn, protocol.MessageChromeAPIResult, 2)
	var result protocol.ChromeAPIResult
	if err := protocol.DecodePayload(resultEnvelope.Data, &result); err != nil {
		t.Fatalf("DecodePayload(chrome.api.result unsupported namespace) returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected unsupported namespace success=false, got true with data %#v", result.Data)
	}
	if !strings.Contains(result.Error, "unsupported chrome namespace") {
		t.Fatalf("unexpected unsupported namespace error: %q", result.Error)
	}

	// Connection remains open after unsupported simulator call; verify with a follow-up query.
	query := protocol.NewQueryStorage(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.QueryStorage{Area: "local"},
	)
	rawQuery, err := protocol.Encode(query)
	if err != nil {
		t.Fatalf("Encode(query.storage follow-up) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawQuery); err != nil {
		t.Fatalf("WriteMessage(query.storage follow-up) returned error: %v", err)
	}
	followUp := mustReadEnvelopeByName(t, conn, protocol.MessageStorageResult, 2)
	if followUp.Name != protocol.MessageStorageResult {
		t.Fatalf("unexpected follow-up message name: got %q, want %q", followUp.Name, protocol.MessageStorageResult)
	}
}

func TestWebSocketChromeAPICallMissingCallIDClosesConnection(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	command := protocol.NewChromeAPICall(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.ChromeAPICall{
			CallID:    " ",
			Namespace: "storage.local",
			Method:    "get",
		},
	)
	rawCommand, err := protocol.Encode(command)
	if err != nil {
		t.Fatalf("Encode(chrome.api.call missing call_id) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawCommand); err != nil {
		t.Fatalf("WriteMessage(chrome.api.call missing call_id) returned error: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close for missing chrome.api.call call_id")
	}

	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected websocket.CloseError, got %T (%v)", err, err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("unexpected close code: got %d, want %d", closeErr.Code, websocket.ClosePolicyViolation)
	}
}

func TestWebSocketStorageMutationValidation(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	if err := server.ws.SetStorageItem(context.Background(), "cookies", "x", "1", "default"); err == nil {
		t.Fatal("expected SetStorageItem invalid area error, got nil")
	}
	if err := server.ws.SetStorageItem(context.Background(), "local", "", "1", "default"); err == nil {
		t.Fatal("expected SetStorageItem empty key error, got nil")
	}
	if err := server.ws.RemoveStorageItem(context.Background(), "cookies", "x", "default"); err == nil {
		t.Fatal("expected RemoveStorageItem invalid area error, got nil")
	}
	if err := server.ws.RemoveStorageItem(context.Background(), "local", "", "default"); err == nil {
		t.Fatal("expected RemoveStorageItem empty key error, got nil")
	}
	if err := server.ws.ClearStorageArea(context.Background(), "cookies", "default"); err == nil {
		t.Fatal("expected ClearStorageArea invalid area error, got nil")
	}
}

func TestWebSocketStorageCommandRejectsInvalidPayload(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	conn := dialAuthorizedConnection(t, server.wsURL, server.token)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	_ = mustHandshake(t, conn)
	waitForConnectionCount(t, server.ws, 1)

	invalidCommand := protocol.NewStorageSet(
		protocol.Source{Role: protocol.SourceInspector, ID: "inspector-1"},
		protocol.StorageSet{
			Area:  "cookies",
			Key:   "theme",
			Value: "dark",
		},
	)
	rawInvalid, err := protocol.Encode(invalidCommand)
	if err != nil {
		t.Fatalf("Encode(storage.set invalid area) returned error: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, rawInvalid); err != nil {
		t.Fatalf("WriteMessage(storage.set invalid area) returned error: %v", err)
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected connection close for invalid storage.set area")
	}

	closeErr, ok := err.(*websocket.CloseError)
	if !ok {
		t.Fatalf("expected websocket.CloseError, got %T (%v)", err, err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("unexpected close code: got %d, want %d", closeErr.Code, websocket.ClosePolicyViolation)
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

func TestIsLocalOrigin(t *testing.T) {
	testCases := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "no origin header", origin: "", want: true},
		{name: "localhost http", origin: "http://localhost", want: true},
		{name: "localhost with port", origin: "http://localhost:4317", want: true},
		{name: "127.0.0.1", origin: "http://127.0.0.1", want: true},
		{name: "127.0.0.1 with port", origin: "http://127.0.0.1:8080", want: true},
		{name: "ipv6 loopback", origin: "http://[::1]", want: true},
		{name: "ipv6 loopback with port", origin: "http://[::1]:4317", want: true},
		{name: "remote host", origin: "http://evil.com", want: false},
		{name: "remote ip", origin: "http://192.168.1.1", want: false},
		{name: "invalid url", origin: "://bad", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{Header: http.Header{}}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := isLocalOrigin(r); got != tc.want {
				t.Fatalf("isLocalOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestWebSocketRejectsNonLocalOrigin(t *testing.T) {
	server := newTestServer(t)
	defer server.httpServer.Close()

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	_, resp, err := dialer.Dial(
		server.wsURL+"/ws",
		http.Header{"Origin": []string{"http://evil.com"}},
	)
	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
	}
	if err == nil {
		t.Fatal("expected connection to be rejected for non-local origin")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %v", resp)
	}
}

func snapshotByArea(
	t *testing.T,
	snapshots []protocol.StorageSnapshot,
	area string,
) protocol.StorageSnapshot {
	t.Helper()

	for _, snapshot := range snapshots {
		if snapshot.Area == area {
			return snapshot
		}
	}

	t.Fatalf("missing storage snapshot for area %q in %v", area, snapshots)
	return protocol.StorageSnapshot{}
}

type testServer struct {
	ws         *WebSocketServer
	httpServer *httptest.Server
	wsURL      string
	token      string
}

func newTestServer(t *testing.T) testServer {
	t.Helper()

	return newTestServerWithConfigAndStore(t, WebSocketConfig{}, nil)
}

func newTestServerWithConfig(t *testing.T, cfg WebSocketConfig) testServer {
	t.Helper()

	return newTestServerWithConfigAndStore(t, cfg, nil)
}

func newTestServerWithStore(t *testing.T, testEventStore eventStore) testServer {
	t.Helper()

	return newTestServerWithConfigAndStore(t, WebSocketConfig{}, testEventStore)
}

func newTestServerWithConfigAndStore(
	t *testing.T,
	cfg WebSocketConfig,
	testEventStore eventStore,
) testServer {
	t.Helper()

	const token = "test-token"

	cfg.Port = 18080
	cfg.AuthToken = token
	cfg.EventStorePath = filepath.Join(t.TempDir(), "events.db")
	cfg.ServerVersion = "test-version"
	cfg.DaemonID = "daemon-test"

	ws, err := NewWebSocketServer(cfg)
	if err != nil {
		t.Fatalf("NewWebSocketServer() returned error: %v", err)
	}
	if testEventStore != nil {
		if err := ws.eventStore.Close(); err != nil {
			t.Fatalf("close sqlite event store before test override: %v", err)
		}
		ws.eventStore = testEventStore
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

const defaultHandshakeToken = "test-token"

func dialAuthorizedConnection(t *testing.T, wsURL, _ string) *websocket.Conn {
	t.Helper()

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	conn, resp, err := dialer.Dial(wsURL+"/ws", nil)
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

func singleSessionID(t *testing.T, server *WebSocketServer) string {
	t.Helper()

	server.mu.RLock()
	defer server.mu.RUnlock()

	if len(server.sessions) != 1 {
		t.Fatalf("expected exactly one session, got %d", len(server.sessions))
	}
	for sessionID := range server.sessions {
		return sessionID
	}

	t.Fatal("expected one registered session")
	return ""
}

func mustHandshake(t *testing.T, conn *websocket.Conn) protocol.Envelope {
	return mustHandshakeWithToken(t, conn, defaultHandshakeToken)
}

func mustHandshakeWithToken(t *testing.T, conn *websocket.Conn, token string) protocol.Envelope {
	t.Helper()

	hello := protocol.NewHello(
		protocol.Source{
			Role: protocol.SourceDevAgent,
			ID:   "agent-1",
		},
		protocol.Hello{
			ProtocolVersion:       protocol.CurrentVersion,
			AuthToken:             token,
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

func mustReadEnvelopeByName(
	t *testing.T,
	conn *websocket.Conn,
	name protocol.MessageName,
	maxReads int,
) protocol.Envelope {
	t.Helper()

	if maxReads < 1 {
		maxReads = 1
	}

	for index := 0; index < maxReads; index++ {
		envelope := mustReadEnvelope(t, conn)
		if envelope.Name == name {
			return envelope
		}
	}

	t.Fatalf("did not receive envelope %q within %d reads", name, maxReads)
	return protocol.Envelope{}
}

func mustMapStringAny(t *testing.T, value any) map[string]any {
	t.Helper()

	record, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result payload, got %T (%#v)", value, value)
	}
	return record
}

func mustInt64(t *testing.T, value any) int64 {
	t.Helper()

	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	default:
		t.Fatalf("expected integer numeric payload, got %T (%#v)", value, value)
		return 0
	}
}

type blockingRecentEventStore struct {
	recentStarted  chan struct{}
	recentCanceled chan struct{}
}

func (s *blockingRecentEventStore) Append(context.Context, protocol.Envelope, string) error {
	return nil
}

func (s *blockingRecentEventStore) Recent(ctx context.Context, _ int, _ int64, _ string) ([]store.Record, bool, error) {
	close(s.recentStarted)
	<-ctx.Done()
	close(s.recentCanceled)
	return nil, false, ctx.Err()
}

func (s *blockingRecentEventStore) StorageSnapshots(context.Context, string, string) ([]protocol.StorageSnapshot, error) {
	return []protocol.StorageSnapshot{{Area: "local", Items: map[string]any{}}}, nil
}

func (s *blockingRecentEventStore) SetStorageItem(
	context.Context,
	protocol.Source,
	string,
	string,
	any,
	string,
) (protocol.Envelope, error) {
	return protocol.Envelope{}, nil
}

func (s *blockingRecentEventStore) RemoveStorageItem(
	context.Context,
	protocol.Source,
	string,
	string,
	string,
) (protocol.Envelope, bool, error) {
	return protocol.Envelope{}, false, nil
}

func (s *blockingRecentEventStore) ClearStorageArea(
	context.Context,
	protocol.Source,
	string,
	string,
) (protocol.Envelope, bool, error) {
	return protocol.Envelope{}, false, nil
}

func (s *blockingRecentEventStore) Close() error {
	return nil
}

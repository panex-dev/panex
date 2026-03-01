package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/panex-dev/panex/internal/protocol"
	"github.com/panex-dev/panex/internal/store"
)

const maxMessageBytes = 1 << 20 // 1 MiB keeps accidental payload explosions bounded.

type WebSocketConfig struct {
	Port           int
	AuthToken      string
	EventStorePath string
	ServerVersion  string
	DaemonID       string
}

type eventStore interface {
	Append(ctx context.Context, envelope protocol.Envelope) error
	Recent(ctx context.Context, limit int) ([]store.Record, error)
	Close() error
}

type WebSocketServer struct {
	cfg      WebSocketConfig
	upgrader websocket.Upgrader

	mu       sync.RWMutex
	sessions map[string]*sessionConn

	seq        uint64
	eventStore eventStore

	closeOnce sync.Once
	closeErr  error
}

type sessionConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewWebSocketServer(cfg WebSocketConfig) (*WebSocketServer, error) {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid websocket port: %d", cfg.Port)
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return nil, errors.New("auth token is required")
	}
	if strings.TrimSpace(cfg.ServerVersion) == "" {
		cfg.ServerVersion = "dev"
	}
	if strings.TrimSpace(cfg.DaemonID) == "" {
		cfg.DaemonID = "daemon-1"
	}
	if strings.TrimSpace(cfg.EventStorePath) == "" {
		cfg.EventStorePath = ".panex/events.db"
	}

	eventStore, err := store.NewSQLiteEventStore(cfg.EventStorePath)
	if err != nil {
		return nil, fmt.Errorf("configure event store: %w", err)
	}

	return &WebSocketServer{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			// The daemon is for local developer workflows. We keep origin checks simple for MVP
			// and rely on token auth; tighten this once the agent connection story is finalized.
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		sessions:   make(map[string]*sessionConn),
		eventStore: eventStore,
	}, nil
}

func (s *WebSocketServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	return mux
}

func (s *WebSocketServer) Run(ctx context.Context) (runErr error) {
	defer func() {
		if closeErr := s.Close(); closeErr != nil && runErr == nil {
			runErr = fmt.Errorf("close event store: %w", closeErr)
		}
	}()

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(s.cfg.Port),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		s.closeAllConnections()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown websocket server: %w", err)
		}

		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("start websocket server: %w", err)
		}

		return nil
	}
}

func (s *WebSocketServer) ConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.sessions)
}

func (s *WebSocketServer) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.eventStore.Close()
	})

	return s.closeErr
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxMessageBytes)

	sessionID, err := s.handshake(conn)
	if err != nil {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		return
	}

	s.register(sessionID, &sessionConn{conn: conn})
	go s.readLoop(sessionID, conn)
}

func (s *WebSocketServer) authorized(r *http.Request) bool {
	return r.URL.Query().Get("token") == s.cfg.AuthToken
}

func (s *WebSocketServer) handshake(conn *websocket.Conn) (string, error) {
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return "", fmt.Errorf("read hello message: %w", err)
	}

	message, err := protocol.DecodeEnvelope(raw)
	if err != nil {
		return "", fmt.Errorf("decode hello envelope: %w", err)
	}
	if err := message.ValidateBase(); err != nil {
		return "", fmt.Errorf("validate hello envelope: %w", err)
	}
	if message.Name != protocol.MessageHello {
		return "", fmt.Errorf("expected first message %q, got %q", protocol.MessageHello, message.Name)
	}
	if message.T != protocol.TypeLifecycle {
		return "", fmt.Errorf("unexpected hello message type %q", message.T)
	}

	var hello protocol.Hello
	if err := protocol.DecodePayload(message.Data, &hello); err != nil {
		return "", fmt.Errorf("decode hello payload: %w", err)
	}
	if hello.ProtocolVersion != protocol.CurrentVersion {
		return "", fmt.Errorf(
			"unsupported hello protocol_version %d (expected %d)",
			hello.ProtocolVersion,
			protocol.CurrentVersion,
		)
	}
	if err := s.eventStore.Append(context.Background(), message); err != nil {
		return "", fmt.Errorf("persist hello message: %w", err)
	}

	sessionID := s.nextSessionID()
	welcome := protocol.NewWelcome(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   s.cfg.DaemonID,
		},
		protocol.Welcome{
			ProtocolVersion: protocol.CurrentVersion,
			SessionID:       sessionID,
			ServerVersion:   s.cfg.ServerVersion,
		},
	)

	encodedWelcome, err := protocol.Encode(welcome)
	if err != nil {
		return "", fmt.Errorf("encode welcome message: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, encodedWelcome); err != nil {
		return "", fmt.Errorf("write welcome message: %w", err)
	}
	if err := s.eventStore.Append(context.Background(), welcome); err != nil {
		return "", fmt.Errorf("persist welcome message: %w", err)
	}

	return sessionID, nil
}

func (s *WebSocketServer) readLoop(sessionID string, conn *websocket.Conn) {
	defer func() {
		s.unregister(sessionID)
		_ = conn.Close()
	}()

	for {
		_, rawMessage, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := s.handleClientMessage(sessionID, rawMessage); err != nil {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
				time.Now().Add(time.Second),
			)
			return
		}
	}
}

func (s *WebSocketServer) Broadcast(message protocol.Envelope) error {
	if err := s.eventStore.Append(context.Background(), message); err != nil {
		return fmt.Errorf("persist broadcast message: %w", err)
	}

	encoded, err := protocol.Encode(message)
	if err != nil {
		return fmt.Errorf("encode broadcast message: %w", err)
	}

	s.mu.RLock()
	sessions := make(map[string]*sessionConn, len(s.sessions))
	for sessionID, session := range s.sessions {
		sessions[sessionID] = session
	}
	s.mu.RUnlock()

	var failed []string
	for sessionID, session := range sessions {
		session.mu.Lock()
		writeErr := session.conn.WriteMessage(websocket.BinaryMessage, encoded)
		session.mu.Unlock()
		if writeErr != nil {
			_ = session.conn.Close()
			failed = append(failed, sessionID)
		}
	}

	for _, sessionID := range failed {
		s.unregister(sessionID)
	}

	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("broadcast failed for sessions: %s", strings.Join(failed, ", "))
	}

	return nil
}

func (s *WebSocketServer) handleClientMessage(sessionID string, rawMessage []byte) error {
	message, err := protocol.DecodeEnvelope(rawMessage)
	if err != nil {
		return fmt.Errorf("decode client message: %w", err)
	}
	if err := message.ValidateBase(); err != nil {
		return fmt.Errorf("validate client message: %w", err)
	}
	if message.Name != protocol.MessageQueryEvents {
		return nil
	}
	if message.T != protocol.TypeCommand {
		return fmt.Errorf("unexpected query.events message type %q", message.T)
	}

	var query protocol.QueryEvents
	if err := protocol.DecodePayload(message.Data, &query); err != nil {
		return fmt.Errorf("decode query.events payload: %w", err)
	}

	records, err := s.eventStore.Recent(context.Background(), query.Limit)
	if err != nil {
		return fmt.Errorf("query recent events: %w", err)
	}

	events := make([]protocol.EventSnapshot, 0, len(records))
	for _, record := range records {
		events = append(events, protocol.EventSnapshot{
			ID:           record.ID,
			RecordedAtMS: record.RecordedAtMS,
			Envelope:     record.Envelope,
		})
	}

	response := protocol.NewQueryEventsResult(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   s.cfg.DaemonID,
		},
		protocol.QueryEventsResult{
			Events: events,
		},
	)

	encoded, err := protocol.Encode(response)
	if err != nil {
		return fmt.Errorf("encode query.events.result response: %w", err)
	}
	if err := s.writeSessionMessage(sessionID, encoded); err != nil {
		return fmt.Errorf("write query.events.result response: %w", err)
	}

	return nil
}

func (s *WebSocketServer) writeSessionMessage(sessionID string, encoded []byte) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %q is not connected", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	return session.conn.WriteMessage(websocket.BinaryMessage, encoded)
}

func (s *WebSocketServer) nextSessionID() string {
	seq := atomic.AddUint64(&s.seq, 1)
	return "sess-" + strconv.FormatUint(seq, 10)
}

func (s *WebSocketServer) register(sessionID string, conn *sessionConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = conn
}

func (s *WebSocketServer) unregister(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
}

func (s *WebSocketServer) closeAllConnections() {
	s.mu.RLock()
	connections := make([]*sessionConn, 0, len(s.sessions))
	for _, session := range s.sessions {
		connections = append(connections, session)
	}
	s.mu.RUnlock()

	for _, session := range connections {
		_ = session.conn.Close()
	}
}

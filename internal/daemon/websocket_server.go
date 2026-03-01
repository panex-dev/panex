package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/panex-dev/panex/internal/protocol"
)

const maxMessageBytes = 1 << 20 // 1 MiB keeps accidental payload explosions bounded.

type WebSocketConfig struct {
	Port          int
	AuthToken     string
	ServerVersion string
	DaemonID      string
}

type WebSocketServer struct {
	cfg      WebSocketConfig
	upgrader websocket.Upgrader

	mu       sync.RWMutex
	sessions map[string]*websocket.Conn

	seq uint64
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

	return &WebSocketServer{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			// The daemon is for local developer workflows. We keep origin checks simple for MVP
			// and rely on token auth; tighten this once the agent connection story is finalized.
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		sessions: make(map[string]*websocket.Conn),
	}, nil
}

func (s *WebSocketServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	return mux
}

func (s *WebSocketServer) Run(ctx context.Context) error {
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

	s.register(sessionID, conn)
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

	return sessionID, nil
}

func (s *WebSocketServer) readLoop(sessionID string, conn *websocket.Conn) {
	defer func() {
		s.unregister(sessionID)
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *WebSocketServer) nextSessionID() string {
	seq := atomic.AddUint64(&s.seq, 1)
	return "sess-" + strconv.FormatUint(seq, 10)
}

func (s *WebSocketServer) register(sessionID string, conn *websocket.Conn) {
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
	connections := make([]*websocket.Conn, 0, len(s.sessions))
	for _, conn := range s.sessions {
		connections = append(connections, conn)
	}
	s.mu.RUnlock()

	for _, conn := range connections {
		_ = conn.Close()
	}
}

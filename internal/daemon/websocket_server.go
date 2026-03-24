package daemon

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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

const (
	maxMessageBytes     = 1 << 20 // 1 MiB keeps accidental payload explosions bounded.
	handshakeTimeout    = 5 * time.Second
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 5 * time.Second
	defaultExtensionID  = "default"
)

var daemonCapabilities = []string{
	"command.reload",
	"query.events",
	"query.storage",
	"storage.diff",
	"storage.set",
	"storage.remove",
	"storage.clear",
	"chrome.api.call",
	"chrome.api.result",
	"chrome.api.event",
}

type WebSocketConfig struct {
	Port           int
	BindAddress    string
	AuthToken      string
	EventStorePath string
	ServerVersion  string
	DaemonID       string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	PingInterval   time.Duration
}

type eventStore interface {
	Append(ctx context.Context, envelope protocol.Envelope, extensionID string) error
	Recent(ctx context.Context, limit int, beforeID int64, extensionID string) ([]store.Record, bool, error)
	StorageSnapshots(ctx context.Context, area string, extensionID string) ([]protocol.StorageSnapshot, error)
	SetStorageItem(
		ctx context.Context,
		source protocol.Source,
		area string,
		key string,
		value any,
		extensionID string,
	) (protocol.Envelope, error)
	RemoveStorageItem(
		ctx context.Context,
		source protocol.Source,
		area string,
		key string,
		extensionID string,
	) (protocol.Envelope, bool, error)
	ClearStorageArea(
		ctx context.Context,
		source protocol.Source,
		area string,
		extensionID string,
	) (protocol.Envelope, bool, error)
	Close() error
}

type WebSocketServer struct {
	cfg      WebSocketConfig
	upgrader websocket.Upgrader

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	sessions map[string]*sessionConn

	tabsMu sync.RWMutex
	tabs   []simulatedTab

	seq        uint64
	eventStore eventStore

	closeOnce sync.Once
	closeErr  error
}

type sessionConn struct {
	conn        *websocket.Conn
	mu          sync.Mutex
	closeOnce   sync.Once
	closeErr    error
	done        chan struct{}
	clientKind  string
	extensionID string
}

type sessionMetadata struct {
	clientKind  string
	extensionID string
}

type simulatedTab struct {
	ID            int    `msgpack:"id" json:"id"`
	WindowID      int    `msgpack:"windowId" json:"windowId"`
	Active        bool   `msgpack:"active" json:"active"`
	CurrentWindow bool   `msgpack:"currentWindow" json:"currentWindow"`
	URL           string `msgpack:"url,omitempty" json:"url,omitempty"`
	Title         string `msgpack:"title,omitempty" json:"title,omitempty"`
}

type tabsQueryFilter struct {
	hasActive        bool
	active           bool
	hasCurrentWindow bool
	currentWindow    bool
	hasWindowID      bool
	windowID         int
	urls             []string
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
	if strings.TrimSpace(cfg.BindAddress) == "" {
		cfg.BindAddress = "127.0.0.1"
	}
	if strings.TrimSpace(cfg.EventStorePath) == "" {
		cfg.EventStorePath = ".panex/events.db"
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultReadTimeout
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.PingInterval <= 0 || cfg.PingInterval >= cfg.ReadTimeout {
		cfg.PingInterval = cfg.ReadTimeout - (cfg.ReadTimeout / 10)
	}

	eventStore, err := store.NewSQLiteEventStore(cfg.EventStorePath)
	if err != nil {
		return nil, fmt.Errorf("configure event store: %w", err)
	}

	serverCtx, cancel := context.WithCancel(context.Background())

	return &WebSocketServer{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: isLocalOrigin,
		},
		ctx:        serverCtx,
		cancel:     cancel,
		sessions:   make(map[string]*sessionConn),
		tabs:       defaultTabsState(),
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
		Addr:              s.cfg.BindAddress + ":" + strconv.Itoa(s.cfg.Port),
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
		s.cancel()
		s.closeAllConnections()
		s.closeErr = s.eventStore.Close()
	})

	return s.closeErr
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxMessageBytes)

	ctx := r.Context()
	sessionID, metadata, err := s.handshake(ctx, conn)
	if err != nil {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
			time.Now().Add(s.cfg.WriteTimeout),
		)
		_ = conn.Close()
		return
	}

	session := &sessionConn{
		conn: conn,
		done: make(chan struct{}),
	}
	session.conn.SetPongHandler(func(string) error {
		return session.conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
	})
	if err := session.conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout)); err != nil {
		_ = session.close()
		return
	}
	s.register(sessionID, session, metadata)
	go s.readLoop(sessionID, session)
	go s.pingLoop(sessionID, session)
}

func (s *WebSocketServer) handshake(ctx context.Context, conn *websocket.Conn) (string, sessionMetadata, error) {
	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return "", sessionMetadata{}, fmt.Errorf("set handshake deadline: %w", err)
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()

	_, raw, err := conn.ReadMessage()
	if err != nil {
		return "", sessionMetadata{}, fmt.Errorf("read hello message: %w", err)
	}

	message, err := protocol.DecodeEnvelope(raw)
	if err != nil {
		return "", sessionMetadata{}, fmt.Errorf("decode hello envelope: %w", err)
	}
	if err := message.ValidateBase(); err != nil {
		return "", sessionMetadata{}, fmt.Errorf("validate hello envelope: %w", err)
	}
	if message.Name != protocol.MessageHello {
		return "", sessionMetadata{}, fmt.Errorf("expected first message %q, got %q", protocol.MessageHello, message.Name)
	}
	if message.T != protocol.TypeLifecycle {
		return "", sessionMetadata{}, fmt.Errorf("unexpected hello message type %q", message.T)
	}

	var hello protocol.Hello
	if err := protocol.DecodePayload(message.Data, &hello); err != nil {
		return "", sessionMetadata{}, fmt.Errorf("decode hello payload: %w", err)
	}
	if hello.ProtocolVersion != protocol.CurrentVersion {
		return "", sessionMetadata{}, fmt.Errorf(
			"unsupported hello protocol_version %d (expected %d)",
			hello.ProtocolVersion,
			protocol.CurrentVersion,
		)
	}
	if subtle.ConstantTimeCompare([]byte(hello.AuthToken), []byte(s.cfg.AuthToken)) != 1 {
		if _, err := s.writeHelloAck(conn, protocol.HelloAck{
			ProtocolVersion:       protocol.CurrentVersion,
			DaemonVersion:         s.cfg.ServerVersion,
			AuthOK:                false,
			CapabilitiesSupported: []string{},
		}); err != nil {
			return "", sessionMetadata{}, fmt.Errorf("write unauthorized hello.ack message: %w", err)
		}
		return "", sessionMetadata{}, errors.New("unauthorized")
	}
	helloExtID := normalizeExtensionID(hello.ClientKind, hello.ExtensionID)
	if err := s.eventStore.Append(ctx, message, helloExtID); err != nil {
		return "", sessionMetadata{}, fmt.Errorf("persist hello message: %w", err)
	}

	requestedCapabilities := hello.CapabilitiesRequested
	if len(requestedCapabilities) == 0 {
		// Support older clients that sent `capabilities` before `capabilities_requested`.
		requestedCapabilities = hello.Capabilities
	}
	supportedCapabilities := negotiateCapabilities(requestedCapabilities, daemonCapabilities)

	sessionID := s.nextSessionID()
	helloAck, err := s.writeHelloAck(conn, protocol.HelloAck{
		ProtocolVersion:       protocol.CurrentVersion,
		DaemonVersion:         s.cfg.ServerVersion,
		SessionID:             sessionID,
		AuthOK:                true,
		CapabilitiesSupported: supportedCapabilities,
	})
	if err != nil {
		return "", sessionMetadata{}, fmt.Errorf("write hello.ack message: %w", err)
	}
	if err := s.eventStore.Append(ctx, helloAck, helloExtID); err != nil {
		return "", sessionMetadata{}, fmt.Errorf("persist hello.ack message: %w", err)
	}

	return sessionID, sessionMetadata{
		clientKind:  normalizeClientKind(hello.ClientKind),
		extensionID: helloExtID,
	}, nil
}

func (s *WebSocketServer) writeHelloAck(conn *websocket.Conn, payload protocol.HelloAck) (protocol.Envelope, error) {
	helloAck := protocol.NewHelloAck(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   s.cfg.DaemonID,
		},
		payload,
	)
	encodedHelloAck, err := protocol.Encode(helloAck)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("encode hello.ack message: %w", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout)); err != nil {
		return protocol.Envelope{}, fmt.Errorf("set hello.ack write deadline: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, encodedHelloAck); err != nil {
		return protocol.Envelope{}, fmt.Errorf("write hello.ack message: %w", err)
	}
	return helloAck, nil
}

func (s *WebSocketServer) readLoop(sessionID string, session *sessionConn) {
	defer func() {
		s.unregister(sessionID)
		_ = session.close()
	}()

	for {
		_, rawMessage, err := session.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				fmt.Fprintf(os.Stderr, "panex: readLoop session %s: %v\n", sessionID, err)
			}
			return
		}
		if err := session.conn.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout)); err != nil {
			_ = session.writeControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()),
				s.cfg.WriteTimeout,
			)
			return
		}
		if err := s.handleClientMessage(s.ctx, sessionID, rawMessage); err != nil {
			_ = session.writeControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
				s.cfg.WriteTimeout,
			)
			return
		}
	}
}

func (s *WebSocketServer) pingLoop(sessionID string, session *sessionConn) {
	ticker := time.NewTicker(s.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-session.done:
			return
		case <-ticker.C:
			if err := session.writeControl(websocket.PingMessage, nil, s.cfg.WriteTimeout); err != nil {
				_ = session.close()
				s.unregister(sessionID)
				return
			}
		}
	}
}

func (s *WebSocketServer) Broadcast(ctx context.Context, message protocol.Envelope) error {
	extID := messageTargetExtensionID(message)
	if err := s.eventStore.Append(ctx, message, extID); err != nil {
		return fmt.Errorf("persist broadcast message: %w", err)
	}

	return s.broadcastLiveMessage(message)
}

func (s *WebSocketServer) broadcastLiveMessage(message protocol.Envelope) error {

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
	targetExtensionID := messageTargetExtensionID(message)
	for sessionID, session := range sessions {
		if !shouldDeliverLiveMessage(session, targetExtensionID) {
			continue
		}
		writeErr := session.writeMessage(websocket.BinaryMessage, encoded, s.cfg.WriteTimeout)
		if writeErr != nil {
			_ = session.close()
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

func (s *WebSocketServer) handleClientMessage(ctx context.Context, sessionID string, rawMessage []byte) error {
	message, err := protocol.DecodeEnvelope(rawMessage)
	if err != nil {
		return fmt.Errorf("decode client message: %w", err)
	}
	if err := message.ValidateBase(); err != nil {
		return fmt.Errorf("validate client message: %w", err)
	}
	extID := s.sessionExtensionID(sessionID)
	switch message.Name {
	case protocol.MessageQueryEvents:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected query.events message type %q", message.T)
		}

		var query protocol.QueryEvents
		if err := protocol.DecodePayload(message.Data, &query); err != nil {
			return fmt.Errorf("decode query.events payload: %w", err)
		}

		records, hasMore, err := s.eventStore.Recent(ctx, query.Limit, query.BeforeID, extID)
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
				Events:  events,
				HasMore: hasMore,
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
	case protocol.MessageQueryStorage:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected query.storage message type %q", message.T)
		}

		var query protocol.QueryStorage
		if err := protocol.DecodePayload(message.Data, &query); err != nil {
			return fmt.Errorf("decode query.storage payload: %w", err)
		}

		snapshots, err := s.eventStore.StorageSnapshots(ctx, query.Area, extID)
		if err != nil {
			return fmt.Errorf("build query.storage snapshots: %w", err)
		}

		response := protocol.NewQueryStorageResult(
			protocol.Source{
				Role: protocol.SourceDaemon,
				ID:   s.cfg.DaemonID,
			},
			protocol.QueryStorageResult{
				Snapshots: snapshots,
			},
		)

		encoded, err := protocol.Encode(response)
		if err != nil {
			return fmt.Errorf("encode query.storage.result response: %w", err)
		}
		if err := s.writeSessionMessage(sessionID, encoded); err != nil {
			return fmt.Errorf("write query.storage.result response: %w", err)
		}

		return nil
	case protocol.MessageStorageSet:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected storage.set message type %q", message.T)
		}

		var command protocol.StorageSet
		if err := protocol.DecodePayload(message.Data, &command); err != nil {
			return fmt.Errorf("decode storage.set payload: %w", err)
		}

		if err := s.SetStorageItem(ctx, command.Area, command.Key, command.Value, extID); err != nil {
			return fmt.Errorf("apply storage.set command: %w", err)
		}

		return nil
	case protocol.MessageStorageRemove:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected storage.remove message type %q", message.T)
		}

		var command protocol.StorageRemove
		if err := protocol.DecodePayload(message.Data, &command); err != nil {
			return fmt.Errorf("decode storage.remove payload: %w", err)
		}

		if err := s.RemoveStorageItem(ctx, command.Area, command.Key, extID); err != nil {
			return fmt.Errorf("apply storage.remove command: %w", err)
		}

		return nil
	case protocol.MessageStorageClear:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected storage.clear message type %q", message.T)
		}

		var command protocol.StorageClear
		if err := protocol.DecodePayload(message.Data, &command); err != nil {
			return fmt.Errorf("decode storage.clear payload: %w", err)
		}

		if err := s.ClearStorageArea(ctx, command.Area, extID); err != nil {
			return fmt.Errorf("apply storage.clear command: %w", err)
		}

		return nil
	case protocol.MessageChromeAPICall:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected chrome.api.call message type %q", message.T)
		}

		var command protocol.ChromeAPICall
		if err := protocol.DecodePayload(message.Data, &command); err != nil {
			return fmt.Errorf("decode chrome.api.call payload: %w", err)
		}

		result, err := s.handleChromeAPICall(ctx, command, extID)
		if err != nil {
			return fmt.Errorf("handle chrome.api.call command: %w", err)
		}

		response := protocol.NewChromeAPIResult(
			protocol.Source{
				Role: protocol.SourceDaemon,
				ID:   s.cfg.DaemonID,
			},
			result,
		)

		encoded, err := protocol.Encode(response)
		if err != nil {
			return fmt.Errorf("encode chrome.api.result response: %w", err)
		}
		if err := s.writeSessionMessage(sessionID, encoded); err != nil {
			return fmt.Errorf("write chrome.api.result response: %w", err)
		}

		return nil
	default:
		return nil
	}
}

func (s *WebSocketServer) handleChromeAPICall(
	ctx context.Context,
	command protocol.ChromeAPICall,
	extensionID string,
) (protocol.ChromeAPIResult, error) {
	callID := strings.TrimSpace(command.CallID)
	if callID == "" {
		return protocol.ChromeAPIResult{}, errors.New("chrome.api.call call_id is required")
	}

	namespace := strings.ToLower(strings.TrimSpace(command.Namespace))
	method := strings.TrimSpace(command.Method)
	if method == "" {
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   "method is required",
		}, nil
	}

	if namespace == "runtime" {
		return s.handleChromeRuntimeCall(ctx, callID, method, command.Args), nil
	}
	if namespace == "tabs" {
		return s.handleChromeTabsCall(callID, method, command.Args), nil
	}

	area, ok := storageAreaFromNamespace(namespace)
	if !ok {
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported chrome namespace %q", command.Namespace),
		}, nil
	}

	return s.handleChromeStorageCall(ctx, callID, namespace, method, area, command.Args, extensionID), nil
}

func (s *WebSocketServer) handleChromeStorageCall(
	ctx context.Context,
	callID string,
	namespace string,
	method string,
	area string,
	args []any,
	extensionID string,
) protocol.ChromeAPIResult {
	switch method {
	case "get":
		items, getErr := s.chromeStorageGet(ctx, area, args, extensionID)
		if failure, failed := chromeAPIFailureResult(callID, getErr); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    items,
		}
	case "set":
		if failure, failed := chromeAPIFailureResult(callID, s.chromeStorageSet(ctx, area, args, extensionID)); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}
	case "remove":
		if failure, failed := chromeAPIFailureResult(callID, s.chromeStorageRemove(ctx, area, args, extensionID)); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}
	case "clear":
		if len(args) > 0 {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   "clear expects no arguments",
			}
		}
		if failure, failed := chromeAPIFailureResult(callID, s.ClearStorageArea(ctx, area, extensionID)); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}
	case "getBytesInUse":
		bytesInUse, bytesErr := s.chromeStorageBytesInUse(ctx, area, args, extensionID)
		if failure, failed := chromeAPIFailureResult(callID, bytesErr); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    bytesInUse,
		}
	default:
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported %s.%s call", namespace, method),
		}
	}
}

func (s *WebSocketServer) handleChromeRuntimeCall(
	ctx context.Context,
	callID string,
	method string,
	args []any,
) protocol.ChromeAPIResult {
	switch method {
	case "sendMessage":
		if len(args) == 0 {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   "runtime.sendMessage expects a message argument",
			}
		}

		message := args[0]
		event := protocol.NewChromeAPIEvent(
			protocol.Source{Role: protocol.SourceDaemon, ID: s.cfg.DaemonID},
			protocol.ChromeAPIEvent{
				Namespace: "runtime",
				Event:     "onMessage",
				Args:      []any{message},
			},
		)
		if failure, failed := chromeAPIFailureResult(callID, s.Broadcast(ctx, event)); failed {
			return failure
		}

		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    message,
		}
	default:
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported runtime.%s call", method),
		}
	}
}

func (s *WebSocketServer) handleChromeTabsCall(
	callID string,
	method string,
	args []any,
) protocol.ChromeAPIResult {
	switch method {
	case "query":
		tabs, queryErr := s.chromeTabsQuery(args)
		if failure, failed := chromeAPIFailureResult(callID, queryErr); failed {
			return failure
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    tabs,
		}
	default:
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported tabs.%s call", method),
		}
	}
}

func chromeAPIFailureResult(callID string, operationErr error) (protocol.ChromeAPIResult, bool) {
	if operationErr == nil {
		return protocol.ChromeAPIResult{}, false
	}

	return protocol.ChromeAPIResult{
		CallID:  callID,
		Success: false,
		Error:   operationErr.Error(),
	}, true
}

func storageAreaFromNamespace(namespace string) (string, bool) {
	switch namespace {
	case "storage.local":
		return "local", true
	case "storage.sync":
		return "sync", true
	case "storage.session":
		return "session", true
	default:
		return "", false
	}
}

func (s *WebSocketServer) chromeStorageGet(ctx context.Context, area string, args []any, extensionID string) (map[string]any, error) {
	var keys []string
	defaults := map[string]any(nil)

	if len(args) > 0 {
		parsedKeys, parsedDefaults, err := parseStorageSelection(args[0])
		if err != nil {
			return nil, err
		}
		keys = parsedKeys
		defaults = parsedDefaults
	}

	snapshots, err := s.eventStore.StorageSnapshots(ctx, area, extensionID)
	if err != nil {
		return nil, err
	}
	items := snapshots[0].Items

	if defaults != nil {
		result := make(map[string]any, len(defaults))
		for key, value := range defaults {
			result[key] = value
		}
		for key, value := range items {
			if _, hasKey := defaults[key]; hasKey {
				result[key] = value
			}
		}
		return result, nil
	}

	if keys == nil {
		return items, nil
	}

	result := make(map[string]any, len(keys))
	for _, key := range keys {
		value, hasKey := items[key]
		if hasKey {
			result[key] = value
		}
	}
	return result, nil
}

func (s *WebSocketServer) chromeStorageSet(ctx context.Context, area string, args []any, extensionID string) error {
	if len(args) == 0 {
		return errors.New("set expects one object argument")
	}

	values, err := coerceStringAnyMap(args[0])
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := s.SetStorageItem(ctx, area, key, values[key], extensionID); err != nil {
			return err
		}
	}

	return nil
}

func (s *WebSocketServer) chromeStorageRemove(ctx context.Context, area string, args []any, extensionID string) error {
	if len(args) == 0 {
		return errors.New("remove expects key or key list argument")
	}

	keys, err := coerceStorageKeys(args[0])
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := s.RemoveStorageItem(ctx, area, key, extensionID); err != nil {
			return err
		}
	}

	return nil
}

func (s *WebSocketServer) chromeStorageBytesInUse(ctx context.Context, area string, args []any, extensionID string) (int, error) {
	var keys []string
	if len(args) > 0 {
		parsedKeys, _, err := parseStorageSelection(args[0])
		if err != nil {
			return 0, err
		}
		keys = parsedKeys
	}

	snapshots, err := s.eventStore.StorageSnapshots(ctx, area, extensionID)
	if err != nil {
		return 0, err
	}
	items := snapshots[0].Items

	selected := items
	if keys != nil {
		selected = make(map[string]any, len(keys))
		for _, key := range keys {
			value, hasKey := items[key]
			if hasKey {
				selected[key] = value
			}
		}
	}

	encoded, err := json.Marshal(selected)
	if err != nil {
		return 0, fmt.Errorf("encode selected storage payload: %w", err)
	}
	return len(encoded), nil
}

func (s *WebSocketServer) chromeTabsQuery(args []any) ([]simulatedTab, error) {
	filter, err := parseTabsQueryArgs(args)
	if err != nil {
		return nil, err
	}

	s.tabsMu.RLock()
	tabs := cloneTabs(s.tabs)
	s.tabsMu.RUnlock()

	if len(tabs) == 0 {
		return []simulatedTab{}, nil
	}

	filtered := make([]simulatedTab, 0, len(tabs))
	for _, tab := range tabs {
		if !matchTabsQuery(tab, filter) {
			continue
		}
		filtered = append(filtered, tab)
	}
	return filtered, nil
}

func parseTabsQueryArgs(args []any) (tabsQueryFilter, error) {
	if len(args) == 0 || args[0] == nil {
		return tabsQueryFilter{}, nil
	}
	if len(args) > 1 {
		return tabsQueryFilter{}, errors.New("tabs.query expects zero or one argument")
	}

	queryInfo, ok := args[0].(map[string]any)
	if !ok {
		return tabsQueryFilter{}, errors.New("tabs.query queryInfo must be an object")
	}

	var filter tabsQueryFilter

	if value, hasValue := queryInfo["active"]; hasValue {
		active, ok := value.(bool)
		if !ok {
			return tabsQueryFilter{}, errors.New("tabs.query active filter must be boolean")
		}
		filter.hasActive = true
		filter.active = active
	}

	if value, hasValue := queryInfo["currentWindow"]; hasValue {
		currentWindow, ok := value.(bool)
		if !ok {
			return tabsQueryFilter{}, errors.New("tabs.query currentWindow filter must be boolean")
		}
		filter.hasCurrentWindow = true
		filter.currentWindow = currentWindow
	}

	if value, hasValue := queryInfo["windowId"]; hasValue {
		windowID, ok := coerceInt(value)
		if !ok {
			return tabsQueryFilter{}, errors.New("tabs.query windowId filter must be numeric")
		}
		filter.hasWindowID = true
		filter.windowID = windowID
	}

	if value, hasValue := queryInfo["url"]; hasValue {
		parsedURLs, err := coerceURLFilters(value)
		if err != nil {
			return tabsQueryFilter{}, err
		}
		filter.urls = parsedURLs
	}

	return filter, nil
}

func coerceURLFilters(value any) ([]string, error) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return []string{}, nil
		}
		return []string{trimmed}, nil
	case []string:
		urls := make([]string, 0, len(typed))
		for _, entry := range typed {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			urls = append(urls, trimmed)
		}
		return urls, nil
	case []any:
		urls := make([]string, 0, len(typed))
		for _, entry := range typed {
			urlValue, ok := entry.(string)
			if !ok {
				return nil, errors.New("tabs.query url filter must be a string or string array")
			}
			trimmed := strings.TrimSpace(urlValue)
			if trimmed == "" {
				continue
			}
			urls = append(urls, trimmed)
		}
		return urls, nil
	default:
		return nil, errors.New("tabs.query url filter must be a string or string array")
	}
}

func coerceInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	default:
		return 0, false
	}
}

func matchTabsQuery(tab simulatedTab, filter tabsQueryFilter) bool {
	if filter.hasActive && tab.Active != filter.active {
		return false
	}
	if filter.hasCurrentWindow && tab.CurrentWindow != filter.currentWindow {
		return false
	}
	if filter.hasWindowID && tab.WindowID != filter.windowID {
		return false
	}
	if len(filter.urls) > 0 {
		matched := false
		for _, urlValue := range filter.urls {
			if tab.URL == urlValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func parseStorageSelection(value any) ([]string, map[string]any, error) {
	if value == nil {
		return nil, nil, nil
	}

	switch typed := value.(type) {
	case string:
		key := strings.TrimSpace(typed)
		if key == "" {
			return []string{}, nil, nil
		}
		return []string{key}, nil, nil
	case []string:
		return normalizeStorageKeys(typed), nil, nil
	case []any:
		keys := make([]string, 0, len(typed))
		for _, entry := range typed {
			key, ok := entry.(string)
			if !ok {
				return nil, nil, errors.New("key list must contain only strings")
			}
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			keys = append(keys, trimmed)
		}
		return deduplicateSorted(keys), nil, nil
	default:
		defaults, err := coerceStringAnyMap(typed)
		if err != nil {
			return nil, nil, errors.New("selection must be null, string, string array, or object defaults")
		}

		keys := make([]string, 0, len(defaults))
		for key := range defaults {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys, defaults, nil
	}
}

func coerceStorageKeys(value any) ([]string, error) {
	switch typed := value.(type) {
	case string:
		key := strings.TrimSpace(typed)
		if key == "" {
			return nil, errors.New("storage key is required")
		}
		return []string{key}, nil
	case []string:
		keys := normalizeStorageKeys(typed)
		if len(keys) == 0 {
			return nil, errors.New("at least one storage key is required")
		}
		return keys, nil
	case []any:
		keys := make([]string, 0, len(typed))
		for _, entry := range typed {
			key, ok := entry.(string)
			if !ok {
				return nil, errors.New("key list must contain only strings")
			}
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			keys = append(keys, trimmed)
		}
		keys = deduplicateSorted(keys)
		if len(keys) == 0 {
			return nil, errors.New("at least one storage key is required")
		}
		return keys, nil
	default:
		return nil, errors.New("key argument must be a string or string array")
	}
}

func coerceStringAnyMap(value any) (map[string]any, error) {
	record, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("argument must be an object")
	}

	normalized := make(map[string]any, len(record))
	for key, entry := range record {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			return nil, errors.New("storage object contains an empty key")
		}
		normalized[trimmed] = entry
	}

	return normalized, nil
}

func normalizeStorageKeys(keys []string) []string {
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return deduplicateSorted(normalized)
}

func deduplicateSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}

	sort.Strings(values)
	unique := make([]string, 0, len(values))
	var last string
	for index, value := range values {
		if index == 0 || value != last {
			unique = append(unique, value)
			last = value
		}
	}
	return unique
}

func (s *WebSocketServer) SetStorageItem(ctx context.Context, area, key string, value any, extensionID string) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return errors.New("storage key is required")
	}

	diff, err := s.eventStore.SetStorageItem(
		ctx,
		protocol.Source{Role: protocol.SourceDaemon, ID: s.cfg.DaemonID},
		normalizedArea,
		normalizedKey,
		value,
		extensionID,
	)
	if err != nil {
		return fmt.Errorf("persist storage.set mutation: %w", err)
	}

	return s.broadcastLiveMessage(diff)
}

func (s *WebSocketServer) RemoveStorageItem(ctx context.Context, area, key string, extensionID string) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return errors.New("storage key is required")
	}

	diff, changed, err := s.eventStore.RemoveStorageItem(
		ctx,
		protocol.Source{Role: protocol.SourceDaemon, ID: s.cfg.DaemonID},
		normalizedArea,
		normalizedKey,
		extensionID,
	)
	if err != nil {
		return fmt.Errorf("persist storage.remove mutation: %w", err)
	}
	if !changed {
		return nil
	}

	return s.broadcastLiveMessage(diff)
}

func (s *WebSocketServer) ClearStorageArea(ctx context.Context, area string, extensionID string) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	diff, changed, err := s.eventStore.ClearStorageArea(
		ctx,
		protocol.Source{Role: protocol.SourceDaemon, ID: s.cfg.DaemonID},
		normalizedArea,
		extensionID,
	)
	if err != nil {
		return fmt.Errorf("persist storage.clear mutation: %w", err)
	}
	if !changed {
		return nil
	}

	return s.broadcastLiveMessage(diff)
}

func (s *WebSocketServer) sessionExtensionID(sessionID string) string {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return ""
	}
	return session.extensionID
}

func (s *WebSocketServer) writeSessionMessage(sessionID string, encoded []byte) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %q is not connected", sessionID)
	}

	return session.writeMessage(websocket.BinaryMessage, encoded, s.cfg.WriteTimeout)
}

func (s *WebSocketServer) nextSessionID() string {
	seq := atomic.AddUint64(&s.seq, 1)
	return "sess-" + strconv.FormatUint(seq, 10)
}

func (s *WebSocketServer) register(sessionID string, conn *sessionConn, metadata sessionMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn.clientKind = metadata.clientKind
	conn.extensionID = metadata.extensionID
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
		_ = session.close()
	}
}

func (s *sessionConn) writeMessage(messageType int, data []byte, timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(messageType, data)
}

func messageTargetExtensionID(message protocol.Envelope) string {
	switch payload := message.Data.(type) {
	case protocol.BuildComplete:
		return strings.TrimSpace(payload.ExtensionID)
	case protocol.CommandReload:
		return strings.TrimSpace(payload.ExtensionID)
	default:
		return ""
	}
}

func shouldDeliverLiveMessage(session *sessionConn, targetExtensionID string) bool {
	if strings.TrimSpace(targetExtensionID) == "" {
		return true
	}
	if session.clientKind == "inspector" {
		return true
	}

	return session.extensionID == targetExtensionID
}

func normalizeClientKind(value string) string {
	return strings.TrimSpace(value)
}

func normalizeExtensionID(clientKind string, extensionID string) string {
	trimmed := strings.TrimSpace(extensionID)
	if trimmed != "" {
		return trimmed
	}
	if strings.TrimSpace(clientKind) == "dev-agent" || strings.TrimSpace(clientKind) == "chrome-sim" {
		return defaultExtensionID
	}

	return ""
}

func (s *sessionConn) writeControl(messageType int, data []byte, timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	deadline := time.Now().Add(timeout)
	if err := s.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return s.conn.WriteControl(messageType, data, deadline)
}

func (s *sessionConn) close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		s.mu.Lock()
		defer s.mu.Unlock()

		s.closeErr = s.conn.Close()
	})

	return s.closeErr
}

func normalizeStorageArea(area string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(area))
	if normalized != "local" && normalized != "sync" && normalized != "session" {
		return "", fmt.Errorf("unsupported storage area %q", area)
	}

	return normalized, nil
}

func defaultTabsState() []simulatedTab {
	return []simulatedTab{
		{
			ID:            1,
			WindowID:      1,
			Active:        true,
			CurrentWindow: true,
			URL:           "https://example.com/",
			Title:         "Example",
		},
		{
			ID:            2,
			WindowID:      1,
			Active:        false,
			CurrentWindow: true,
			URL:           "https://panex.dev/docs",
			Title:         "Panex Docs",
		},
		{
			ID:            3,
			WindowID:      2,
			Active:        true,
			CurrentWindow: false,
			URL:           "https://panex.dev/inspector",
			Title:         "Panex Inspector",
		},
	}
}

func cloneTabs(tabs []simulatedTab) []simulatedTab {
	if len(tabs) == 0 {
		return []simulatedTab{}
	}

	cloned := make([]simulatedTab, len(tabs))
	copy(cloned, tabs)
	return cloned
}

func negotiateCapabilities(requested, supported []string) []string {
	if len(requested) == 0 {
		return append([]string(nil), supported...)
	}

	supportedSet := make(map[string]struct{}, len(supported))
	for _, capability := range supported {
		trimmed := strings.TrimSpace(capability)
		if trimmed == "" {
			continue
		}
		supportedSet[trimmed] = struct{}{}
	}

	accepted := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, capability := range requested {
		trimmed := strings.TrimSpace(capability)
		if trimmed == "" {
			continue
		}
		if _, alreadyIncluded := seen[trimmed]; alreadyIncluded {
			continue
		}
		if _, ok := supportedSet[trimmed]; !ok {
			continue
		}
		accepted = append(accepted, trimmed)
		seen[trimmed] = struct{}{}
	}

	return accepted
}

// isLocalOrigin validates that WebSocket upgrade requests originate from localhost.
// Chrome extensions connect without an Origin header, which gorilla/websocket
// treats as a same-origin request (returns true). For browser-based inspector
// connections, only localhost origins are permitted.
func isLocalOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

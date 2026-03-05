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

const maxMessageBytes = 1 << 20 // 1 MiB keeps accidental payload explosions bounded.

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

var storageAreaOrder = []string{"local", "sync", "session"}

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

	storageMu sync.RWMutex
	storage   map[string]map[string]any

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
			CheckOrigin: isLocalOrigin,
		},
		sessions:   make(map[string]*sessionConn),
		storage:    defaultStorageState(),
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

	ctx := r.Context()
	sessionID, err := s.handshake(ctx, conn)
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
	// Token is passed as a query parameter because the browser WebSocket API
	// does not support custom headers (no Authorization header possible).
	token := r.URL.Query().Get("token")
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AuthToken)) == 1
}

func (s *WebSocketServer) handshake(ctx context.Context, conn *websocket.Conn) (string, error) {
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
	if err := s.eventStore.Append(ctx, message); err != nil {
		return "", fmt.Errorf("persist hello message: %w", err)
	}

	requestedCapabilities := hello.CapabilitiesRequested
	if len(requestedCapabilities) == 0 {
		// Support older clients that sent `capabilities` before `capabilities_requested`.
		requestedCapabilities = hello.Capabilities
	}
	supportedCapabilities := negotiateCapabilities(requestedCapabilities, daemonCapabilities)

	sessionID := s.nextSessionID()
	helloAck := protocol.NewHelloAck(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   s.cfg.DaemonID,
		},
		protocol.HelloAck{
			ProtocolVersion:       protocol.CurrentVersion,
			DaemonVersion:         s.cfg.ServerVersion,
			SessionID:             sessionID,
			AuthOK:                true,
			CapabilitiesSupported: supportedCapabilities,
		},
	)

	encodedHelloAck, err := protocol.Encode(helloAck)
	if err != nil {
		return "", fmt.Errorf("encode hello.ack message: %w", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, encodedHelloAck); err != nil {
		return "", fmt.Errorf("write hello.ack message: %w", err)
	}
	if err := s.eventStore.Append(ctx, helloAck); err != nil {
		return "", fmt.Errorf("persist hello.ack message: %w", err)
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
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				fmt.Fprintf(os.Stderr, "panex: readLoop session %s: %v\n", sessionID, err)
			}
			return
		}
		if err := s.handleClientMessage(context.Background(), sessionID, rawMessage); err != nil {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()),
				time.Now().Add(time.Second),
			)
			return
		}
	}
}

func (s *WebSocketServer) Broadcast(ctx context.Context, message protocol.Envelope) error {
	if err := s.eventStore.Append(ctx, message); err != nil {
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

func (s *WebSocketServer) handleClientMessage(ctx context.Context, sessionID string, rawMessage []byte) error {
	message, err := protocol.DecodeEnvelope(rawMessage)
	if err != nil {
		return fmt.Errorf("decode client message: %w", err)
	}
	if err := message.ValidateBase(); err != nil {
		return fmt.Errorf("validate client message: %w", err)
	}
	switch message.Name {
	case protocol.MessageQueryEvents:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected query.events message type %q", message.T)
		}

		var query protocol.QueryEvents
		if err := protocol.DecodePayload(message.Data, &query); err != nil {
			return fmt.Errorf("decode query.events payload: %w", err)
		}

		records, err := s.eventStore.Recent(ctx, query.Limit)
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
	case protocol.MessageQueryStorage:
		if message.T != protocol.TypeCommand {
			return fmt.Errorf("unexpected query.storage message type %q", message.T)
		}

		var query protocol.QueryStorage
		if err := protocol.DecodePayload(message.Data, &query); err != nil {
			return fmt.Errorf("decode query.storage payload: %w", err)
		}

		snapshots, err := s.buildStorageSnapshots(query.Area)
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

		if err := s.SetStorageItem(ctx, command.Area, command.Key, command.Value); err != nil {
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

		if err := s.RemoveStorageItem(ctx, command.Area, command.Key); err != nil {
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

		if err := s.ClearStorageArea(ctx, command.Area); err != nil {
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

		result, err := s.handleChromeAPICall(ctx, command)
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

	area, ok := storageAreaFromNamespace(namespace)
	if !ok {
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported chrome namespace %q", command.Namespace),
		}, nil
	}

	switch method {
	case "get":
		items, err := s.chromeStorageGet(area, command.Args)
		if err != nil {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    items,
		}, nil
	case "set":
		if err := s.chromeStorageSet(ctx, area, command.Args); err != nil {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}, nil
	case "remove":
		if err := s.chromeStorageRemove(ctx, area, command.Args); err != nil {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}, nil
	case "clear":
		if len(command.Args) > 0 {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   "clear expects no arguments",
			}, nil
		}
		if err := s.ClearStorageArea(ctx, area); err != nil {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
		}, nil
	case "getBytesInUse":
		bytesInUse, err := s.chromeStorageBytesInUse(area, command.Args)
		if err != nil {
			return protocol.ChromeAPIResult{
				CallID:  callID,
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: true,
			Data:    bytesInUse,
		}, nil
	default:
		return protocol.ChromeAPIResult{
			CallID:  callID,
			Success: false,
			Error:   fmt.Sprintf("unsupported %s.%s call", namespace, method),
		}, nil
	}
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

func (s *WebSocketServer) chromeStorageGet(area string, args []any) (map[string]any, error) {
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

	s.storageMu.RLock()
	items := cloneStorageItems(s.storage[area])
	s.storageMu.RUnlock()

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

func (s *WebSocketServer) chromeStorageSet(ctx context.Context, area string, args []any) error {
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
		if err := s.SetStorageItem(ctx, area, key, values[key]); err != nil {
			return err
		}
	}

	return nil
}

func (s *WebSocketServer) chromeStorageRemove(ctx context.Context, area string, args []any) error {
	if len(args) == 0 {
		return errors.New("remove expects key or key list argument")
	}

	keys, err := coerceStorageKeys(args[0])
	if err != nil {
		return err
	}

	for _, key := range keys {
		if err := s.RemoveStorageItem(ctx, area, key); err != nil {
			return err
		}
	}

	return nil
}

func (s *WebSocketServer) chromeStorageBytesInUse(area string, args []any) (int, error) {
	var keys []string
	if len(args) > 0 {
		parsedKeys, _, err := parseStorageSelection(args[0])
		if err != nil {
			return 0, err
		}
		keys = parsedKeys
	}

	s.storageMu.RLock()
	items := cloneStorageItems(s.storage[area])
	s.storageMu.RUnlock()

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

func (s *WebSocketServer) SetStorageItem(ctx context.Context, area, key string, value any) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return errors.New("storage key is required")
	}

	s.storageMu.Lock()
	items := s.storage[normalizedArea]
	oldValue, hadOldValue := items[normalizedKey]
	items[normalizedKey] = value
	s.storageMu.Unlock()

	change := protocol.StorageChange{
		Key:      normalizedKey,
		NewValue: value,
	}
	if hadOldValue {
		change.OldValue = oldValue
	}

	return s.broadcastStorageDiff(ctx, normalizedArea, []protocol.StorageChange{change})
}

func (s *WebSocketServer) RemoveStorageItem(ctx context.Context, area, key string) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return errors.New("storage key is required")
	}

	s.storageMu.Lock()
	items := s.storage[normalizedArea]
	oldValue, hadOldValue := items[normalizedKey]
	if hadOldValue {
		delete(items, normalizedKey)
	}
	s.storageMu.Unlock()

	if !hadOldValue {
		return nil
	}

	change := protocol.StorageChange{
		Key:      normalizedKey,
		OldValue: oldValue,
	}
	return s.broadcastStorageDiff(ctx, normalizedArea, []protocol.StorageChange{change})
}

func (s *WebSocketServer) ClearStorageArea(ctx context.Context, area string) error {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return err
	}

	s.storageMu.Lock()
	items := s.storage[normalizedArea]
	if len(items) == 0 {
		s.storageMu.Unlock()
		return nil
	}

	changes := make([]protocol.StorageChange, 0, len(items))
	for key, oldValue := range items {
		changes = append(changes, protocol.StorageChange{
			Key:      key,
			OldValue: oldValue,
		})
	}
	clear(items)
	s.storageMu.Unlock()

	sort.Slice(changes, func(left, right int) bool {
		return changes[left].Key < changes[right].Key
	})

	return s.broadcastStorageDiff(ctx, normalizedArea, changes)
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

func (s *WebSocketServer) buildStorageSnapshots(area string) ([]protocol.StorageSnapshot, error) {
	trimmed := strings.TrimSpace(area)
	if trimmed == "" {
		s.storageMu.RLock()
		defer s.storageMu.RUnlock()

		snapshots := make([]protocol.StorageSnapshot, 0, len(storageAreaOrder))
		for _, storageArea := range storageAreaOrder {
			snapshots = append(snapshots, protocol.StorageSnapshot{
				Area:  storageArea,
				Items: cloneStorageItems(s.storage[storageArea]),
			})
		}

		return snapshots, nil
	}

	normalized, err := normalizeStorageArea(trimmed)
	if err != nil {
		return nil, err
	}

	s.storageMu.RLock()
	defer s.storageMu.RUnlock()

	return []protocol.StorageSnapshot{
		{Area: normalized, Items: cloneStorageItems(s.storage[normalized])},
	}, nil
}

func (s *WebSocketServer) broadcastStorageDiff(ctx context.Context, area string, changes []protocol.StorageChange) error {
	if len(changes) == 0 {
		return nil
	}

	diff := protocol.NewStorageDiff(
		protocol.Source{
			Role: protocol.SourceDaemon,
			ID:   s.cfg.DaemonID,
		},
		protocol.StorageDiff{
			Area:    area,
			Changes: changes,
		},
	)

	return s.Broadcast(ctx, diff)
}

func normalizeStorageArea(area string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(area))
	if normalized != "local" && normalized != "sync" && normalized != "session" {
		return "", fmt.Errorf("unsupported storage area %q", area)
	}

	return normalized, nil
}

func defaultStorageState() map[string]map[string]any {
	return map[string]map[string]any{
		"local":   {},
		"sync":    {},
		"session": {},
	}
}

func cloneStorageItems(items map[string]any) map[string]any {
	if len(items) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(items))
	for key, value := range items {
		cloned[key] = value
	}
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

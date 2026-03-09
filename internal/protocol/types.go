package protocol

import (
	"errors"
	"fmt"
	"strings"
)

const CurrentVersion uint8 = 1

type MessageType string

const (
	TypeLifecycle MessageType = "lifecycle"
	TypeEvent     MessageType = "event"
	TypeCommand   MessageType = "command"
)

type MessageName string

const (
	MessageHello           MessageName = "hello"
	MessageHelloAck        MessageName = "hello.ack"
	MessageBuildComplete   MessageName = "build.complete"
	MessageContextLog      MessageName = "context.log"
	MessageCommandReload   MessageName = "command.reload"
	MessageQueryEvents     MessageName = "query.events"
	MessageQueryResult     MessageName = "query.events.result"
	MessageQueryStorage    MessageName = "query.storage"
	MessageStorageResult   MessageName = "query.storage.result"
	MessageStorageDiff     MessageName = "storage.diff"
	MessageStorageSet      MessageName = "storage.set"
	MessageStorageRemove   MessageName = "storage.remove"
	MessageStorageClear    MessageName = "storage.clear"
	MessageChromeAPICall   MessageName = "chrome.api.call"
	MessageChromeAPIResult MessageName = "chrome.api.result"
	MessageChromeAPIEvent  MessageName = "chrome.api.event"
)

type SourceRole string

const (
	SourceDaemon    SourceRole = "daemon"
	SourceDevAgent  SourceRole = "dev-agent"
	SourceInspector SourceRole = "inspector"
)

type Source struct {
	Role SourceRole `msgpack:"role"`
	ID   string     `msgpack:"id"`
}

func (s Source) Validate() error {
	if strings.TrimSpace(string(s.Role)) == "" {
		return errors.New("source role is required")
	}
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("source id is required")
	}

	return nil
}

type Envelope struct {
	V    uint8       `msgpack:"v"`
	T    MessageType `msgpack:"t"`
	Name MessageName `msgpack:"name"`
	Src  Source      `msgpack:"src"`
	Data any         `msgpack:"data"`
}

func (e Envelope) ValidateBase() error {
	if e.V != CurrentVersion {
		return fmt.Errorf("unsupported protocol version %d (expected %d)", e.V, CurrentVersion)
	}
	if strings.TrimSpace(string(e.T)) == "" {
		return errors.New("message type is required")
	}
	if strings.TrimSpace(string(e.Name)) == "" {
		return errors.New("message name is required")
	}
	if err := e.Src.Validate(); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	if e.Data == nil {
		return errors.New("message data is required")
	}

	return nil
}

type Hello struct {
	ProtocolVersion       uint8    `msgpack:"protocol_version"`
	AuthToken             string   `msgpack:"auth_token,omitempty"`
	ClientKind            string   `msgpack:"client_kind,omitempty"`
	ClientVersion         string   `msgpack:"client_version,omitempty"`
	CapabilitiesRequested []string `msgpack:"capabilities_requested,omitempty"`
	// Capabilities is retained for backward compatibility with early clients.
	Capabilities []string `msgpack:"capabilities,omitempty"`
}

type HelloAck struct {
	ProtocolVersion       uint8    `msgpack:"protocol_version"`
	DaemonVersion         string   `msgpack:"daemon_version"`
	SessionID             string   `msgpack:"session_id"`
	AuthOK                bool     `msgpack:"auth_ok"`
	CapabilitiesSupported []string `msgpack:"capabilities_supported"`
}

type BuildComplete struct {
	BuildID      string   `msgpack:"build_id"`
	Success      bool     `msgpack:"success"`
	DurationMS   int64    `msgpack:"duration_ms"`
	ChangedFiles []string `msgpack:"changed_files,omitempty"`
}

type ContextLog struct {
	ContextID  string `msgpack:"context_id"`
	Level      string `msgpack:"level"`
	Message    string `msgpack:"message"`
	TimestampS int64  `msgpack:"timestamp_s"`
}

type CommandReload struct {
	Reason  string `msgpack:"reason"`
	BuildID string `msgpack:"build_id,omitempty"`
}

type QueryEvents struct {
	Limit int `msgpack:"limit,omitempty"`
}

type EventSnapshot struct {
	ID           int64    `msgpack:"id"`
	RecordedAtMS int64    `msgpack:"recorded_at_ms"`
	Envelope     Envelope `msgpack:"envelope"`
}

type QueryEventsResult struct {
	Events []EventSnapshot `msgpack:"events"`
}

type QueryStorage struct {
	Area string `msgpack:"area,omitempty"`
}

type StorageSnapshot struct {
	Area  string         `msgpack:"area"`
	Items map[string]any `msgpack:"items"`
}

type QueryStorageResult struct {
	Snapshots []StorageSnapshot `msgpack:"snapshots"`
}

type StorageDiff struct {
	Area    string          `msgpack:"area"`
	Changes []StorageChange `msgpack:"changes"`
}

type StorageChange struct {
	Key      string `msgpack:"key"`
	OldValue any    `msgpack:"old_value,omitempty"`
	NewValue any    `msgpack:"new_value,omitempty"`
}

type StorageSet struct {
	Area  string `msgpack:"area"`
	Key   string `msgpack:"key"`
	Value any    `msgpack:"value"`
}

type StorageRemove struct {
	Area string `msgpack:"area"`
	Key  string `msgpack:"key"`
}

type StorageClear struct {
	Area string `msgpack:"area"`
}

type ChromeAPICall struct {
	CallID    string `msgpack:"call_id"`
	Namespace string `msgpack:"namespace"`
	Method    string `msgpack:"method"`
	Args      []any  `msgpack:"args,omitempty"`
}

type ChromeAPIResult struct {
	CallID  string `msgpack:"call_id"`
	Success bool   `msgpack:"success"`
	Data    any    `msgpack:"data,omitempty"`
	Error   string `msgpack:"error,omitempty"`
}

type ChromeAPIEvent struct {
	Namespace string `msgpack:"namespace"`
	Event     string `msgpack:"event"`
	Args      []any  `msgpack:"args,omitempty"`
}

var messageTypeByName = map[MessageName]MessageType{
	MessageHello:           TypeLifecycle,
	MessageHelloAck:        TypeLifecycle,
	MessageBuildComplete:   TypeEvent,
	MessageContextLog:      TypeEvent,
	MessageCommandReload:   TypeCommand,
	MessageQueryEvents:     TypeCommand,
	MessageQueryResult:     TypeEvent,
	MessageQueryStorage:    TypeCommand,
	MessageStorageResult:   TypeEvent,
	MessageStorageDiff:     TypeEvent,
	MessageStorageSet:      TypeCommand,
	MessageStorageRemove:   TypeCommand,
	MessageStorageClear:    TypeCommand,
	MessageChromeAPICall:   TypeCommand,
	MessageChromeAPIResult: TypeEvent,
	MessageChromeAPIEvent:  TypeEvent,
}

func MessageTypeForName(name MessageName) (MessageType, bool) {
	messageType, ok := messageTypeByName[name]
	return messageType, ok
}

func NewHello(src Source, data Hello) Envelope {
	return newEnvelope(TypeLifecycle, MessageHello, src, data)
}

func NewHelloAck(src Source, data HelloAck) Envelope {
	return newEnvelope(TypeLifecycle, MessageHelloAck, src, data)
}

func NewBuildComplete(src Source, data BuildComplete) Envelope {
	return newEnvelope(TypeEvent, MessageBuildComplete, src, data)
}

func NewContextLog(src Source, data ContextLog) Envelope {
	return newEnvelope(TypeEvent, MessageContextLog, src, data)
}

func NewCommandReload(src Source, data CommandReload) Envelope {
	return newEnvelope(TypeCommand, MessageCommandReload, src, data)
}

func NewQueryEvents(src Source, data QueryEvents) Envelope {
	return newEnvelope(TypeCommand, MessageQueryEvents, src, data)
}

func NewQueryEventsResult(src Source, data QueryEventsResult) Envelope {
	return newEnvelope(TypeEvent, MessageQueryResult, src, data)
}

func NewQueryStorage(src Source, data QueryStorage) Envelope {
	return newEnvelope(TypeCommand, MessageQueryStorage, src, data)
}

func NewQueryStorageResult(src Source, data QueryStorageResult) Envelope {
	return newEnvelope(TypeEvent, MessageStorageResult, src, data)
}

func NewStorageDiff(src Source, data StorageDiff) Envelope {
	return newEnvelope(TypeEvent, MessageStorageDiff, src, data)
}

func NewStorageSet(src Source, data StorageSet) Envelope {
	return newEnvelope(TypeCommand, MessageStorageSet, src, data)
}

func NewStorageRemove(src Source, data StorageRemove) Envelope {
	return newEnvelope(TypeCommand, MessageStorageRemove, src, data)
}

func NewStorageClear(src Source, data StorageClear) Envelope {
	return newEnvelope(TypeCommand, MessageStorageClear, src, data)
}

func NewChromeAPICall(src Source, data ChromeAPICall) Envelope {
	return newEnvelope(TypeCommand, MessageChromeAPICall, src, data)
}

func NewChromeAPIResult(src Source, data ChromeAPIResult) Envelope {
	return newEnvelope(TypeEvent, MessageChromeAPIResult, src, data)
}

func NewChromeAPIEvent(src Source, data ChromeAPIEvent) Envelope {
	return newEnvelope(TypeEvent, MessageChromeAPIEvent, src, data)
}

func newEnvelope(messageType MessageType, name MessageName, src Source, data any) Envelope {
	return Envelope{
		V:    CurrentVersion,
		T:    messageType,
		Name: name,
		Src:  src,
		Data: data,
	}
}

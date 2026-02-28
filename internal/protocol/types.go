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
	MessageHello         MessageName = "hello"
	MessageWelcome       MessageName = "welcome"
	MessageBuildComplete MessageName = "build.complete"
	MessageContextLog    MessageName = "context.log"
	MessageCommandReload MessageName = "command.reload"
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
	ProtocolVersion uint8    `msgpack:"protocol_version"`
	Capabilities    []string `msgpack:"capabilities,omitempty"`
}

type Welcome struct {
	ProtocolVersion uint8  `msgpack:"protocol_version"`
	SessionID       string `msgpack:"session_id"`
	ServerVersion   string `msgpack:"server_version"`
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

var messageTypeByName = map[MessageName]MessageType{
	MessageHello:         TypeLifecycle,
	MessageWelcome:       TypeLifecycle,
	MessageBuildComplete: TypeEvent,
	MessageContextLog:    TypeEvent,
	MessageCommandReload: TypeCommand,
}

func MessageTypeForName(name MessageName) (MessageType, bool) {
	messageType, ok := messageTypeByName[name]
	return messageType, ok
}

func NewHello(src Source, data Hello) Envelope {
	return newEnvelope(TypeLifecycle, MessageHello, src, data)
}

func NewWelcome(src Source, data Welcome) Envelope {
	return newEnvelope(TypeLifecycle, MessageWelcome, src, data)
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

func newEnvelope(messageType MessageType, name MessageName, src Source, data any) Envelope {
	return Envelope{
		V:    CurrentVersion,
		T:    messageType,
		Name: name,
		Src:  src,
		Data: data,
	}
}

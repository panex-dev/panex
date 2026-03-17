package protocol

import (
	"reflect"
	"strings"
	"testing"
)

func TestMessageTypeForName(t *testing.T) {
	testCases := []struct {
		name    MessageName
		want    MessageType
		wantOK  bool
		caseTag string
	}{
		{name: MessageHello, want: TypeLifecycle, wantOK: true, caseTag: "hello"},
		{name: MessageHelloAck, want: TypeLifecycle, wantOK: true, caseTag: "hello.ack"},
		{name: MessageBuildComplete, want: TypeEvent, wantOK: true, caseTag: "build.complete"},
		{name: MessageContextLog, want: TypeEvent, wantOK: true, caseTag: "context.log"},
		{name: MessageCommandReload, want: TypeCommand, wantOK: true, caseTag: "command.reload"},
		{name: MessageQueryEvents, want: TypeCommand, wantOK: true, caseTag: "query.events"},
		{name: MessageQueryResult, want: TypeEvent, wantOK: true, caseTag: "query.events.result"},
		{name: MessageQueryStorage, want: TypeCommand, wantOK: true, caseTag: "query.storage"},
		{name: MessageStorageResult, want: TypeEvent, wantOK: true, caseTag: "query.storage.result"},
		{name: MessageStorageDiff, want: TypeEvent, wantOK: true, caseTag: "storage.diff"},
		{name: MessageStorageSet, want: TypeCommand, wantOK: true, caseTag: "storage.set"},
		{name: MessageStorageRemove, want: TypeCommand, wantOK: true, caseTag: "storage.remove"},
		{name: MessageStorageClear, want: TypeCommand, wantOK: true, caseTag: "storage.clear"},
		{name: MessageChromeAPICall, want: TypeCommand, wantOK: true, caseTag: "chrome.api.call"},
		{name: MessageChromeAPIResult, want: TypeEvent, wantOK: true, caseTag: "chrome.api.result"},
		{name: MessageChromeAPIEvent, want: TypeEvent, wantOK: true, caseTag: "chrome.api.event"},
		{name: MessageName("unknown"), want: "", wantOK: false, caseTag: "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.caseTag, func(t *testing.T) {
			got, ok := MessageTypeForName(tc.name)
			if ok != tc.wantOK {
				t.Fatalf("unexpected lookup status: got %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("unexpected message type: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSourceValidate(t *testing.T) {
	valid := Source{
		Role: SourceDaemon,
		ID:   "daemon-1",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid source should pass validation: %v", err)
	}

	testCases := []struct {
		name      string
		src       Source
		wantError string
	}{
		{
			name: "missing role",
			src: Source{
				Role: "",
				ID:   "daemon-1",
			},
			wantError: "source role is required",
		},
		{
			name: "missing id",
			src: Source{
				Role: SourceDaemon,
				ID:   "",
			},
			wantError: "source id is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.src.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantError)
			}
		})
	}
}

func TestEnvelopeValidateBase(t *testing.T) {
	valid := NewHello(
		Source{Role: SourceDevAgent, ID: "agent-1"},
		Hello{ProtocolVersion: CurrentVersion},
	)
	if err := valid.ValidateBase(); err != nil {
		t.Fatalf("valid envelope should pass validation: %v", err)
	}

	testCases := []struct {
		name      string
		envelope  Envelope
		wantError string
	}{
		{
			name: "wrong version",
			envelope: Envelope{
				V:    9,
				T:    TypeLifecycle,
				Name: MessageHello,
				Src:  Source{Role: SourceDevAgent, ID: "agent-1"},
				Data: Hello{ProtocolVersion: CurrentVersion},
			},
			wantError: "unsupported protocol version",
		},
		{
			name: "missing type",
			envelope: Envelope{
				V:    CurrentVersion,
				T:    "",
				Name: MessageHello,
				Src:  Source{Role: SourceDevAgent, ID: "agent-1"},
				Data: Hello{ProtocolVersion: CurrentVersion},
			},
			wantError: "message type is required",
		},
		{
			name: "missing name",
			envelope: Envelope{
				V:    CurrentVersion,
				T:    TypeLifecycle,
				Name: "",
				Src:  Source{Role: SourceDevAgent, ID: "agent-1"},
				Data: Hello{ProtocolVersion: CurrentVersion},
			},
			wantError: "message name is required",
		},
		{
			name: "invalid source",
			envelope: Envelope{
				V:    CurrentVersion,
				T:    TypeLifecycle,
				Name: MessageHello,
				Src:  Source{Role: SourceDevAgent, ID: ""},
				Data: Hello{ProtocolVersion: CurrentVersion},
			},
			wantError: "invalid source",
		},
		{
			name: "missing data",
			envelope: Envelope{
				V:    CurrentVersion,
				T:    TypeLifecycle,
				Name: MessageHello,
				Src:  Source{Role: SourceDevAgent, ID: "agent-1"},
				Data: nil,
			},
			wantError: "message data is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.envelope.ValidateBase()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: got %v, want contains %q", err, tc.wantError)
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	src := Source{
		Role: SourceDaemon,
		ID:   "daemon-1",
	}

	testCases := []struct {
		name         string
		got          Envelope
		wantType     MessageType
		wantName     MessageName
		wantDataType any
	}{
		{
			name:         "hello",
			got:          NewHello(src, Hello{ProtocolVersion: CurrentVersion}),
			wantType:     TypeLifecycle,
			wantName:     MessageHello,
			wantDataType: Hello{},
		},
		{
			name: "hello.ack",
			got: NewHelloAck(src, HelloAck{
				ProtocolVersion:       CurrentVersion,
				DaemonVersion:         "dev",
				SessionID:             "s1",
				AuthOK:                true,
				CapabilitiesSupported: []string{"query.events"},
			}),
			wantType:     TypeLifecycle,
			wantName:     MessageHelloAck,
			wantDataType: HelloAck{},
		},
		{
			name:         "build.complete",
			got:          NewBuildComplete(src, BuildComplete{BuildID: "b1", Success: true, DurationMS: 10, ExtensionID: "default"}),
			wantType:     TypeEvent,
			wantName:     MessageBuildComplete,
			wantDataType: BuildComplete{},
		},
		{
			name:         "context.log",
			got:          NewContextLog(src, ContextLog{ContextID: "background", Level: "info", Message: "ok", TimestampMS: 10000}),
			wantType:     TypeEvent,
			wantName:     MessageContextLog,
			wantDataType: ContextLog{},
		},
		{
			name:         "command.reload",
			got:          NewCommandReload(src, CommandReload{Reason: "build complete", ExtensionID: "default"}),
			wantType:     TypeCommand,
			wantName:     MessageCommandReload,
			wantDataType: CommandReload{},
		},
		{
			name:         "query.events",
			got:          NewQueryEvents(src, QueryEvents{Limit: 10, BeforeID: 99}),
			wantType:     TypeCommand,
			wantName:     MessageQueryEvents,
			wantDataType: QueryEvents{},
		},
		{
			name: "query.events.result",
			got: NewQueryEventsResult(src, QueryEventsResult{
				HasMore: true,
				Events: []EventSnapshot{
					{
						ID:           1,
						RecordedAtMS: 1234,
						Envelope: NewBuildComplete(src, BuildComplete{
							BuildID:    "b1",
							Success:    true,
							DurationMS: 9,
						}),
					},
				},
			}),
			wantType:     TypeEvent,
			wantName:     MessageQueryResult,
			wantDataType: QueryEventsResult{},
		},
		{
			name:         "query.storage",
			got:          NewQueryStorage(src, QueryStorage{Area: "local"}),
			wantType:     TypeCommand,
			wantName:     MessageQueryStorage,
			wantDataType: QueryStorage{},
		},
		{
			name: "query.storage.result",
			got: NewQueryStorageResult(src, QueryStorageResult{
				Snapshots: []StorageSnapshot{
					{Area: "local", Items: map[string]any{"theme": "light"}},
				},
			}),
			wantType:     TypeEvent,
			wantName:     MessageStorageResult,
			wantDataType: QueryStorageResult{},
		},
		{
			name: "storage.diff",
			got: NewStorageDiff(src, StorageDiff{
				Area: "local",
				Changes: []StorageChange{
					{Key: "theme", OldValue: "dark", NewValue: "light"},
				},
			}),
			wantType:     TypeEvent,
			wantName:     MessageStorageDiff,
			wantDataType: StorageDiff{},
		},
		{
			name: "storage.set",
			got: NewStorageSet(src, StorageSet{
				Area:  "local",
				Key:   "theme",
				Value: "dark",
			}),
			wantType:     TypeCommand,
			wantName:     MessageStorageSet,
			wantDataType: StorageSet{},
		},
		{
			name: "storage.remove",
			got: NewStorageRemove(src, StorageRemove{
				Area: "local",
				Key:  "theme",
			}),
			wantType:     TypeCommand,
			wantName:     MessageStorageRemove,
			wantDataType: StorageRemove{},
		},
		{
			name: "storage.clear",
			got: NewStorageClear(src, StorageClear{
				Area: "local",
			}),
			wantType:     TypeCommand,
			wantName:     MessageStorageClear,
			wantDataType: StorageClear{},
		},
		{
			name: "chrome.api.call",
			got: NewChromeAPICall(src, ChromeAPICall{
				CallID:    "call-1",
				Namespace: "storage.local",
				Method:    "get",
				Args:      []any{"theme"},
			}),
			wantType:     TypeCommand,
			wantName:     MessageChromeAPICall,
			wantDataType: ChromeAPICall{},
		},
		{
			name: "chrome.api.result",
			got: NewChromeAPIResult(src, ChromeAPIResult{
				CallID:  "call-1",
				Success: true,
				Data:    map[string]any{"theme": "dark"},
			}),
			wantType:     TypeEvent,
			wantName:     MessageChromeAPIResult,
			wantDataType: ChromeAPIResult{},
		},
		{
			name: "chrome.api.event",
			got: NewChromeAPIEvent(src, ChromeAPIEvent{
				Namespace: "storage.onChanged",
				Event:     "changed",
				Args:      []any{map[string]any{"theme": map[string]any{"newValue": "dark"}}},
			}),
			wantType:     TypeEvent,
			wantName:     MessageChromeAPIEvent,
			wantDataType: ChromeAPIEvent{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.V != CurrentVersion {
				t.Fatalf("unexpected protocol version: got %d, want %d", tc.got.V, CurrentVersion)
			}
			if tc.got.T != tc.wantType {
				t.Fatalf("unexpected message type: got %q, want %q", tc.got.T, tc.wantType)
			}
			if tc.got.Name != tc.wantName {
				t.Fatalf("unexpected message name: got %q, want %q", tc.got.Name, tc.wantName)
			}
			if tc.got.Src != src {
				t.Fatalf("unexpected source: got %+v, want %+v", tc.got.Src, src)
			}
			if reflect.TypeOf(tc.got.Data) != reflect.TypeOf(tc.wantDataType) {
				t.Fatalf("unexpected payload type: got %T, want %T", tc.got.Data, tc.wantDataType)
			}
		})
	}
}

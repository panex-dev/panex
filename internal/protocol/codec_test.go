package protocol

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := NewHello(
		Source{
			Role: SourceDevAgent,
			ID:   "agent-1",
		},
		Hello{
			ProtocolVersion:       CurrentVersion,
			ClientKind:            "dev-agent",
			ClientVersion:         "dev",
			ExtensionID:           "default",
			CapabilitiesRequested: []string{"command.reload"},
		},
	)

	raw, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	decoded, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("DecodeEnvelope() returned error: %v", err)
	}

	if decoded.V != original.V {
		t.Fatalf("unexpected version: got %d, want %d", decoded.V, original.V)
	}
	if decoded.T != original.T {
		t.Fatalf("unexpected type: got %q, want %q", decoded.T, original.T)
	}
	if decoded.Name != original.Name {
		t.Fatalf("unexpected name: got %q, want %q", decoded.Name, original.Name)
	}
	if decoded.Src != original.Src {
		t.Fatalf("unexpected source: got %+v, want %+v", decoded.Src, original.Src)
	}
	if _, ok := decoded.Data.(msgpack.RawMessage); !ok {
		t.Fatalf("expected decoded payload to stay raw msgpack bytes, got %T", decoded.Data)
	}

	var hello Hello
	if err := DecodePayload(decoded.Data, &hello); err != nil {
		t.Fatalf("DecodePayload() returned error: %v", err)
	}
	if hello.ProtocolVersion != CurrentVersion {
		t.Fatalf("unexpected protocol version: got %d, want %d", hello.ProtocolVersion, CurrentVersion)
	}
	if hello.ClientKind != "dev-agent" {
		t.Fatalf("unexpected client kind: got %q, want %q", hello.ClientKind, "dev-agent")
	}
	if hello.ClientVersion != "dev" {
		t.Fatalf("unexpected client version: got %q, want %q", hello.ClientVersion, "dev")
	}
	if hello.ExtensionID != "default" {
		t.Fatalf("unexpected extension id: got %q, want %q", hello.ExtensionID, "default")
	}
	if len(hello.CapabilitiesRequested) != 1 || hello.CapabilitiesRequested[0] != "command.reload" {
		t.Fatalf("unexpected capabilities requested: %+v", hello.CapabilitiesRequested)
	}
}

func TestDecodePayloadTypedCompatibility(t *testing.T) {
	raw := Hello{
		ProtocolVersion:       CurrentVersion,
		ClientKind:            "dev-agent",
		ClientVersion:         "dev",
		ExtensionID:           "default",
		CapabilitiesRequested: []string{"command.reload"},
	}

	var hello Hello
	if err := DecodePayload(raw, &hello); err != nil {
		t.Fatalf("DecodePayload(typed) returned error: %v", err)
	}

	if hello.ProtocolVersion != raw.ProtocolVersion {
		t.Fatalf("unexpected protocol version: got %d, want %d", hello.ProtocolVersion, raw.ProtocolVersion)
	}
	if hello.ClientKind != raw.ClientKind {
		t.Fatalf("unexpected client kind: got %q, want %q", hello.ClientKind, raw.ClientKind)
	}
	if hello.ClientVersion != raw.ClientVersion {
		t.Fatalf("unexpected client version: got %q, want %q", hello.ClientVersion, raw.ClientVersion)
	}
	if hello.ExtensionID != raw.ExtensionID {
		t.Fatalf("unexpected extension id: got %q, want %q", hello.ExtensionID, raw.ExtensionID)
	}
	if len(hello.CapabilitiesRequested) != len(raw.CapabilitiesRequested) {
		t.Fatalf("unexpected decoded payload: got %+v, want %+v", hello, raw)
	}
	for i := range raw.CapabilitiesRequested {
		if hello.CapabilitiesRequested[i] != raw.CapabilitiesRequested[i] {
			t.Fatalf("unexpected decoded payload: got %+v, want %+v", hello, raw)
		}
	}
}

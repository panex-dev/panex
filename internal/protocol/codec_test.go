package protocol

import (
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := NewHello(
		Source{
			Role: SourceDevAgent,
			ID:   "agent-1",
		},
		Hello{
			ProtocolVersion: CurrentVersion,
			Capabilities:    []string{"reload"},
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

	var hello Hello
	if err := DecodePayload(decoded.Data, &hello); err != nil {
		t.Fatalf("DecodePayload() returned error: %v", err)
	}
	if hello.ProtocolVersion != CurrentVersion {
		t.Fatalf("unexpected protocol version: got %d, want %d", hello.ProtocolVersion, CurrentVersion)
	}
	if len(hello.Capabilities) != 1 || hello.Capabilities[0] != "reload" {
		t.Fatalf("unexpected capabilities: %+v", hello.Capabilities)
	}
}

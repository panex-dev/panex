package daemon

import (
	"slices"
	"testing"

	"github.com/panex-dev/panex/internal/protocol"
)

func TestTypeScriptClientCapabilityParity(t *testing.T) {
	gotNegotiable := append([]string(nil), daemonCapabilities...)
	wantNegotiable := messageNamesToStrings(protocol.NegotiableCapabilityNames)
	slices.Sort(gotNegotiable)
	slices.Sort(wantNegotiable)
	if !slices.Equal(gotNegotiable, wantNegotiable) {
		t.Fatalf("negotiable capability drift:\n  daemon=%v\n  protocol=%v", gotNegotiable, wantNegotiable)
	}

	gotClientKinds := clientKindsToStrings(protocol.FirstPartyClientKinds)
	wantClientKinds := []string{
		string(protocol.ClientKindDevAgent),
		string(protocol.ClientKindInspector),
		string(protocol.ClientKindChromeSim),
	}
	if !slices.Equal(gotClientKinds, wantClientKinds) {
		t.Fatalf("first-party client kind drift:\n  got=%v\n  want=%v", gotClientKinds, wantClientKinds)
	}

	for _, clientKind := range wantClientKinds {
		got := append([]string(nil), supportedCapabilitiesForClientKind(clientKind)...)
		want := expectedCapabilitiesForClientKind(clientKind)
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("capability scope drift for %q:\n  daemon=%v\n  protocol=%v", clientKind, got, want)
		}
	}
}

func messageNamesToStrings(names []protocol.MessageName) []string {
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, string(name))
	}

	return values
}

func clientKindsToStrings(kinds []protocol.ClientKind) []string {
	values := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, string(kind))
	}

	return values
}

func expectedCapabilitiesForClientKind(clientKind string) []string {
	switch clientKind {
	case string(protocol.ClientKindDevAgent):
		return []string{string(protocol.MessageCommandReload)}
	case string(protocol.ClientKindChromeSim):
		return []string{
			string(protocol.MessageChromeAPICall),
			string(protocol.MessageChromeAPIEvent),
			string(protocol.MessageStorageDiff),
		}
	case string(protocol.ClientKindInspector):
		return messageNamesToStrings(protocol.NegotiableCapabilityNames)
	default:
		return nil
	}
}

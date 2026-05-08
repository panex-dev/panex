package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestTypeScriptClientCapabilityParity(t *testing.T) {
	source := loadSharedProtocolSource(t)

	gotNegotiable := parseTSStringArray(t, source, "negotiableCapabilityNames")
	wantNegotiable := append([]string(nil), daemonCapabilities...)
	slices.Sort(gotNegotiable)
	slices.Sort(wantNegotiable)
	if !slices.Equal(gotNegotiable, wantNegotiable) {
		t.Fatalf("negotiable capability drift:\n  ts=%v\n  go=%v", gotNegotiable, wantNegotiable)
	}

	gotClientKinds := parseTSStringArray(t, source, "firstPartyClientKinds")
	wantClientKinds := []string{"dev-agent", "inspector", "chrome-sim"}
	if !slices.Equal(gotClientKinds, wantClientKinds) {
		t.Fatalf("first-party client kind drift:\n  ts=%v\n  go=%v", gotClientKinds, wantClientKinds)
	}

	gotRequests := parseTSStringArrayMap(t, source, "firstPartyRequestedCapabilities")
	for _, clientKind := range wantClientKinds {
		got, ok := gotRequests[clientKind]
		if !ok {
			t.Fatalf("first-party request set %q missing from TypeScript contract", clientKind)
		}

		want := append([]string(nil), supportedCapabilitiesForClientKind(clientKind)...)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("capability scope drift for %q:\n  ts=%v\n  go=%v", clientKind, got, want)
		}
	}
}

func loadSharedProtocolSource(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve parity test path: runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	tsPath := filepath.Join(repoRoot, "shared", "protocol", "src", "index.ts")

	raw, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("read shared protocol source %q: %v", tsPath, err)
	}

	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func parseTSStringArray(t *testing.T, source, constName string) []string {
	t.Helper()

	arrayRE := regexp.MustCompile(fmt.Sprintf(`(?s)export const %s = \[(.*?)\] as const;`, regexp.QuoteMeta(constName)))
	match := arrayRE.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("parse ts array %q: declaration not found", constName)
	}

	itemRE := regexp.MustCompile(`"([^"]+)"`)
	itemMatches := itemRE.FindAllStringSubmatch(match[1], -1)
	if len(itemMatches) == 0 {
		t.Fatalf("parse ts array %q: no string values found", constName)
	}

	values := make([]string, 0, len(itemMatches))
	for _, item := range itemMatches {
		values = append(values, item[1])
	}

	return values
}

func parseTSStringArrayMap(t *testing.T, source, constName string) map[string][]string {
	t.Helper()

	mapRE := regexp.MustCompile(
		fmt.Sprintf(`(?s)export const %s(?:\s*:[^=]+)?\s*=\s*\{(.*?)\}\s*as const`, regexp.QuoteMeta(constName)),
	)
	match := mapRE.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("parse ts array map %q: declaration not found", constName)
	}

	entryRE := regexp.MustCompile(`(?ms)^\s*(?:"([^"]+)"|([A-Za-z0-9_.-]+))\s*:\s*\[(.*?)\],?\s*$`)
	entryMatches := entryRE.FindAllStringSubmatch(match[1], -1)
	if len(entryMatches) == 0 {
		t.Fatalf("parse ts array map %q: no entries found", constName)
	}

	itemRE := regexp.MustCompile(`"([^"]+)"`)
	values := make(map[string][]string, len(entryMatches))
	for _, entry := range entryMatches {
		key := entry[1]
		if key == "" {
			key = entry[2]
		}

		itemMatches := itemRE.FindAllStringSubmatch(entry[3], -1)
		items := make([]string, 0, len(itemMatches))
		for _, item := range itemMatches {
			items = append(items, item[1])
		}

		values[key] = items
	}

	return values
}

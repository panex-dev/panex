package protocol

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var tsProtocolVersionRE = regexp.MustCompile(`(?m)^export const PROTOCOL_VERSION = (\d+);$`)

func TestTypeScriptProtocolParity(t *testing.T) {
	source := loadSharedProtocolSource(t)

	if got, want := parseTSProtocolVersion(t, source), int(CurrentVersion); got != want {
		t.Fatalf("protocol version drift: ts=%d go=%d", got, want)
	}

	if got, want := parseTSStringArray(t, source, "envelopeTypes"), []string{
		string(TypeLifecycle),
		string(TypeEvent),
		string(TypeCommand),
	}; !slices.Equal(got, want) {
		t.Fatalf("envelope type drift:\n  ts=%v\n  go=%v", got, want)
	}

	if got, want := parseTSStringArray(t, source, "sourceRoles"), []string{
		string(SourceDaemon),
		string(SourceDevAgent),
		string(SourceInspector),
	}; !slices.Equal(got, want) {
		t.Fatalf("source role drift:\n  ts=%v\n  go=%v", got, want)
	}

	if got, want := parseTSStringArray(t, source, "envelopeNames"), []string{
		string(MessageHello),
		string(MessageHelloAck),
		string(MessageBuildComplete),
		string(MessageContextLog),
		string(MessageCommandReload),
		string(MessageQueryEvents),
		string(MessageQueryResult),
		string(MessageQueryStorage),
		string(MessageStorageResult),
		string(MessageStorageDiff),
		string(MessageStorageSet),
		string(MessageStorageRemove),
		string(MessageStorageClear),
		string(MessageChromeAPICall),
		string(MessageChromeAPIResult),
		string(MessageChromeAPIEvent),
	}; !slices.Equal(got, want) {
		t.Fatalf("message name drift:\n  ts=%v\n  go=%v", got, want)
	}

	gotMapping := parseTSStringMap(t, source, "messageTypeByName")
	wantMapping := make(map[string]string, len(messageTypeByName))
	for name, messageType := range messageTypeByName {
		wantMapping[string(name)] = string(messageType)
	}

	if !maps.Equal(gotMapping, wantMapping) {
		t.Fatalf("message-type mapping drift:\n  ts=%s\n  go=%s", formatMap(gotMapping), formatMap(wantMapping))
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

	return string(raw)
}

func parseTSProtocolVersion(t *testing.T, source string) int {
	t.Helper()

	match := tsProtocolVersionRE.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatal("parse ts protocol version: PROTOCOL_VERSION constant not found")
	}

	version, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("parse ts protocol version %q: %v", match[1], err)
	}

	return version
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

func parseTSStringMap(t *testing.T, source, constName string) map[string]string {
	t.Helper()

	mapRE := regexp.MustCompile(
		fmt.Sprintf(`(?s)export const %s(?:\s*:[^=]+)?\s*=\s*\{(.*?)\};`, regexp.QuoteMeta(constName)),
	)
	match := mapRE.FindStringSubmatch(source)
	if len(match) != 2 {
		t.Fatalf("parse ts map %q: declaration not found", constName)
	}

	entryRE := regexp.MustCompile(`(?m)^\s*(?:"([^"]+)"|([A-Za-z0-9_.-]+))\s*:\s*"([^"]+)",?\s*$`)
	entryMatches := entryRE.FindAllStringSubmatch(match[1], -1)
	if len(entryMatches) == 0 {
		t.Fatalf("parse ts map %q: no entries found", constName)
	}

	values := make(map[string]string, len(entryMatches))
	for _, entry := range entryMatches {
		key := entry[1]
		if key == "" {
			key = entry[2]
		}
		values[key] = entry[3]
	}

	return values
}

func formatMap(value map[string]string) string {
	if len(value) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%q:%q", key, value[key]))
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

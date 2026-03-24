package protocol

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
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

func TestPayloadFieldShapeParity(t *testing.T) {
	source := loadSharedProtocolSource(t)

	// Registry of Go structs that must match TypeScript interfaces.
	// The map key is the shared name (e.g. "Hello") used in both Go and TS.
	registry := map[string]reflect.Type{
		"Source":             reflect.TypeOf(Source{}),
		"Envelope":           reflect.TypeOf(Envelope{}),
		"Hello":              reflect.TypeOf(Hello{}),
		"HelloAck":           reflect.TypeOf(HelloAck{}),
		"BuildComplete":      reflect.TypeOf(BuildComplete{}),
		"ContextLog":         reflect.TypeOf(ContextLog{}),
		"CommandReload":      reflect.TypeOf(CommandReload{}),
		"QueryEvents":        reflect.TypeOf(QueryEvents{}),
		"EventSnapshot":      reflect.TypeOf(EventSnapshot{}),
		"QueryEventsResult":  reflect.TypeOf(QueryEventsResult{}),
		"QueryStorage":       reflect.TypeOf(QueryStorage{}),
		"StorageSnapshot":    reflect.TypeOf(StorageSnapshot{}),
		"QueryStorageResult": reflect.TypeOf(QueryStorageResult{}),
		"StorageChange":      reflect.TypeOf(StorageChange{}),
		"StorageDiff":        reflect.TypeOf(StorageDiff{}),
		"StorageSet":         reflect.TypeOf(StorageSet{}),
		"StorageRemove":      reflect.TypeOf(StorageRemove{}),
		"StorageClear":       reflect.TypeOf(StorageClear{}),
		"ChromeAPICall":      reflect.TypeOf(ChromeAPICall{}),
		"ChromeAPIResult":    reflect.TypeOf(ChromeAPIResult{}),
		"ChromeAPIEvent":     reflect.TypeOf(ChromeAPIEvent{}),
	}

	tsInterfaces := parseTSInterfaces(t, source)

	for name, goType := range registry {
		tsFields, ok := tsInterfaces[name]
		if !ok {
			t.Errorf("interface %q: present in Go but not found in TypeScript", name)
			continue
		}

		goFields := goMsgpackFieldNames(t, name, goType)

		if !slices.Equal(goFields, tsFields) {
			t.Errorf("field shape drift for %q:\n  go=%v\n  ts=%v", name, goFields, tsFields)
		}
	}
}

// goMsgpackFieldNames returns the sorted msgpack tag names for a struct type.
func goMsgpackFieldNames(t *testing.T, name string, rt reflect.Type) []string {
	t.Helper()

	fields := make([]string, 0, rt.NumField())
	for i := range rt.NumField() {
		tag := rt.Field(i).Tag.Get("msgpack")
		if tag == "" || tag == "-" {
			t.Fatalf("struct %q field %d (%s): missing or ignored msgpack tag",
				name, i, rt.Field(i).Name)
		}
		// Strip options like ",omitempty".
		tagName, _, _ := strings.Cut(tag, ",")
		fields = append(fields, tagName)
	}

	slices.Sort(fields)
	return fields
}

// parseTSInterfaces extracts all `export interface Name { ... }` blocks and
// returns a map from interface name to sorted property names.
func parseTSInterfaces(t *testing.T, source string) map[string][]string {
	t.Helper()

	// Match interface blocks. The generic parameter (e.g. <TData = unknown>)
	// is optional. We use a non-greedy match up to the closing brace at column 0.
	interfaceRE := regexp.MustCompile(`(?ms)^export interface (\w+)(?:<[^>]+>)?\s*\{(.*?)^\}`)
	matches := interfaceRE.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		t.Fatal("parse ts interfaces: no interface blocks found")
	}

	// Match property names: lines like "  field_name: type;" or "  field_name?: type;"
	propRE := regexp.MustCompile(`(?m)^\s+(\w+)\??:\s+`)

	result := make(map[string][]string, len(matches))
	for _, m := range matches {
		name := m[1]
		body := m[2]

		propMatches := propRE.FindAllStringSubmatch(body, -1)
		fields := make([]string, 0, len(propMatches))
		for _, pm := range propMatches {
			fields = append(fields, pm[1])
		}

		slices.Sort(fields)
		result[name] = fields
	}

	return result
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

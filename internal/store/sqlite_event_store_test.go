package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/panex-dev/panex/internal/protocol"
)

func TestNewSQLiteEventStoreValidation(t *testing.T) {
	_, err := NewSQLiteEventStore(" ")
	if err == nil {
		t.Fatal("expected validation error for blank event store path")
	}
}

func TestSQLiteEventStoreAppendAndRecent(t *testing.T) {
	store := mustNewStore(t)
	defer func() {
		_ = store.Close()
	}()

	ctx := context.Background()

	first := protocol.NewBuildComplete(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"},
		protocol.BuildComplete{
			BuildID:    "build-1",
			Success:    true,
			DurationMS: 11,
		},
	)
	second := protocol.NewCommandReload(
		protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"},
		protocol.CommandReload{
			Reason:  "build.complete",
			BuildID: "build-1",
		},
	)

	if err := store.Append(ctx, first, "default"); err != nil {
		t.Fatalf("Append(first) returned error: %v", err)
	}
	if err := store.Append(ctx, second, "default"); err != nil {
		t.Fatalf("Append(second) returned error: %v", err)
	}

	records, hasMore, err := store.Recent(ctx, 10, 0, "")
	if err != nil {
		t.Fatalf("Recent() returned error: %v", err)
	}
	if hasMore {
		t.Fatal("expected hasMore=false for full result set")
	}
	if len(records) != 2 {
		t.Fatalf("unexpected record count: got %d, want %d", len(records), 2)
	}
	if records[0].ID <= 0 || records[1].ID <= 0 {
		t.Fatalf("unexpected record ids: %d, %d", records[0].ID, records[1].ID)
	}
	if records[0].ID >= records[1].ID {
		t.Fatalf("expected chronological order by id, got %d then %d", records[0].ID, records[1].ID)
	}
	if records[0].Envelope.Name != protocol.MessageBuildComplete {
		t.Fatalf("unexpected first event name: got %q, want %q", records[0].Envelope.Name, protocol.MessageBuildComplete)
	}
	if records[1].Envelope.Name != protocol.MessageCommandReload {
		t.Fatalf("unexpected second event name: got %q, want %q", records[1].Envelope.Name, protocol.MessageCommandReload)
	}
}

func TestSQLiteEventStoreRecentLimit(t *testing.T) {
	store := mustNewStore(t)
	defer func() {
		_ = store.Close()
	}()

	ctx := context.Background()
	source := protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"}
	for i := 0; i < 3; i++ {
		if err := store.Append(ctx, protocol.NewBuildComplete(source, protocol.BuildComplete{
			BuildID:    "build-limit",
			Success:    true,
			DurationMS: int64(i + 1),
		}), "default"); err != nil {
			t.Fatalf("Append(%d) returned error: %v", i, err)
		}
	}

	records, hasMore, err := store.Recent(ctx, 1, 0, "")
	if err != nil {
		t.Fatalf("Recent(limit=1) returned error: %v", err)
	}
	if !hasMore {
		t.Fatal("expected hasMore=true when newer history remains")
	}
	if len(records) != 1 {
		t.Fatalf("unexpected record count: got %d, want %d", len(records), 1)
	}
	var payload protocol.BuildComplete
	if err := protocol.DecodePayload(records[0].Envelope.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(last) returned error: %v", err)
	}
	if payload.DurationMS != 3 {
		t.Fatalf("expected newest payload for limit=1, got duration_ms=%d", payload.DurationMS)
	}
}

func TestSQLiteEventStoreRecentBeforeID(t *testing.T) {
	store := mustNewStore(t)
	defer func() {
		_ = store.Close()
	}()

	ctx := context.Background()
	source := protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"}

	var ids []int64
	for i := 0; i < 4; i++ {
		if err := store.Append(ctx, protocol.NewBuildComplete(source, protocol.BuildComplete{
			BuildID:    "build-before",
			Success:    true,
			DurationMS: int64(i + 1),
		}), "default"); err != nil {
			t.Fatalf("Append(%d) returned error: %v", i, err)
		}

		records, _, err := store.Recent(ctx, 1, 0, "")
		if err != nil {
			t.Fatalf("Recent(latest) after append %d returned error: %v", i, err)
		}
		ids = append(ids, records[0].ID)
	}

	records, hasMore, err := store.Recent(ctx, 2, ids[3], "")
	if err != nil {
		t.Fatalf("Recent(before_id) returned error: %v", err)
	}
	if !hasMore {
		t.Fatal("expected hasMore=true when older history remains before cursor")
	}
	if len(records) != 2 {
		t.Fatalf("unexpected record count: got %d, want %d", len(records), 2)
	}
	if records[0].ID != ids[1] || records[1].ID != ids[2] {
		t.Fatalf("unexpected record ids before cursor: got [%d %d], want [%d %d]", records[0].ID, records[1].ID, ids[1], ids[2])
	}
}

func TestSQLiteEventStoreStorageSnapshotsPersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "events.db")

	store, err := NewSQLiteEventStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteEventStore() returned error: %v", err)
	}

	source := protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"}
	if _, err := store.SetStorageItem(ctx, source, "local", "theme", "dark", "default"); err != nil {
		t.Fatalf("SetStorageItem(local theme) returned error: %v", err)
	}
	if _, err := store.SetStorageItem(ctx, source, "sync", "enabled", true, "default"); err != nil {
		t.Fatalf("SetStorageItem(sync enabled) returned error: %v", err)
	}
	if _, changed, err := store.RemoveStorageItem(ctx, source, "sync", "enabled", "default"); err != nil {
		t.Fatalf("RemoveStorageItem(sync enabled) returned error: %v", err)
	} else if !changed {
		t.Fatal("expected RemoveStorageItem(sync enabled) to report a change")
	}
	if _, err := store.SetStorageItem(ctx, source, "session", "temp", int64(42), "default"); err != nil {
		t.Fatalf("SetStorageItem(session temp) returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	reopened, err := NewSQLiteEventStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteEventStore(reopen) returned error: %v", err)
	}
	defer func() {
		_ = reopened.Close()
	}()

	snapshots, err := reopened.StorageSnapshots(ctx, "", "default")
	if err != nil {
		t.Fatalf("StorageSnapshots() returned error: %v", err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("unexpected snapshot count: got %d, want %d", len(snapshots), 3)
	}
	if got := snapshots[0].Items["theme"]; got != "dark" {
		t.Fatalf("unexpected persisted local theme value: %#v", got)
	}
	if len(snapshots[1].Items) != 0 {
		t.Fatalf("expected sync area to be empty after remove, got %#v", snapshots[1].Items)
	}
	if got := snapshots[2].Items["temp"]; got != int64(42) {
		t.Fatalf("unexpected persisted session temp value: %#v", got)
	}

	records, hasMore, err := reopened.Recent(ctx, 10, 0, "")
	if err != nil {
		t.Fatalf("Recent() after reopen returned error: %v", err)
	}
	if hasMore {
		t.Fatal("expected hasMore=false for small persisted mutation history")
	}
	if len(records) != 4 {
		t.Fatalf("unexpected persisted event count: got %d, want %d", len(records), 4)
	}
	for _, record := range records {
		if record.Envelope.Name != protocol.MessageStorageDiff {
			t.Fatalf("expected only storage.diff events, got %q", record.Envelope.Name)
		}
	}
}

func TestSQLiteEventStoreClearStorageAreaPersistsEmptySnapshot(t *testing.T) {
	ctx := context.Background()
	store := mustNewStore(t)
	defer func() {
		_ = store.Close()
	}()

	source := protocol.Source{Role: protocol.SourceDaemon, ID: "daemon-1"}
	if _, err := store.SetStorageItem(ctx, source, "local", "one", int64(1), "default"); err != nil {
		t.Fatalf("SetStorageItem(local one) returned error: %v", err)
	}
	if _, err := store.SetStorageItem(ctx, source, "local", "two", int64(2), "default"); err != nil {
		t.Fatalf("SetStorageItem(local two) returned error: %v", err)
	}

	diff, changed, err := store.ClearStorageArea(ctx, source, "local", "default")
	if err != nil {
		t.Fatalf("ClearStorageArea(local) returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected ClearStorageArea(local) to report a change")
	}

	var payload protocol.StorageDiff
	if err := protocol.DecodePayload(diff.Data, &payload); err != nil {
		t.Fatalf("DecodePayload(storage.diff clear) returned error: %v", err)
	}
	if len(payload.Changes) != 2 {
		t.Fatalf("unexpected clear diff size: got %d, want %d", len(payload.Changes), 2)
	}
	if payload.Changes[0].Key != "one" || payload.Changes[1].Key != "two" {
		t.Fatalf("unexpected clear diff order: %+v", payload.Changes)
	}

	snapshots, err := store.StorageSnapshots(ctx, "local", "default")
	if err != nil {
		t.Fatalf("StorageSnapshots(local) returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("unexpected local snapshot count: got %d, want %d", len(snapshots), 1)
	}
	if len(snapshots[0].Items) != 0 {
		t.Fatalf("expected cleared local area to be empty, got %#v", snapshots[0].Items)
	}
}

func mustNewStore(t *testing.T) *SQLiteEventStore {
	t.Helper()

	storePath := filepath.Join(t.TempDir(), "events.db")
	store, err := NewSQLiteEventStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteEventStore() returned error: %v", err)
	}

	return store
}

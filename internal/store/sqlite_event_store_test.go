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

	if err := store.Append(ctx, first); err != nil {
		t.Fatalf("Append(first) returned error: %v", err)
	}
	if err := store.Append(ctx, second); err != nil {
		t.Fatalf("Append(second) returned error: %v", err)
	}

	records, err := store.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent() returned error: %v", err)
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
		})); err != nil {
			t.Fatalf("Append(%d) returned error: %v", i, err)
		}
	}

	records, err := store.Recent(ctx, 1)
	if err != nil {
		t.Fatalf("Recent(limit=1) returned error: %v", err)
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

func mustNewStore(t *testing.T) *SQLiteEventStore {
	t.Helper()

	storePath := filepath.Join(t.TempDir(), "events.db")
	store, err := NewSQLiteEventStore(storePath)
	if err != nil {
		t.Fatalf("NewSQLiteEventStore() returned error: %v", err)
	}

	return store
}

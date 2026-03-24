package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/panex-dev/panex/internal/protocol"
)

const (
	defaultRecentLimit = 100
	maxRecentLimit     = 1000
)

type Record struct {
	ID           int64
	RecordedAtMS int64
	Envelope     protocol.Envelope
}

type SQLiteEventStore struct {
	db *sql.DB
}

func NewSQLiteEventStore(path string) (*SQLiteEventStore, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, errors.New("event store path is required")
	}

	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o755); err != nil {
		return nil, fmt.Errorf("create event store directory: %w", err)
	}

	db, err := sql.Open("sqlite", trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite event store: %w", err)
	}

	store := &SQLiteEventStore{db: db}
	if err := store.initialize(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteEventStore) initialize() error {
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("set sqlite journal mode: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		return fmt.Errorf("set sqlite synchronous mode: %w", err)
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS protocol_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			recorded_at_ms INTEGER NOT NULL,
			extension_id TEXT NOT NULL DEFAULT 'default',
			envelope BLOB NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create sqlite schema: %w", err)
	}
	if err := s.migrateProtocolEventsExtensionID(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS storage_items (
			extension_id TEXT NOT NULL DEFAULT 'default',
			area TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			PRIMARY KEY(extension_id, area, key)
		);
	`); err != nil {
		return fmt.Errorf("create sqlite storage schema: %w", err)
	}
	if err := s.migrateStorageItemsExtensionID(); err != nil {
		return err
	}

	return nil
}

func (s *SQLiteEventStore) migrateProtocolEventsExtensionID() error {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('protocol_events') WHERE name = 'extension_id';`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check protocol_events extension_id column: %w", err)
	}
	if count > 0 {
		return nil
	}
	if _, err := s.db.Exec(`ALTER TABLE protocol_events ADD COLUMN extension_id TEXT NOT NULL DEFAULT 'default';`); err != nil {
		return fmt.Errorf("migrate protocol_events: add extension_id: %w", err)
	}
	return nil
}

func (s *SQLiteEventStore) migrateStorageItemsExtensionID() error {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('storage_items') WHERE name = 'extension_id';`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check storage_items extension_id column: %w", err)
	}
	if count > 0 {
		return nil
	}
	// Recreate table with new primary key since SQLite can't alter PKs.
	if _, err := s.db.Exec(`
		ALTER TABLE storage_items RENAME TO storage_items_old;
	`); err != nil {
		return fmt.Errorf("migrate storage_items: rename: %w", err)
	}
	if _, err := s.db.Exec(`
		CREATE TABLE storage_items (
			extension_id TEXT NOT NULL DEFAULT 'default',
			area TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			PRIMARY KEY(extension_id, area, key)
		);
	`); err != nil {
		return fmt.Errorf("migrate storage_items: create new table: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO storage_items(extension_id, area, key, value)
		SELECT 'default', area, key, value FROM storage_items_old;
	`); err != nil {
		return fmt.Errorf("migrate storage_items: copy data: %w", err)
	}
	if _, err := s.db.Exec(`DROP TABLE storage_items_old;`); err != nil {
		return fmt.Errorf("migrate storage_items: drop old table: %w", err)
	}
	return nil
}

func (s *SQLiteEventStore) Append(ctx context.Context, envelope protocol.Envelope, extensionID string) error {
	encoded, err := protocol.Encode(envelope)
	if err != nil {
		return fmt.Errorf("encode event envelope: %w", err)
	}

	extID := normalizeExtensionIDParam(extensionID)
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO protocol_events(recorded_at_ms, extension_id, envelope) VALUES(?, ?, ?);`,
		time.Now().UnixMilli(),
		extID,
		encoded,
	)
	if err != nil {
		return fmt.Errorf("insert event envelope: %w", err)
	}

	return nil
}

func normalizeExtensionIDParam(extensionID string) string {
	trimmed := strings.TrimSpace(extensionID)
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

func (s *SQLiteEventStore) Recent(ctx context.Context, limit int, beforeID int64, extensionID string) ([]Record, bool, error) {
	boundedLimit := boundLimit(limit)

	query := `SELECT id, recorded_at_ms, envelope FROM protocol_events`
	args := []any{}
	var conditions []string
	if beforeID > 0 {
		conditions = append(conditions, `id < ?`)
		args = append(args, beforeID)
	}
	extID := strings.TrimSpace(extensionID)
	if extID != "" {
		conditions = append(conditions, `extension_id = ?`)
		args = append(args, extID)
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}
	query += ` ORDER BY id DESC LIMIT ?;`
	args = append(args, boundedLimit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query recent events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	records := make([]Record, 0, boundedLimit+1)
	for rows.Next() {
		var (
			record      Record
			rawEnvelope []byte
		)

		if err := rows.Scan(&record.ID, &record.RecordedAtMS, &rawEnvelope); err != nil {
			return nil, false, fmt.Errorf("scan recent event row: %w", err)
		}

		record.Envelope, err = protocol.DecodeEnvelope(rawEnvelope)
		if err != nil {
			return nil, false, fmt.Errorf("decode recent event envelope: %w", err)
		}

		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate recent event rows: %w", err)
	}

	hasMore := false
	if len(records) > boundedLimit {
		hasMore = true
		records = records[:boundedLimit]
	}

	// Query uses DESC for index locality; reverse so consumers get chronological order.
	for left, right := 0, len(records)-1; left < right; left, right = left+1, right-1 {
		records[left], records[right] = records[right], records[left]
	}

	return records, hasMore, nil
}

func (s *SQLiteEventStore) Close() error {
	return s.db.Close()
}

func boundLimit(limit int) int {
	if limit <= 0 {
		return defaultRecentLimit
	}
	if limit > maxRecentLimit {
		return maxRecentLimit
	}

	return limit
}

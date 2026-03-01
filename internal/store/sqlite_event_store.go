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
			envelope BLOB NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create sqlite schema: %w", err)
	}

	return nil
}

func (s *SQLiteEventStore) Append(ctx context.Context, envelope protocol.Envelope) error {
	encoded, err := protocol.Encode(envelope)
	if err != nil {
		return fmt.Errorf("encode event envelope: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO protocol_events(recorded_at_ms, envelope) VALUES(?, ?);`,
		time.Now().UnixMilli(),
		encoded,
	)
	if err != nil {
		return fmt.Errorf("insert event envelope: %w", err)
	}

	return nil
}

func (s *SQLiteEventStore) Recent(ctx context.Context, limit int) ([]Record, error) {
	boundedLimit := boundLimit(limit)

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, recorded_at_ms, envelope FROM protocol_events ORDER BY id DESC LIMIT ?;`,
		boundedLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	records := make([]Record, 0, boundedLimit)
	for rows.Next() {
		var (
			record      Record
			rawEnvelope []byte
		)

		if err := rows.Scan(&record.ID, &record.RecordedAtMS, &rawEnvelope); err != nil {
			return nil, fmt.Errorf("scan recent event row: %w", err)
		}

		record.Envelope, err = protocol.DecodeEnvelope(rawEnvelope)
		if err != nil {
			return nil, fmt.Errorf("decode recent event envelope: %w", err)
		}

		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent event rows: %w", err)
	}

	// Query uses DESC for index locality; reverse so consumers get chronological order.
	for left, right := 0, len(records)-1; left < right; left, right = left+1, right-1 {
		records[left], records[right] = records[right], records[left]
	}

	return records, nil
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

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/panex-dev/panex/internal/protocol"
)

var storageAreaOrder = []string{"local", "sync", "session"}

func (s *SQLiteEventStore) StorageSnapshots(ctx context.Context, area string, extensionID string) ([]protocol.StorageSnapshot, error) {
	extID := normalizeExtensionIDParam(extensionID)
	trimmed := strings.TrimSpace(area)
	if trimmed == "" {
		snapshots := make([]protocol.StorageSnapshot, 0, len(storageAreaOrder))
		for _, storageArea := range storageAreaOrder {
			items, err := s.loadStorageItems(ctx, storageArea, extID)
			if err != nil {
				return nil, err
			}
			snapshots = append(snapshots, protocol.StorageSnapshot{
				Area:  storageArea,
				Items: items,
			})
		}
		return snapshots, nil
	}

	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return nil, err
	}

	items, err := s.loadStorageItems(ctx, normalizedArea, extID)
	if err != nil {
		return nil, err
	}

	return []protocol.StorageSnapshot{{
		Area:  normalizedArea,
		Items: items,
	}}, nil
}

func (s *SQLiteEventStore) SetStorageItem(
	ctx context.Context,
	source protocol.Source,
	area string,
	key string,
	value any,
	extensionID string,
) (protocol.Envelope, error) {
	normalizedArea, normalizedKey, err := normalizeStorageTarget(area, key)
	if err != nil {
		return protocol.Envelope{}, err
	}
	extID := normalizeExtensionIDParam(extensionID)

	encodedValue, err := msgpack.Marshal(value)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("encode storage value: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.Envelope{}, fmt.Errorf("begin sqlite storage set transaction: %w", err)
	}
	defer rollbackTx(tx)

	var (
		oldValue    any
		hadOldValue bool
		rawOldValue []byte
	)
	err = tx.QueryRowContext(
		ctx,
		`SELECT value FROM storage_items WHERE extension_id = ? AND area = ? AND key = ?;`,
		extID,
		normalizedArea,
		normalizedKey,
	).Scan(&rawOldValue)
	switch {
	case err == nil:
		hadOldValue = true
		oldValue, err = decodeStorageValue(rawOldValue)
		if err != nil {
			return protocol.Envelope{}, err
		}
	case errors.Is(err, sql.ErrNoRows):
		hadOldValue = false
	default:
		return protocol.Envelope{}, fmt.Errorf("query existing storage item: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO storage_items(extension_id, area, key, value) VALUES(?, ?, ?, ?)
		 ON CONFLICT(extension_id, area, key) DO UPDATE SET value = excluded.value;`,
		extID,
		normalizedArea,
		normalizedKey,
		encodedValue,
	); err != nil {
		return protocol.Envelope{}, fmt.Errorf("upsert storage item: %w", err)
	}

	change := protocol.StorageChange{
		Key:      normalizedKey,
		NewValue: value,
	}
	if hadOldValue {
		change.OldValue = oldValue
	}

	diff := protocol.NewStorageDiff(source, protocol.StorageDiff{
		Area:    normalizedArea,
		Changes: []protocol.StorageChange{change},
	})
	if err := appendEventTx(ctx, tx, diff, extID); err != nil {
		return protocol.Envelope{}, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.Envelope{}, fmt.Errorf("commit sqlite storage set transaction: %w", err)
	}

	return diff, nil
}

func (s *SQLiteEventStore) RemoveStorageItem(
	ctx context.Context,
	source protocol.Source,
	area string,
	key string,
	extensionID string,
) (protocol.Envelope, bool, error) {
	normalizedArea, normalizedKey, err := normalizeStorageTarget(area, key)
	if err != nil {
		return protocol.Envelope{}, false, err
	}
	extID := normalizeExtensionIDParam(extensionID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("begin sqlite storage remove transaction: %w", err)
	}
	defer rollbackTx(tx)

	var rawOldValue []byte
	err = tx.QueryRowContext(
		ctx,
		`SELECT value FROM storage_items WHERE extension_id = ? AND area = ? AND key = ?;`,
		extID,
		normalizedArea,
		normalizedKey,
	).Scan(&rawOldValue)
	if errors.Is(err, sql.ErrNoRows) {
		return protocol.Envelope{}, false, nil
	}
	if err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("query existing storage item: %w", err)
	}

	oldValue, err := decodeStorageValue(rawOldValue)
	if err != nil {
		return protocol.Envelope{}, false, err
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM storage_items WHERE extension_id = ? AND area = ? AND key = ?;`,
		extID,
		normalizedArea,
		normalizedKey,
	); err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("delete storage item: %w", err)
	}

	diff := protocol.NewStorageDiff(source, protocol.StorageDiff{
		Area: normalizedArea,
		Changes: []protocol.StorageChange{{
			Key:      normalizedKey,
			OldValue: oldValue,
		}},
	})
	if err := appendEventTx(ctx, tx, diff, extID); err != nil {
		return protocol.Envelope{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("commit sqlite storage remove transaction: %w", err)
	}

	return diff, true, nil
}

func (s *SQLiteEventStore) ClearStorageArea(
	ctx context.Context,
	source protocol.Source,
	area string,
	extensionID string,
) (protocol.Envelope, bool, error) {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return protocol.Envelope{}, false, err
	}
	extID := normalizeExtensionIDParam(extensionID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("begin sqlite storage clear transaction: %w", err)
	}
	defer rollbackTx(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT key, value FROM storage_items WHERE extension_id = ? AND area = ? ORDER BY key;`,
		extID,
		normalizedArea,
	)
	if err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("query storage items for clear: %w", err)
	}

	changes := make([]protocol.StorageChange, 0, 8)
	for rows.Next() {
		var (
			key      string
			rawValue []byte
		)
		if err := rows.Scan(&key, &rawValue); err != nil {
			_ = rows.Close()
			return protocol.Envelope{}, false, fmt.Errorf("scan storage item for clear: %w", err)
		}
		oldValue, err := decodeStorageValue(rawValue)
		if err != nil {
			_ = rows.Close()
			return protocol.Envelope{}, false, err
		}
		changes = append(changes, protocol.StorageChange{
			Key:      key,
			OldValue: oldValue,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return protocol.Envelope{}, false, fmt.Errorf("iterate storage items for clear: %w", err)
	}
	if err := rows.Close(); err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("close storage clear rows: %w", err)
	}

	if len(changes) == 0 {
		return protocol.Envelope{}, false, nil
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM storage_items WHERE extension_id = ? AND area = ?;`,
		extID,
		normalizedArea,
	); err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("clear storage area: %w", err)
	}

	diff := protocol.NewStorageDiff(source, protocol.StorageDiff{
		Area:    normalizedArea,
		Changes: changes,
	})
	if err := appendEventTx(ctx, tx, diff, extID); err != nil {
		return protocol.Envelope{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return protocol.Envelope{}, false, fmt.Errorf("commit sqlite storage clear transaction: %w", err)
	}

	return diff, true, nil
}

func (s *SQLiteEventStore) loadStorageItems(ctx context.Context, area string, extensionID string) (map[string]any, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT key, value FROM storage_items WHERE extension_id = ? AND area = ? ORDER BY key;`,
		extensionID,
		area,
	)
	if err != nil {
		return nil, fmt.Errorf("query storage items: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make(map[string]any)
	for rows.Next() {
		var (
			key      string
			rawValue []byte
		)
		if err := rows.Scan(&key, &rawValue); err != nil {
			return nil, fmt.Errorf("scan storage item row: %w", err)
		}
		value, err := decodeStorageValue(rawValue)
		if err != nil {
			return nil, err
		}
		items[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate storage item rows: %w", err)
	}

	return items, nil
}

func appendEventTx(ctx context.Context, tx *sql.Tx, envelope protocol.Envelope, extensionID string) error {
	encoded, err := protocol.Encode(envelope)
	if err != nil {
		return fmt.Errorf("encode event envelope: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO protocol_events(recorded_at_ms, extension_id, envelope) VALUES(?, ?, ?);`,
		time.Now().UnixMilli(),
		extensionID,
		encoded,
	); err != nil {
		return fmt.Errorf("insert event envelope: %w", err)
	}

	return nil
}

func decodeStorageValue(raw []byte) (any, error) {
	var value any
	if err := msgpack.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode storage value: %w", err)
	}
	return value, nil
}

func normalizeStorageArea(area string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(area))
	switch normalized {
	case "local", "sync", "session":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported storage area %q", area)
	}
}

func normalizeStorageTarget(area string, key string) (string, string, error) {
	normalizedArea, err := normalizeStorageArea(area)
	if err != nil {
		return "", "", err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return "", "", errors.New("storage key is required")
	}

	return normalizedArea, normalizedKey, nil
}

func rollbackTx(tx *sql.Tx) {
	_ = tx.Rollback()
}

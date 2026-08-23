package store

import (
	"context"
	"errors"
	"time"
)

// PairingCode is the stored, hashed form of a short-lived phone pairing code.
type PairingCode struct {
	CodeHash  string
	Route     string
	ExpiresAt string
	CreatedAt string
}

// SavePairingCode stores one hashed pairing code. The plaintext code never reaches the DB.
func (s *Store) SavePairingCode(ctx context.Context, codeHash, route string, expiresAt time.Time) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO pairing_codes (code_hash, route, expires_at, created_at)
		VALUES (?, ?, ?, ?)`,
		codeHash, route, expiresAt.UTC().Format(time.RFC3339), s.utcNow(),
	); err != nil {
		return classify("save pairing code", err)
	}
	return nil
}

// RedeemPairingCode burns a code and returns its route when it exists and has not expired.
// Unknown, expired and already-used codes all return ErrNotFound.
func (s *Store) RedeemPairingCode(ctx context.Context, codeHash string, now time.Time) (string, error) {
	var route string
	var expiresRaw string
	expired := false

	err := s.tx(ctx, func(tx execer) error {
		if err := tx.QueryRowContext(ctx, `
			SELECT route, expires_at
			FROM pairing_codes
			WHERE code_hash = ?`, codeHash).Scan(&route, &expiresRaw); err != nil {
			return classify("read pairing code", err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM pairing_codes WHERE code_hash = ?`, codeHash); err != nil {
			return classify("burn pairing code", err)
		}

		expiresAt, err := time.Parse(time.RFC3339, expiresRaw)
		if err != nil {
			return classify("parse pairing code expiry", err)
		}
		if !now.UTC().Before(expiresAt) {
			expired = true
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	if expired {
		return "", ErrNotFound
	}

	return route, nil
}

// SweepExpiredPairingCodes deletes stale pairing codes and returns how many rows were removed.
func (s *Store) SweepExpiredPairingCodes(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM pairing_codes WHERE expires_at <= ?`,
		now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, classify("sweep expired pairing codes", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, classify("count swept pairing codes", err)
	}
	return n, nil
}

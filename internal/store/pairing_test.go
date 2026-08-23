package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPairingCodeRedeemsOnceAndStoresHashOnly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	expiresAt := fixedNow.Add(90 * time.Second)
	if err := s.SavePairingCode(ctx, "hash-only", "/goals?project=synapse", expiresAt); err != nil {
		t.Fatalf("save pairing code: %v", err)
	}

	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT code_hash FROM pairing_codes`).Scan(&stored); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if stored != "hash-only" || stored == "plaintext-code" {
		t.Fatalf("stored code_hash = %q, want hash-only and never plaintext", stored)
	}

	route, err := s.RedeemPairingCode(ctx, "hash-only", fixedNow)
	if err != nil {
		t.Fatalf("redeem pairing code: %v", err)
	}
	if route != "/goals?project=synapse" {
		t.Fatalf("route = %q, want /goals?project=synapse", route)
	}

	if _, err := s.RedeemPairingCode(ctx, "hash-only", fixedNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second redeem error = %v, want ErrNotFound", err)
	}
}

func TestExpiredPairingCodeFailsAndIsBurned(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SavePairingCode(ctx, "expired", "/goals", fixedNow.Add(-time.Second)); err != nil {
		t.Fatalf("save pairing code: %v", err)
	}

	if _, err := s.RedeemPairingCode(ctx, "expired", fixedNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("redeem expired error = %v, want ErrNotFound", err)
	}
	if got := countRows(t, s, "pairing_codes"); got != 0 {
		t.Fatalf("pairing_codes rows = %d, want burned expired row", got)
	}
}

func TestSweepExpiredPairingCodes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SavePairingCode(ctx, "old", "/old", fixedNow.Add(-time.Second)); err != nil {
		t.Fatalf("save old pairing code: %v", err)
	}
	if err := s.SavePairingCode(ctx, "new", "/new", fixedNow.Add(time.Minute)); err != nil {
		t.Fatalf("save new pairing code: %v", err)
	}

	n, err := s.SweepExpiredPairingCodes(ctx, fixedNow)
	if err != nil {
		t.Fatalf("sweep expired pairing codes: %v", err)
	}
	if n != 1 {
		t.Fatalf("swept = %d, want 1", n)
	}

	var route string
	if err := s.db.QueryRowContext(ctx, `SELECT route FROM pairing_codes WHERE code_hash = 'new'`).Scan(&route); err != nil {
		t.Fatalf("read remaining pairing code: %v", err)
	}
	if route != "/new" {
		t.Fatalf("remaining route = %q, want /new", route)
	}
}

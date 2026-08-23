-- 007_pairing_codes.sql
--
-- Short-lived, one-use pairing codes for phone handoff. Codes are stored as hashes so a copied
-- database or backup cannot be used to redeem a still-live QR.

CREATE TABLE pairing_codes (
    code_hash  TEXT PRIMARY KEY CHECK (length(trim(code_hash)) > 0),
    route      TEXT NOT NULL CHECK (length(trim(route)) > 0),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_pairing_codes_expires_at ON pairing_codes (expires_at);

PRAGMA foreign_key_check;

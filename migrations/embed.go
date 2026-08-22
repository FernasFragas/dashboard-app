// Package migrations holds the numbered, forward-only SQL migrations, embedded in the binary
// and applied at boot. See docs/DATABASE.md section 7 for the policy.
package migrations

import "embed"

// FS contains every numbered migration file, named NNN_description.sql.
//
//go:embed *.sql
var FS embed.FS

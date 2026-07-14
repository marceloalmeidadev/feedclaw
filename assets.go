// Package feedclaw holds assets embedded at the repository root so that other
// packages (store migrations, api UI) can consume them via embed.FS without
// duplicating files. Keeping the embed at the module root lets migrations/ and
// the built UI stay in their documented locations.
package feedclaw

import "embed"

// Migrations contains the SQL migration files applied in lexical order.
//
//go:embed migrations/*.sql
var Migrations embed.FS

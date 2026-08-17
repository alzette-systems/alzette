// Package migrations exposes the versioned PostgreSQL schema to the migration runner.
package migrations

import "embed"

// Files contains all migration scripts shipped with the application image.
//
//go:embed *.sql
var Files embed.FS

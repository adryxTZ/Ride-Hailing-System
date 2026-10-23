// Package db embeds the SQL migration files so the binary carries its own schema.
package db

import "embed"

// Migrations contains migrations/*.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS

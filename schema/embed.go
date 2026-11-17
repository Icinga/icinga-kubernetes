package schema

import "embed"

// MySQLSchemaDir is the embedded directory containing the MySQL schema and
// upgrade files. It is used by ApplySchema() to populate an empty database
// with the initial schema.
//
//go:embed mysql
var MySQLSchemaDir embed.FS

// TODO: Add PostgreSQL schema and upgrade files when they are available.

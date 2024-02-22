package mysql

import _ "embed"

// Schema is a copy of schema.sql. It resides here
// and not in ../../cmd/icinga-kubernetes/main.go due to go:embed restrictions.
//
//go:embed schema.sql
var Schema string

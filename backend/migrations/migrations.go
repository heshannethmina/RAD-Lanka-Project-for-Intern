// Package migrations carries the schema as embedded SQL files.
//
// The files live here rather than under internal/store because go:embed
// cannot reach outside its own package directory, and the SQL is worth
// keeping at the top level of the module where it is easy to find and to
// read without a Go toolchain.
package migrations

import "embed"

// FS holds every migration, named NNNN_description.sql. They are applied in
// lexical order, which is why the number is zero-padded: 0010 must sort after
// 0009, not between 0001 and 0002.
//
//go:embed *.sql
var FS embed.FS

// Package migrations embeds the database migrations, applied in order by
// version (the number prefix). Applied migrations must not be edited; a schema
// change is a new file.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

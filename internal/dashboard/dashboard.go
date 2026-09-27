// Package dashboard embeds the static web dashboard served at "/".
package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// FS returns the dashboard files, rooted at the static directory.
func FS() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err) // the directory is embedded at compile time, so this cannot fail
	}
	return sub
}

package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

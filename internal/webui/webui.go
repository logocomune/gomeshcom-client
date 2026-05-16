package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var raw embed.FS

// FS returns the SPA build rooted at the dist directory.
func FS() fs.FS {
	sub, err := fs.Sub(raw, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

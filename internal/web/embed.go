package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var FS embed.FS

// StaticFS is the sub-filesystem for serving static assets at /-/static/.
var StaticFS = mustSub(FS, "static")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("web: fs.Sub(" + dir + "): " + err.Error())
	}
	return sub
}

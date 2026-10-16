// Package webui embeds the built dashboard. `npm run build` in web/ writes it to dist/app,
// which is gitignored; dist/placeholder is committed so the server builds before the frontend.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Built reports whether the real dashboard was embedded.
func Built() bool {
	_, err := fs.Stat(dist, "dist/app/index.html")
	return err == nil
}

// FS returns the dashboard's files rooted at index.html: the real build when present, else the
// placeholder page.
func FS() fs.FS {
	dir := "dist/placeholder"
	if Built() {
		dir = "dist/app"
	}
	sub, err := fs.Sub(dist, dir)
	if err != nil {
		panic(err) // dir is a constant inside the embedded tree
	}
	return sub
}

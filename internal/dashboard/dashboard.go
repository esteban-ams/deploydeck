package dashboard

import (
	"embed"
	"net/http"
)

//go:embed index.html
var files embed.FS

// Handler returns an http.Handler that serves the embedded dashboard UI.
func Handler() http.Handler {
	return http.FileServer(http.FS(files))
}

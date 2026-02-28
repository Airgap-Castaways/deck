package server

import (
	"net/http"
	"path/filepath"
	"strings"
)

func NewHandler(root string) http.Handler {
	mux := http.NewServeMux()

	filesDir := filepath.Join(root, "files")
	packagesDir := filepath.Join(root, "packages")

	mux.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(filesDir))))
	mux.Handle("/packages/", http.StripPrefix("/packages/", http.FileServer(http.Dir(packagesDir))))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/files/") || strings.HasPrefix(r.URL.Path, "/packages/") {
			mux.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

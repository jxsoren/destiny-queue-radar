package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
)

// handleStatic serves the built dashboard (spec §2, §8.3). Real files are served as they are;
// Vite's hashed files under assets/ never change, so they are cached for a year. Every other GET
// outside /api and /auth gets index.html (history fallback), which browsers must revalidate.
// A missing file under assets/ is a 404, so a stale tab never parses HTML as JavaScript.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	// Reserved paths get a JSON 404, never index.html. This also covers a real API route used with
	// the wrong method (GET /api/refresh): "GET /" matches it, so the mux never answers 405 there.
	// The dashboard only calls routes with the right method, so the 404 is accepted.
	if isReserved(name) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if name != "" && name != "index.html" {
		if st, err := fs.Stat(s.d.Static, name); err == nil && !st.IsDir() {
			cache := cacheRevalidate
			if strings.HasPrefix(name, "assets/") {
				cache = cacheImmutable
			}
			w.Header().Set("Cache-Control", cache)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeFileFS(w, r, s.d.Static, name)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
	}
	s.serveIndex(w, r)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	b, err := fs.ReadFile(s.d.Static, "index.html")
	if err != nil {
		s.internal(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", cacheRevalidate)
	h.Set("X-Content-Type-Options", "nosniff")
	w.Write(b)
}

// isReserved reports whether a cleaned path belongs to the API or sign-in, which never fall
// back to the SPA.
func isReserved(name string) bool {
	for _, p := range []string{"api", "auth"} {
		if name == p || strings.HasPrefix(name, p+"/") {
			return true
		}
	}
	return false
}

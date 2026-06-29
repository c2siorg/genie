package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed ui/*
var uiFS embed.FS

// UI serves the embedded single-page UI (ui/index.html + ui/styles.css +
// ui/app.js). Mount it at /ui/* and Genie ships the front-end inside the
// binary — no Node, no build step, no separate static-file hosting.
//
// The UI dialogs with the same JSON API the curl examples in the README
// use; SSE streaming is read manually with fetch + ReadableStream so the
// browser doesn't need EventSource (which forbids Authorization headers).
type UI struct {
	root fs.FS
}

// NewUI builds the handler. Returns an error only if the embedded FS is
// missing — that's a build-time bug, not a runtime one.
func NewUI() (*UI, error) {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	return &UI{root: sub}, nil
}

// ServeHTTP serves the embedded files. The router mounts this under /ui/.
// Requests to /ui/ (no file) get index.html.
func (h *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// chi strips the mount prefix, so r.URL.Path is relative to /ui/.
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if rel == "" || rel == "/" {
		rel = "index.html"
	}
	// Block directory traversal.
	if strings.Contains(rel, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data, err := fs.ReadFile(h.root, rel)
	if err != nil {
		// SPA fallback for the React app subtree (served at /ui/app/). A
		// client-side route like /ui/app/commerce has no corresponding file;
		// serve app/index.html so the router can take over. Requests that look
		// like assets (have a file extension) still 404, so a genuinely missing
		// bundle file is not masked as a 200.
		if isAppRoute(rel) {
			if idx, e := fs.ReadFile(h.root, "app/index.html"); e == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-cache, max-age=60")
				_, _ = w.Write(idx)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentTypeForPath(rel))
	// Short cache so an `index.html` edit shows up after one refresh, but
	// hashed assets can be cached aggressively if you ever add them.
	w.Header().Set("Cache-Control", "no-cache, max-age=60")
	_, _ = w.Write(data)
}

// IndexHTML returns the entry point — used by the root redirect handler so
// a bare GET / lands on the app instead of a 404.
func (h *UI) IndexHTML(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui/", http.StatusFound)
}

// isAppRoute reports whether rel is a client-side route of the React app
// (the /ui/app/ subtree) that should fall back to app/index.html. Extension-
// bearing paths (assets) are excluded so missing files surface as 404s.
func isAppRoute(rel string) bool {
	if rel != "app" && !strings.HasPrefix(rel, "app/") {
		return false
	}
	return path.Ext(rel) == ""
}

// contentTypeForPath picks a Content-Type from the extension. Keep this
// tiny — we only ship a handful of file types.
func contentTypeForPath(p string) string {
	switch {
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(p, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(p, ".json"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

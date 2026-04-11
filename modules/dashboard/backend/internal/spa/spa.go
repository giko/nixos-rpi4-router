// Package spa holds the embedded React SPA build output and exposes an
// http.Handler that serves it with an SPA-style fallback to index.html.
// The dist/ subdirectory is populated from modules/dashboard/frontend/dist
// during the CI build, before `go build` is invoked. A committed placeholder
// index.html keeps the embed directive compiling in local dev.
package spa

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var fsys embed.FS

// Handler returns an http.Handler that serves embedded SPA assets. Requests
// for paths that do not correspond to a real file AND look like browser
// navigation (Accept header includes text/html) are served the embedded
// index.html so the React client-side router can resolve them. Missing
// asset requests (a stale hashed .js/.css path, a mistyped image URL)
// return 404 instead — this prevents browsers from receiving the HTML
// shell with 200 OK when they asked for JavaScript, which would surface
// as an opaque "Unexpected token '<'" parse error.
func Handler() http.Handler {
	sub, err := fs.Sub(fsys, "dist")
	if err != nil {
		// Impossible in practice — the embed directive guarantees the path.
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			if isNavigationRequest(r) {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// isNavigationRequest reports whether r looks like a browser navigation
// (top-level document fetch). Heuristic: the Accept header explicitly lists
// text/html. Browsers send this on address-bar navigations and <a> clicks;
// <script>/<link>/fetch() calls typically do not.
func isNavigationRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

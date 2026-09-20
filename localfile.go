package main

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// allowedPreviewExts limits preview serving to video files.
var allowedPreviewExts = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/x-m4v",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".webm": "video/webm",
}

// localFileHandler serves local video files to the webview for preview:
//
//	GET /localfile?path=<absolute-file-path>
//
// http.ServeContent provides Range support so <video> can seek.
// Only regular files with a video extension are served; everything else
// falls through to the embedded frontend assets (404).
func localFileHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/localfile" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimSpace(r.URL.Query().Get("path"))
		if p == "" || !filepath.IsAbs(p) {
			http.Error(w, "missing absolute path", http.StatusBadRequest)
			return
		}
		ctype, ok := allowedPreviewExts[strings.ToLower(filepath.Ext(p))]
		if !ok {
			http.Error(w, "unsupported file type", http.StatusUnsupportedMediaType)
			return
		}
		f, err := os.Open(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		if ctype == "" {
			ctype = mime.TypeByExtension(strings.ToLower(filepath.Ext(p)))
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
	})
}

// previewURL builds the webview URL for a local file preview.
func previewURL(path string) string {
	return "/localfile?path=" + url.QueryEscape(path)
}

// startPreviewServer serves /localfile on 127.0.0.1 and returns its base URL.
//
// Why a separate server: in `wails dev` the page is served by Vite, so a
// relative /localfile URL hits Vite (404/HTML) and never reaches the
// AssetServer handler — every preview fails. An absolute loopback URL works
// identically in dev and production builds.
func startPreviewServer() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("preview server listen: %w", err)
	}
	go func() {
		_ = http.Serve(ln, localFileHandler())
	}()
	return "http://" + ln.Addr().String(), nil
}

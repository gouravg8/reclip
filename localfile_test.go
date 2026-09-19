package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFileFullAndRange(t *testing.T) {
	video := makeTestVideo(t)
	h := localFileHandler()

	// Full request.
	req := httptest.NewRequest(http.MethodGet, "/localfile?path="+url.QueryEscape(video), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("full: status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("content-type = %q", ct)
	}
	fullBody := rec.Body.Bytes()
	if len(fullBody) == 0 {
		t.Fatal("empty body")
	}

	// Range request (video seeking).
	req = httptest.NewRequest(http.MethodGet, "/localfile?path="+url.QueryEscape(video), nil)
	req.Header.Set("Range", "bytes=0-99")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res = rec.Result()
	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: status %d", res.StatusCode)
	}
	if got := len(rec.Body.Bytes()); got != 100 {
		t.Fatalf("range body = %d bytes, want 100", got)
	}
}

func TestLocalFileRejects(t *testing.T) {
	h := localFileHandler()
	// A directory carrying a video extension exercises the IsRegular check.
	fakeDir := t.TempDir() + ".mp4"
	if err := os.Mkdir(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		target string
		want   int
	}{
		{"missing path param", "/localfile", http.StatusBadRequest},
		{"relative path", "/localfile?path=relative.mp4", http.StatusBadRequest},
		{"wrong route", "/other?path=/x.mp4", http.StatusNotFound},
		{"bad extension", "/localfile?path=" + url.QueryEscape(filepath.Join(string(filepath.Separator), "x.exe")), http.StatusUnsupportedMediaType},
		{"missing file", "/localfile?path=" + url.QueryEscape(filepath.Join(os.TempDir(), "reclip-nope.mp4")), http.StatusNotFound},
		{"directory", "/localfile?path=" + url.QueryEscape(fakeDir), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

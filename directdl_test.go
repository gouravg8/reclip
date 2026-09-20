package main

import (
	"testing"
)

func TestExtractDirectURL(t *testing.T) {
	og := `<html><head><meta property="og:video:secure_url" content="https://scontent.cdninstagram.com/v/abc.mp4?x=1&amp;y=2" /></head></html>`
	if got := extractDirectURL(og); got != "https://scontent.cdninstagram.com/v/abc.mp4?x=1&y=2" {
		t.Fatalf("og meta: %q", got)
	}

	embedded := `{"video_url":"https:\/\/scontent.fbcdn.net\/v\/def.mp4?x=1\u0026y=2","other":1}`
	if got := extractDirectURL(embedded); got != "https://scontent.fbcdn.net/v/def.mp4?x=1&y=2" {
		t.Fatalf("embedded json: %q", got)
	}

	for _, html := range []string{
		"",
		"<html>login wall, nothing here</html>",
		`<meta property="og:video" content="https://evil.example/v.mp4" />`,
		`"video_url":"not a url"`,
	} {
		if got := extractDirectURL(html); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	}
}

func TestURLSlug(t *testing.T) {
	cases := map[string]string{
		"https://www.instagram.com/reel/DdgKq8Op3mj/":      "DdgKq8Op3mj",
		"https://www.instagram.com/reel/DdgKq8Op3mj?utm=x": "DdgKq8Op3mj",
		"not a url": "reel",
		"":          "reel",
	}
	for in, want := range cases {
		if got := urlSlug(in); got != want {
			t.Fatalf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

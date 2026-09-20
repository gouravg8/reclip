package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

var (
	ogVideoRe  = regexp.MustCompile(`<meta[^>]+property=["']og:video(?::secure_url)?["'][^>]+content=["']([^"']+)["']`)
	videoURLRe = regexp.MustCompile(`"video_url"\s*:\s*"((?:[^"\\]|\\.)+)"`)
)

// directClient is shared, with short timeouts — this is a best-effort
// fast path, never the thing that hangs a download.
var directClient = &http.Client{Timeout: 30 * time.Second}

// extractDirectURL pulls a playable mp4 URL out of reel page HTML:
// og:video meta first, then embedded video_url JSON. Returns "" if none.
func extractDirectURL(html string) string {
	if m := ogVideoRe.FindStringSubmatch(html); m != nil {
		if u := cleanEmbedURL(m[1]); u != "" {
			return u
		}
	}
	if m := videoURLRe.FindStringSubmatch(html); m != nil {
		if u := cleanEmbedURL(m[1]); u != "" {
			return u
		}
	}
	return ""
}

// cleanEmbedURL unescapes JSON-embedded URLs and sanity-checks the result.
func cleanEmbedURL(raw string) string {
	u := strings.ReplaceAll(raw, `\/`, "/")
	u = strings.ReplaceAll(u, `\u0026`, "&")
	u = strings.ReplaceAll(u, "&amp;", "&")
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsed.Host), "fbcdn") &&
		!strings.Contains(strings.ToLower(parsed.Host), "cdninstagram") {
		return ""
	}
	return u
}

// urlSlug derives a filename stem from the reel URL path.
func urlSlug(pageURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "reel"
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return "reel"
}

// tryDirectDownload attempts an anonymous CDN fetch: page HTML first,
// then the mp4 bytes. Any failure returns an error and the caller falls
// back to yt-dlp — this path must never be fatal by itself.
func tryDirectDownload(pageURL, outputDir string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := directClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("page fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	videoURL := extractDirectURL(string(body))
	if videoURL == "" {
		return "", fmt.Errorf("no public video url (login-walled or private?)")
	}

	vreq, err := http.NewRequest(http.MethodGet, videoURL, nil)
	if err != nil {
		return "", err
	}
	vreq.Header.Set("User-Agent", browserUA)
	vreq.Header.Set("Referer", "https://www.instagram.com/")
	vresp, err := directClient.Do(vreq)
	if err != nil {
		return "", fmt.Errorf("video fetch: %w", err)
	}
	defer vresp.Body.Close()
	if vresp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("video HTTP %s", vresp.Status)
	}
	if ct := vresp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "video") &&
		!strings.Contains(ct, "octet-stream") {
		return "", fmt.Errorf("unexpected content type %q", ct)
	}
	out := resolveOutput(filepath.Join(outputDir, urlSlug(pageURL)+"_direct.mp4"))
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, vresp.Body)
	f.Close()
	if err != nil {
		os.Remove(out)
		return "", fmt.Errorf("video download: %w", err)
	}
	if n < 100*1024 {
		os.Remove(out)
		return "", fmt.Errorf("response too small (%d bytes), likely a block page", n)
	}
	return out, nil
}

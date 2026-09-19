package main

import (
	"strings"
	"testing"
)

func TestYtDlpAssetURL(t *testing.T) {
	url, err := ytDlpDownloadURL()
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if !strings.HasPrefix(url, "https://github.com/yt-dlp/yt-dlp/releases/latest/download/") {
		t.Fatalf("unexpected url %q", url)
	}
	asset, err := ytDlpAsset()
	if err != nil {
		t.Fatalf("asset: %v", err)
	}
	if !strings.HasSuffix(url, "/"+asset) {
		t.Fatalf("url %q missing asset %q", url, asset)
	}
	if asset != "yt-dlp.exe" && asset != "yt-dlp" && asset != "yt-dlp_macos" {
		t.Fatalf("unexpected asset %q", asset)
	}
}

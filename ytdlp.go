package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ytDlpAsset maps the OS to the upstream release asset name.
// (Documented names from the yt-dlp README: plain `yt-dlp` for Linux.)
func ytDlpAsset() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "yt-dlp.exe", nil
	case "linux":
		return "yt-dlp", nil
	case "darwin":
		return "yt-dlp_macos", nil
	default:
		return "", fmt.Errorf("yt-dlp auto-download not supported on %s", runtime.GOOS)
	}
}

// Latest release is used (not a pinned version) so the URL never rots.
func ytDlpDownloadURL() (string, error) {
	asset, err := ytDlpAsset()
	if err != nil {
		return "", err
	}
	return "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + asset, nil
}

// setupProgress is emitted as "reclip:setup-progress" during downloads.
type setupProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// DownloadYtDlp fetches the pinned yt-dlp build into the app config dir,
// makes it executable, remembers it as the ytdlp engine path and returns it.
// One click in Setup — no manual download needed on Windows or Ubuntu.
func (a *App) DownloadYtDlp() (string, error) {
	url, err := ytDlpDownloadURL()
	if err != nil {
		return "", err
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	final := filepath.Join(binDir, name)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download yt-dlp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download yt-dlp: HTTP %s", resp.Status)
	}
	tmp, err := os.CreateTemp(binDir, "yt-dlp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	buf := make([]byte, 128*1024)
	var done int64
	total := resp.ContentLength
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				return "", werr
			}
			done += int64(n)
			a.emit("reclip:setup-progress", setupProgress{Done: done, Total: total})
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			tmp.Close()
			return "", fmt.Errorf("download yt-dlp: %w", rerr)
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpName, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmpName, final); err != nil {
		return "", err
	}
	if err := a.SetEnginePath("ytdlp", final); err != nil {
		return "", err
	}
	return final, nil
}

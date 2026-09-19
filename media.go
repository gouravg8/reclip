package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// VideoInfo describes a probed media file.
type VideoInfo struct {
	Path      string  `json:"path"`
	Duration  float64 `json:"duration"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FPS       float64 `json:"fps"`
	SizeBytes int64   `json:"sizeBytes"`
	HasAudio  bool    `json:"hasAudio"`
}

// Deps reports the availability of external binaries.
type Deps struct {
	Ffmpeg  string `json:"ffmpeg"`
	Ffprobe string `json:"ffprobe"`
	YtDlp   string `json:"ytDlp"`
}

// resolveBin finds an external binary. Order:
// 1. env override (e.g. RECLIP_FFMPEG)
// 2. next to the app executable (bundled sidecar for packaged builds)
// 3. system PATH.
func resolveBin(envKey string, names ...string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
		if p, err := exec.LookPath(v); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%s not found at %q", names[0], v)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, n := range names {
			candidate := filepath.Join(dir, n)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH (set %s to override)", names[0], envKey)
}

func ffmpegBin() (string, error) {
	return resolveBin("RECLIP_FFMPEG", "ffmpeg", "ffmpeg.exe")
}

func ffprobeBin() (string, error) {
	return resolveBin("RECLIP_FFPROBE", "ffprobe", "ffprobe.exe")
}

func ytDlpBin() (string, error) {
	return resolveBin("RECLIP_YTDLP", "yt-dlp", "yt-dlp.exe")
}

// emit is a nil-ctx-safe wrapper around runtime.EventsEmit
// (a.ctx is nil in unit tests).
func (a *App) emit(event string, data ...interface{}) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, event, data...)
}

// CheckDeps returns version strings (or missing markers) for ffmpeg,
// ffprobe and yt-dlp so the UI can warn early.
func (a *App) CheckDeps() Deps {
	return Deps{
		Ffmpeg:  binVersion("RECLIP_FFMPEG", []string{"ffmpeg"}, "-version"),
		Ffprobe: binVersion("RECLIP_FFPROBE", []string{"ffprobe"}, "-version"),
		YtDlp:   binVersion("RECLIP_YTDLP", []string{"yt-dlp"}, "--version"),
	}
}

func binVersion(envKey string, names []string, versionArg string) string {
	p, err := resolveBin(envKey, names...)
	if err != nil {
		return "missing: " + err.Error()
	}
	out, err := exec.Command(p, versionArg).Output()
	if err != nil {
		return "missing: " + err.Error()
	}
	first, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(first)
}

// ProbeVideo returns duration, dimensions, fps and size for a media file.
func (a *App) ProbeVideo(path string) (VideoInfo, error) {
	info := VideoInfo{Path: path}
	st, err := os.Stat(path)
	if err != nil {
		return info, fmt.Errorf("stat %q: %w", path, err)
	}
	info.SizeBytes = st.Size()

	bin, err := ffprobeBin()
	if err != nil {
		return info, err
	}
	out, err := exec.Command(bin,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	).Output()
	if err != nil {
		return info, fmt.Errorf("ffprobe failed: %w", err)
	}
	var probed struct {
		Streams []struct {
			CodecType    string `json:"codec_type"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
			Duration     string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probed); err != nil {
		return info, fmt.Errorf("parse ffprobe output: %w", err)
	}
	for _, s := range probed.Streams {
		if s.CodecType == "audio" {
			info.HasAudio = true
			continue
		}
		if s.CodecType == "video" && info.Width == 0 {
			info.Width = s.Width
			info.Height = s.Height
			info.FPS = parseFPS(s.AvgFrameRate)
			if d, err := strconv.ParseFloat(s.Duration, 64); err == nil && d > 0 {
				info.Duration = d
			}
		}
	}
	if info.Duration == 0 {
		if d, err := strconv.ParseFloat(probed.Format.Duration, 64); err == nil {
			info.Duration = d
		}
	}
	if info.Width == 0 || info.Height == 0 {
		return info, fmt.Errorf("no video stream found in %q", path)
	}
	return info, nil
}

func parseFPS(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

// DownloadReel downloads a single Instagram reel (or any yt-dlp-supported
// URL) into outputDir and returns the final file path. Progress lines are
// emitted as "reclip:download-progress" events for the UI.
func (a *App) DownloadReel(url, outputDir string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("empty URL")
	}
	if strings.TrimSpace(outputDir) == "" {
		return "", fmt.Errorf("empty output directory")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	bin, err := ytDlpBin()
	if err != nil {
		return "", err
	}
	template := filepath.Join(outputDir, "%(title).50s [%(id)s].%(ext)s")
	cmd := exec.Command(bin,
		"--no-playlist",
		"--newline",
		"-f", "bv*+ba/b",
		"--merge-output-format", "mp4",
		"--print", "after_move:filepath",
		"-o", template,
		url,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start yt-dlp: %w", err)
	}
	// Stream progress: stdout carries the final filepath line(s),
	// stderr carries [download] progress lines.
	var printed []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			a.emit("reclip:download-progress", sc.Text())
		}
	}()
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			printed = append(printed, line)
		}
	}
	<-done
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}
	// Last printed existing file wins (after_move:filepath).
	for i := len(printed) - 1; i >= 0; i-- {
		if st, err := os.Stat(printed[i]); err == nil && !st.IsDir() {
			return printed[i], nil
		}
	}
	return "", fmt.Errorf("yt-dlp finished but no output file found")
}

// SelectVideoFiles opens a multi-file dialog for reel/credit videos.
func (a *App) SelectVideoFiles() ([]string, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("dialogs unavailable (no app context)")
	}
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select videos",
		Filters: []runtime.FileFilter{
			{DisplayName: "Videos (*.mp4, *.mov, *.mkv, *.webm)", Pattern: "*.mp4;*.mov;*.mkv;*.webm"},
		},
	})
}

// SelectCreditFile opens a single-file dialog for the credit video.
func (a *App) SelectCreditFile() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("dialogs unavailable (no app context)")
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select credit video",
		Filters: []runtime.FileFilter{
			{DisplayName: "Videos (*.mp4, *.mov, *.mkv, *.webm)", Pattern: "*.mp4;*.mov;*.mkv;*.webm"},
		},
	})
}

// SelectOutputDir opens a directory dialog for downloads/exports.
func (a *App) SelectOutputDir() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("dialogs unavailable (no app context)")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select output folder",
	})
}

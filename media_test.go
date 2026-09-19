package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeTestVideo generates a tiny 2s 640x480 mp4 via the system ffmpeg.
func makeTestVideo(t *testing.T) string {
	t.Helper()
	bin, err := ffmpegBin()
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "probe_test.mp4")
	cmd := exec.Command(bin,
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=640x480:rate=30",
		"-pix_fmt", "yuv420p",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg failed: %v (%s)", err, out)
	}
	return path
}

func TestProbeVideo(t *testing.T) {
	path := makeTestVideo(t)
	app := NewApp()
	info, err := app.ProbeVideo(path)
	if err != nil {
		t.Fatalf("ProbeVideo: %v", err)
	}
	if info.Width != 640 || info.Height != 480 {
		t.Fatalf("unexpected dims: %+v", info)
	}
	if info.Duration < 1.5 || info.Duration > 2.5 {
		t.Fatalf("unexpected duration: %+v", info)
	}
	if info.SizeBytes == 0 {
		t.Fatalf("expected non-zero size: %+v", info)
	}
}

func TestProbeVideoMissing(t *testing.T) {
	app := NewApp()
	if _, err := app.ProbeVideo(filepath.Join(t.TempDir(), "nope.mp4")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCreditPresetRoundtrip(t *testing.T) {
	t.Setenv("RECLIP_CONFIG_DIR", t.TempDir())
	app := NewApp()
	if got := app.GetCreditPreset(); got != "" {
		t.Fatalf("expected empty preset, got %q", got)
	}
	fake := filepath.Join(t.TempDir(), "credit.mp4")
	if err := os.WriteFile(fake, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCreditPreset(fake); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := app.GetCreditPreset(); got != fake {
		t.Fatalf("Get = %q, want %q", got, fake)
	}
	if err := app.ClearCreditPreset(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if got := app.GetCreditPreset(); got != "" {
		t.Fatalf("expected empty after clear, got %q", got)
	}
}

func TestResolveBinEnvOverride(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "my-ffmpeg")
	if err := os.WriteFile(fake, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RECLIP_FFMPEG", fake)
	got, err := ffmpegBin()
	if err != nil || got != fake {
		t.Fatalf("env override = %q, %v", got, err)
	}
	t.Setenv("RECLIP_FFMPEG", filepath.Join(t.TempDir(), "nope"))
	if _, err := ffmpegBin(); err == nil {
		t.Fatal("expected error for missing override path")
	}
}

func TestCheckDepsPresent(t *testing.T) {
	app := NewApp()
	deps := app.CheckDeps()
	for name, v := range map[string]string{
		"ffmpeg": deps.Ffmpeg, "ffprobe": deps.Ffprobe, "yt-dlp": deps.YtDlp,
	} {
		if v == "" || len(v) < 3 || v[:7] == "missing" {
			t.Fatalf("%s unresolved: %q", name, v)
		}
	}
}

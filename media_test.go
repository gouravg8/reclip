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

func TestParseFfmpegInfo(t *testing.T) {
	sample := `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from '/tmp/probe-sample.mp4':
  Duration: 00:00:01.00, start: 0.000000, bitrate: 1885 kb/s
  Stream #0:0[0x1](und): Video: h264 (High) (avc1 / 0x31637661), yuv420p(progressive), 640x960 [SAR 1:1 DAR 2:3], 1794 kb/s, 30 fps, 30 tbr, 15360 tbn (default)
  Stream #0:1[0x2](und): Audio: aac (LC) (mp4a / 0x6134706D), 44100 Hz, mono, fltp, 70 kb/s (default)
At least one output file must be specified
`
	info, err := parseFfmpegInfo(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.duration != 1.0 {
		t.Fatalf("duration = %v", info.duration)
	}
	if info.width != 640 || info.height != 960 {
		t.Fatalf("dims = %dx%d", info.width, info.height)
	}
	if info.fps != 30 {
		t.Fatalf("fps = %v", info.fps)
	}
	if !info.hasAudio {
		t.Fatal("expected audio")
	}

	noAudio := `  Duration: 00:01:30.50, start: 0.000000, bitrate: 500 kb/s
  Stream #0:0: Video: h264, yuv420p, 1080x1920, 25 fps
`
	info, err = parseFfmpegInfo(noAudio)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.duration != 90.5 || info.fps != 25 || info.hasAudio {
		t.Fatalf("unexpected: %+v", info)
	}

	if _, err := parseFfmpegInfo("garbage\nno streams here\n"); err == nil {
		t.Fatal("expected error for stream-less output")
	}
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

func TestEnginePathRoundtrip(t *testing.T) {
	t.Setenv("RECLIP_CONFIG_DIR", t.TempDir())
	app := NewApp()
	fake := filepath.Join(t.TempDir(), "ffmpeg-test")
	if err := os.WriteFile(fake, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := app.SetEnginePath("ffmpeg", fake); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := app.GetEnginePaths()["ffmpeg"]; got != fake {
		t.Fatalf("Get = %q, want %q", got, fake)
	}
	// Config fallback resolves through resolveBin (no env override set here).
	t.Setenv("RECLIP_FFMPEG", "")
	if got, err := ffmpegBin(); err != nil || got != fake {
		t.Fatalf("resolveBin = %q, %v", got, err)
	}
	// Credit preset still works alongside engines in one file.
	credit := filepath.Join(t.TempDir(), "c.mp4")
	if err := os.WriteFile(credit, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCreditPreset(credit); err != nil {
		t.Fatalf("SaveCredit: %v", err)
	}
	if got := app.GetCreditPreset(); got != credit {
		t.Fatalf("credit lost: %q", got)
	}
	if got := app.GetEnginePaths()["ffmpeg"]; got != fake {
		t.Fatalf("engine lost after credit save: %q", got)
	}
	if err := app.SetEnginePath("nope", fake); err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestCheckDepsPresent(t *testing.T) {
	app := NewApp()
	deps := app.CheckDeps()
	for name, v := range map[string]string{
		"ffmpeg": deps.Ffmpeg, "yt-dlp": deps.YtDlp,
	} {
		if v == "" || len(v) < 3 || v[:7] == "missing" {
			t.Fatalf("%s unresolved: %q", name, v)
		}
	}
}

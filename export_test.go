package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestKeptSegments(t *testing.T) {
	cases := []struct {
		name  string
		dur   float64
		edit  ExportEdit
		want  [][2]float64
		total float64
	}{
		{
			"full", 10, ExportEdit{},
			[][2]float64{{0, 10}}, 10,
		},
		{
			"trim", 10, ExportEdit{TrimStart: 2, TrimEnd: 8},
			[][2]float64{{2, 8}}, 6,
		},
		{
			"trim+cut", 10, ExportEdit{TrimStart: 1, TrimEnd: 9, Cuts: []ExportCut{{3, 5}}},
			[][2]float64{{1, 3}, {5, 9}}, 6,
		},
		{
			"cut outside trim ignored", 10,
			ExportEdit{TrimStart: 4, TrimEnd: 6, Cuts: []ExportCut{{0, 2}}},
			[][2]float64{{4, 6}}, 2,
		},
		{
			"overlapping cuts merge", 10,
			ExportEdit{Cuts: []ExportCut{{2, 5}, {4, 7}}},
			[][2]float64{{0, 2}, {7, 10}}, 5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := keptSegments(tc.dur, tc.edit)
			if err != nil {
				t.Fatalf("keptSegments: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("segs = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("segs = %v, want %v", got, tc.want)
				}
			}
			if total := keptTotal(got); total != tc.total {
				t.Fatalf("total = %v, want %v", total, tc.total)
			}
		})
	}
}

func TestKeptSegmentsEmpty(t *testing.T) {
	if _, err := keptSegments(10, ExportEdit{TrimStart: 8, TrimEnd: 8}); err == nil {
		t.Fatal("expected error for empty trim")
	}
	if _, err := keptSegments(10, ExportEdit{Cuts: []ExportCut{{0, 10}}}); err == nil {
		t.Fatal("expected error when cuts remove everything")
	}
}

func TestBuildFilter(t *testing.T) {
	f := buildFilter(1080, 1920, ExportEdit{Zoom: 1})
	if !strings.Contains(f, "scale=1080:1920") {
		t.Fatalf("plain filter = %q", f)
	}
	f = buildFilter(720, 1280, ExportEdit{Zoom: 1.5, PanX: -20, PanY: 10})
	for _, want := range []string{"crop=", "scale=", "1080", "1920", "fps=30"} {
		if !strings.Contains(f, want) {
			t.Fatalf("zoom filter %q missing %q", f, want)
		}
	}
	// Odd dims must not crash or produce odd crop values.
	f = buildFilter(721, 1281, ExportEdit{Zoom: 2})
	if !strings.Contains(f, "fps=30") {
		t.Fatalf("odd-dim filter = %q", f)
	}
}

// synthVideo makes a small test clip; withAudio adds a sine track.
func synthVideo(t *testing.T, name string, dur, w, h int, withAudio bool) string {
	t.Helper()
	bin, err := ffmpegBin()
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	args := []string{"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=duration=" + strconv.Itoa(dur) + ":size=" + strconv.Itoa(w) + "x" + strconv.Itoa(h) + ":rate=30",
	}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(dur))
	}
	args = append(args, "-pix_fmt", "yuv420p", "-c:v", "libx264",
		"-c:a", "aac", "-shortest", path)
	if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
		t.Fatalf("synth %s: %v (%s)", name, err, out)
	}
	return path
}

func TestExportPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skip full export in short mode")
	}
	src := synthVideo(t, "src.mp4", 5, 640, 960, true)
	credit := synthVideo(t, "credit.mp4", 2, 1080, 1920, false)
	app := NewApp()
	out := filepath.Join(t.TempDir(), "out.mp4")
	res, err := app.ExportOne(`{"id":"j1","input":` + jsonStr(src) +
		`,"output":` + jsonStr(out) +
		`,"edit":{"trimStart":1,"trimEnd":4,"cuts":[{"start":2,"end":3}],"zoom":1.5,"panX":-10,"panY":5},` +
		`"creditPath":` + jsonStr(credit) + `}`)
	if err != nil {
		t.Fatalf("ExportOne: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("export error: %s", res.Error)
	}
	info, err := app.ProbeVideo(res.Output)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if info.Width != 1080 || info.Height != 1920 {
		t.Fatalf("dims = %dx%d, want 1080x1920", info.Width, info.Height)
	}
	// (4-1) - (3-2) + 2 credit = 4s.
	if info.Duration < 3.5 || info.Duration > 4.5 {
		t.Fatalf("duration = %v, want ~4s", info.Duration)
	}
	if !info.HasAudio {
		t.Fatal("expected audio track in output")
	}
}

func jsonStr(s string) string {
	return strconv.Quote(s)
}

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ExportCut is a [start, end) section (seconds, source timeline) removed.
type ExportCut struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ExportEdit mirrors the frontend EditSpec. Passed as JSON inside ExportJob
// to keep the Wails binding surface simple.
type ExportEdit struct {
	TrimStart float64     `json:"trimStart"`
	TrimEnd   float64     `json:"trimEnd"`
	Cuts      []ExportCut `json:"cuts"`
	Zoom      float64     `json:"zoom"`
	PanX      float64     `json:"panX"`
	PanY      float64     `json:"panY"`
}

// ExportJob is one video export.
type ExportJob struct {
	ID         string     `json:"id"`
	Input      string     `json:"input"`
	Output     string     `json:"output"`
	Edit       ExportEdit `json:"edit"`
	CreditPath string     `json:"creditPath"`
}

// ExportResult is the per-job outcome returned to the UI.
type ExportResult struct {
	ID      string `json:"id"`
	Output  string `json:"output"`
	Skipped bool   `json:"skipped"`
	Error   string `json:"error"`
}

// ExportProgress is emitted as "reclip:export-progress".
type ExportProgress struct {
	ID    string  `json:"id"`
	Phase string  `json:"phase"`
	Done  float64 `json:"done"`
	Total float64 `json:"total"`
}

const (
	outW   = 1080
	outH   = 1920
	outFPS = 30
)

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func even(n int) int {
	return (n / 2) * 2
}

// keptSegments applies trim then subtracts cuts. Empty range => error.
func keptSegments(dur float64, e ExportEdit) ([][2]float64, error) {
	start := clampF(e.TrimStart, 0, dur)
	end := dur
	if e.TrimEnd > 0 {
		end = clampF(e.TrimEnd, 0, dur)
	}
	if end-start < 0.05 {
		return nil, fmt.Errorf("empty selection (trim %.1fs → %.1fs)", start, end)
	}
	cuts := append([]ExportCut(nil), e.Cuts...)
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Start < cuts[j].Start })
	segs := [][2]float64{{start, end}}
	for _, c := range cuts {
		cs := clampF(c.Start, start, end)
		ce := clampF(c.End, start, end)
		if ce-cs < 0.05 {
			continue
		}
		var next [][2]float64
		for _, s := range segs {
			if ce <= s[0] || cs >= s[1] {
				next = append(next, s)
				continue
			}
			if cs > s[0] {
				next = append(next, [2]float64{s[0], cs})
			}
			if ce < s[1] {
				next = append(next, [2]float64{ce, s[1]})
			}
		}
		segs = next
	}
	// Drop slivers.
	var kept [][2]float64
	for _, s := range segs {
		if s[1]-s[0] >= 0.05 {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("cuts removed everything")
	}
	return kept, nil
}

func keptTotal(segs [][2]float64) float64 {
	var t float64
	for _, s := range segs {
		t += s[1] - s[0]
	}
	return t
}

// buildFilter maps the editor preview (9:16 base, zoom around center, pan
// as translate %) to an ffmpeg filter chain:
//
//  1. crop to the largest centered 9:16 rect (bw×bh)
//  2. scale up by zoom
//  3. crop back to bw×bh, offset by pan (-50 = min edge, +50 = max edge)
//  4. scale to 1080×1920, square pixels, 30fps
func buildFilter(w, h int, e ExportEdit) string {
	bw, bh, bx, by := w, h, 0, 0
	if w*16 > h*9 {
		bw = even(h * 9 / 16)
		bx = even((w - bw) / 2)
	} else if h*9 > w*16 {
		bh = even(w * 16 / 9)
		by = even((h - bh) / 2)
	}
	z := clampF(e.Zoom, 1, 3)
	if z < 1.01 && bx == 0 && by == 0 && bw == w && bh == h {
		return fmt.Sprintf("scale=%d:%d:flags=lanczos,setsar=1,fps=%d", outW, outH, outFPS)
	}
	sw := even(int(float64(bw)*z + 0.5))
	sh := even(int(float64(bh)*z + 0.5))
	maxX := (sw - bw) / 2
	maxY := (sh - bh) / 2
	x := even(int(float64(maxX)*(1+clampF(e.PanX, -50, 50)/50) + 0.5))
	y := even(int(float64(maxY)*(1+clampF(e.PanY, -50, 50)/50) + 0.5))
	x = min(max(x, 0), sw-bw)
	y = min(max(y, 0), sh-bh)
	return fmt.Sprintf(
		"crop=%d:%d:%d:%d,scale=%d:%d:flags=lanczos,crop=%d:%d:%d:%d,scale=%d:%d:flags=lanczos,setsar=1,fps=%d",
		bw, bh, bx, by, sw, sh, bw, bh, x, y, outW, outH, outFPS,
	)
}

// hasAudioStream reports whether path contains an audio stream.
func hasAudioStream(path string) bool {
	bin, err := ffmpegBin()
	if err != nil {
		return false
	}
	out, _ := exec.Command(bin, "-hide_banner", "-i", path).CombinedOutput()
	parsed, err := parseFfmpegInfo(string(out))
	if err != nil {
		return false
	}
	return parsed.hasAudio
}

func ff3(f float64) string {
	return strconv.FormatFloat(f, 'f', 3, 64)
}

// runFFmpeg runs ffmpeg with -progress parsing, emitting ExportProgress.
func (a *App) runFFmpeg(jobID, phase string, doneBase, total float64, args ...string) error {
	bin, err := ffmpegBin()
	if err != nil {
		return err
	}
	full := append([]string{"-hide_banner", "-nostats", "-loglevel", "error",
		"-progress", "pipe:1", "-nostdin"}, args...)
	cmd := exec.Command(bin, full...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		var ms float64
		if v, ok := strings.CutPrefix(line, "out_time_ms="); ok {
			ms, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
		} else if v, ok := strings.CutPrefix(line, "out_time_us="); ok {
			if us, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				ms = us / 1000
			}
		} else {
			continue
		}
		a.emit("reclip:export-progress", ExportProgress{
			ID: jobID, Phase: phase, Done: doneBase + ms/1000, Total: total,
		})
	}
	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ffmpeg (%s) failed: %s", phase, msg)
	}
	return nil
}

// encodeSegment renders one kept [s,e) range to a uniform 1080×1920 file.
func (a *App) encodeSegment(jobID, phase string, doneBase, total float64, src string, seg [2]float64, filter string, dst string) error {
	args := []string{"-y", "-ss", ff3(seg[0]), "-t", ff3(seg[1] - seg[0]), "-i", src}
	if !hasAudioStream(src) {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo")
	}
	args = append(args, "-map", "0:v:0")
	if hasAudioStream(src) {
		args = append(args, "-map", "0:a:0?")
	} else {
		args = append(args, "-map", "1:a:0", "-shortest")
	}
	args = append(args,
		"-vf", filter,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "-ar", "44100", "-ac", "2",
		"-movflags", "faststart",
		dst,
	)
	return a.runFFmpeg(jobID, phase, doneBase, total, args...)
}

// normalizeCredit renders the credit clip to the same uniform format.
func (a *App) normalizeCredit(jobID string, doneBase, total float64, src, dst string) error {
	filter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d,setsar=1,fps=%d",
		outW, outH, outW, outH, outFPS,
	)
	args := []string{"-y", "-i", src}
	if !hasAudioStream(src) {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo")
	}
	args = append(args, "-map", "0:v:0")
	if hasAudioStream(src) {
		args = append(args, "-map", "0:a:0?")
	} else {
		args = append(args, "-map", "1:a:0", "-shortest")
	}
	args = append(args,
		"-vf", filter,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "-ar", "44100", "-ac", "2",
		"-movflags", "faststart",
		dst,
	)
	return a.runFFmpeg(jobID, "credit", doneBase, total, args...)
}

// resolveOutput avoids overwriting: name.mp4, name_2.mp4, …
func resolveOutput(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// exportOneJob renders a single job: segments → concat (+ credit) → output.
func (a *App) exportOneJob(job ExportJob) (string, error) {
	if strings.TrimSpace(job.Input) == "" {
		return "", fmt.Errorf("empty input")
	}
	if strings.TrimSpace(job.Output) == "" {
		return "", fmt.Errorf("empty output")
	}
	srcInfo, err := a.ProbeVideo(job.Input)
	if err != nil {
		return "", err
	}
	segs, err := keptSegments(srcInfo.Duration, job.Edit)
	if err != nil {
		return "", err
	}
	filter := buildFilter(srcInfo.Width, srcInfo.Height, job.Edit)
	total := keptTotal(segs)
	var creditDur float64
	if strings.TrimSpace(job.CreditPath) != "" {
		if _, err := os.Stat(job.CreditPath); err != nil {
			return "", fmt.Errorf("credit not found: %w", err)
		}
		if ci, err := a.ProbeVideo(job.CreditPath); err == nil {
			creditDur = ci.Duration
		}
		total += creditDur
	}

	tmp, err := os.MkdirTemp("", "reclip-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// 1. Encode kept segments uniformly.
	var parts []string
	var done float64
	for i, s := range segs {
		dst := filepath.Join(tmp, fmt.Sprintf("seg_%03d.mp4", i))
		if err := a.encodeSegment(job.ID, "video", done, total, job.Input, s, filter, dst); err != nil {
			return "", err
		}
		parts = append(parts, dst)
		done += s[1] - s[0]
		a.emit("reclip:export-progress", ExportProgress{ID: job.ID, Phase: "video", Done: done, Total: total})
	}
	// 2. Normalize credit.
	if strings.TrimSpace(job.CreditPath) != "" {
		dst := filepath.Join(tmp, "credit.mp4")
		if err := a.normalizeCredit(job.ID, done, total, job.CreditPath, dst); err != nil {
			return "", err
		}
		parts = append(parts, dst)
		done += creditDur
		a.emit("reclip:export-progress", ExportProgress{ID: job.ID, Phase: "credit", Done: done, Total: total})
	}
	// 3. Concat with stream copy (uniform inputs).
	listPath := filepath.Join(tmp, "list.txt")
	var list strings.Builder
	for _, p := range parts {
		list.WriteString("file '" + strings.ReplaceAll(p, "'", "'\\''") + "'\n")
	}
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return "", err
	}
	out := resolveOutput(job.Output)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	a.emit("reclip:export-progress", ExportProgress{ID: job.ID, Phase: "mux", Done: done, Total: total})
	if err := a.runFFmpeg(job.ID, "mux", done, total,
		"-y", "-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy", "-movflags", "faststart", out); err != nil {
		return "", err
	}
	a.emit("reclip:export-progress", ExportProgress{ID: job.ID, Phase: "done", Done: total, Total: total})
	return out, nil
}

// ExportOne renders a single job described by a JSON ExportJob.
func (a *App) ExportOne(jobJSON string) (ExportResult, error) {
	var job ExportJob
	if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
		return ExportResult{}, fmt.Errorf("bad job: %w", err)
	}
	out, err := a.exportOneJob(job)
	if err != nil {
		return ExportResult{ID: job.ID, Error: err.Error()}, nil
	}
	return ExportResult{ID: job.ID, Output: out}, nil
}

// ExportBatch renders jobs sequentially, emitting progress per job id.
// Jobs that fail don't stop the batch; their error lands in the result.
func (a *App) ExportBatch(jobsJSON string) ([]ExportResult, error) {
	var jobs []ExportJob
	if err := json.Unmarshal([]byte(jobsJSON), &jobs); err != nil {
		return nil, fmt.Errorf("bad jobs: %w", err)
	}
	results := make([]ExportResult, 0, len(jobs))
	for _, job := range jobs {
		out, err := a.exportOneJob(job)
		if err != nil {
			results = append(results, ExportResult{ID: job.ID, Error: err.Error()})
			continue
		}
		results = append(results, ExportResult{ID: job.ID, Output: out})
	}
	return results, nil
}

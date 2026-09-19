# Reclip

Batch Instagram reel downloader + editor + credit appender. Desktop, offline-first
(video processing is 100% local via bundled ffmpeg; only link downloads need internet).

Workflow: paste reel links → auto-download → trim / cut / zoom each video →
append your 10s credit clip → export one or all as 1080×1920 MP4.

## Stack

Wails v2 (Go) + React 19 + Vite 8 + Tailwind v4 + Ant Design 6 + Oxlint/Oxfmt.
External tools (resolved in this order): `RECLIP_FFMPEG` / `RECLIP_FFPROBE` /
`RECLIP_YTDLP` env vars → next to the app executable → system `PATH`.

## Dev

```bash
cd reclip
cd frontend && pnpm install && cd ..
wails dev            # hot reload; regenerates frontend/wailsjs bindings
```

Frontend-only checks: `cd frontend && pnpm run lint && pnpm run build`.
Backend checks: `gofmt -l . ; go vet ./... ; go test ./...`.

## Offline packaging (single folder / installer)

1. Fetch sidecars for your OS (downloads static builds, git-ignored):
   ```bash
   bash scripts/fetch-sidecars.sh windows   # linux | darwin | all
   ```
2. Build:
   ```bash
   wails build            # windows: .\reclip.exe + installer under build/bin
   ```
3. Copy the 3 sidecar files (`ffmpeg*`, `ffprobe*`, `yt-dlp*`) next to the
   built binary before zipping / running the NSIS installer step.

Linux note: `wails build` needs webkit2gtk dev libs
(`libwebkit2gtk-4.1-dev` on Ubuntu). Windows/macOS builds must run on
their own OS (or CI) — Wails does not cross-compile GUI apps.

## Env overrides

| Var | Purpose |
| --- | --- |
| `RECLIP_FFMPEG` | custom ffmpeg path |
| `RECLIP_FFPROBE` | custom ffprobe path |
| `RECLIP_YTDLP` | custom yt-dlp path |
| `RECLIP_CONFIG_DIR` | override config dir (credit preset; used by tests) |

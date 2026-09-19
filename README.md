# Reclip

Batch Instagram reel downloader + editor + credit appender. Desktop, offline-first
(video processing is 100% local via bundled ffmpeg; only link downloads need internet).

Workflow: paste reel links → auto-download → trim / cut / zoom each video →
append your 10s credit clip → export one or all as 1080×1920 MP4.

## Stack

Wails v2 (Go) + React 19 + Vite 8 + Tailwind v4 + Ant Design 6 + Oxlint/Oxfmt.
External tools (resolved in this order): `RECLIP_FFMPEG` / `RECLIP_YTDLP`
env vars → Setup-screen saved paths → next to the app executable → system `PATH`.

## Dev

```bash
cd reclip
cd frontend && pnpm install && cd ..
wails dev            # hot reload; regenerates frontend/wailsjs bindings
```

Frontend-only checks: `cd frontend && pnpm run lint && pnpm run build`.
Backend checks: `gofmt -l . ; go vet ./... ; go test ./...`.

## Sharing with others (installer)

Recipients need two free helper programs: **ffmpeg** and **yt-dlp**
(~100 MB together; no ffprobe needed — probing uses `ffmpeg -i`).
You have two ways to give them a working app — no manual setup either way:

**Option A — installer (recommended).** On a Windows PC run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\package-windows.ps1
```

It downloads the two tools into `build\bin`, then runs `wails build --nsis`.
The installer (`build\bin\reclip-*-installer.exe`) bundles everything, so the
person you send it to just clicks Next → done. First launch auto-detects the
bundled tools. (No internet on their side needed for install; yt-dlp can also
be fetched one-click from Setup later, which re-downloads the pinned build.)

**Option B — portable zip.** Copy these 3 files into one folder and zip it:

```text
reclip.exe  ffmpeg.exe  yt-dlp.exe
```

The app looks for the tools next to its own `.exe` first, so this also works
with zero setup.

**If a tool is still missing** (red dots in the top bar), the user clicks the
engines badge → **Setup** screen: it shows exactly what's missing, links to the
download pages, and a *Locate file…* button to point at the program once. The
choice is remembered on that PC. Advanced overrides:
`RECLIP_FFMPEG` / `RECLIP_YTDLP` env vars.

## Offline packaging (single folder / installer)

1. Fetch sidecars for your OS (downloads static builds, git-ignored):
   ```bash
   bash scripts/fetch-sidecars.sh windows   # linux | darwin | all
   ```
2. Build:
   ```bash
   wails build            # windows: .\reclip.exe + installer under build/bin
   ```
3. Copy the 2 sidecar files (`ffmpeg*`, `yt-dlp*`) next to the
   built binary before zipping / running the NSIS installer step.

Linux note: `wails build` needs webkit2gtk dev libs
(`libwebkit2gtk-4.1-dev` on Ubuntu). Windows/macOS builds must run on
their own OS (or CI) — Wails does not cross-compile GUI apps.
Ubuntu shortcut: `sudo apt install ffmpeg` covers video + probing system-wide;
yt-dlp is one click in Setup (or `sudo apt install yt-dlp`, usually outdated).

## Env overrides

| Var | Purpose |
| --- | --- |
| `RECLIP_FFMPEG` | custom ffmpeg path |
| `RECLIP_YTDLP` | custom yt-dlp path |
| `RECLIP_CONFIG_DIR` | override config dir (credit preset; used by tests) |

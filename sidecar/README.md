# Sidecar binaries (ffmpeg, yt-dlp) live here, one folder per OS:
#
#   sidecar/windows/  ffmpeg.exe  yt-dlp.exe
#   sidecar/linux/    ffmpeg      yt-dlp
#   sidecar/darwin/   ffmpeg      yt-dlp
#
# Binaries are git-ignored (large). Fetch them with:
#
#   bash scripts/fetch-sidecars.sh windows   # or: linux | darwin | all
#
# Reclip finds them in this order:
#   1. RECLIP_FFMPEG / RECLIP_YTDLP env vars
#   2. paths saved via the in-app Setup screen
#   3. next to the app executable  <- copy the 2 files beside reclip.exe
#   4. system PATH
#
# For `wails build`, copy the 2 files for your OS next to the produced
# binary, or ship them in the installer folder. The NSIS installer packs
# everything under build/bin, so drop them there before building.
#
# (No ffprobe needed — probing is done with `ffmpeg -i`.)

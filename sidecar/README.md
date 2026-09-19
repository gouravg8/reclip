# Sidecar binaries (ffmpeg, ffprobe, yt-dlp) live here, one folder per OS:
#
#   sidecar/windows/  ffmpeg.exe  ffprobe.exe  yt-dlp.exe
#   sidecar/linux/    ffmpeg      ffprobe      yt-dlp
#   sidecar/darwin/   ffmpeg      ffprobe      yt-dlp
#
# Binaries are git-ignored (large). Fetch them with:
#
#   bash scripts/fetch-sidecars.sh windows   # or: linux | darwin | all
#
# Reclip finds them in this order:
#   1. RECLIP_FFMPEG / RECLIP_FFPROBE / RECLIP_YTDLP env vars
#   2. next to the app executable  <- copy the 3 files beside reclip.exe
#   3. system PATH
#
# For `wails build`, copy the 3 files for your OS next to the produced
# binary, or ship them in the installer folder. The NSIS installer packs
# everything under build/bin, so drop them there before building.

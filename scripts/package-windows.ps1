#Requires -Version 5.1
<#
.SYNOPSIS
  One-command Reclip release for Windows: fetch sidecars, build, NSIS installer.
.DESCRIPTION
  1. Downloads ffmpeg.exe + ffprobe.exe (gyan.dev essentials) and yt-dlp.exe
     into build\bin next to the app binary.
  2. Runs `wails build --nsis`, whose project.nsi bundles those three files
     into the installer, so recipients need zero manual setup.
.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\package-windows.ps1
#>
$ErrorActionPreference = "Stop"

$Root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$Bin = Join-Path $Root "build\bin"
$YtDlpVersion = "2025.01.01"

New-Item -ItemType Directory -Force -Path $Bin | Out-Null
$tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "reclip-sidecar")
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Get-File($Url, $Out) {
  Write-Host "-> $Url"
  Invoke-WebRequest -Uri $Url -OutFile $Out
}

# --- ffmpeg (only if missing; ffprobe no longer needed) ---
if (-not (Test-Path (Join-Path $Bin "ffmpeg.exe"))) {
  $zip = Join-Path $tmp.FullName "ffmpeg.zip"
  Get-File "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip" $zip
  $extract = Join-Path $tmp.FullName "ffmpeg"
  if (Test-Path $extract) { Remove-Item -Recurse -Force $extract }
  Expand-Archive -Path $zip -DestinationPath $extract
  $bindir = Get-ChildItem -Directory (Join-Path $extract "ffmpeg-*-essentials_build") |
    ForEach-Object { Join-Path $_.FullName "bin" } | Select-Object -First 1
  Copy-Item (Join-Path $bindir "ffmpeg.exe") (Join-Path $Bin "ffmpeg.exe") -Force
} else {
  Write-Host "ffmpeg.exe already in build\bin, skipping download."
}

# --- yt-dlp (only if missing; users can also fetch it one-click in Setup) ---
if (-not (Test-Path (Join-Path $Bin "yt-dlp.exe"))) {
  Get-File "https://github.com/yt-dlp/yt-dlp/releases/download/$YtDlpVersion/yt-dlp.exe" (Join-Path $Bin "yt-dlp.exe")
} else {
  Write-Host "yt-dlp.exe already in build\bin, skipping download."
}

Write-Host "Sidecars ready in $Bin :"
Get-ChildItem $Bin -Include ffmpeg.exe, yt-dlp.exe | ForEach-Object {
  Write-Host ("  {0}  ({1:N1} MB)" -f $_.Name, ($_.Length / 1MB))
}

# --- build + installer ---
Push-Location $Root
try {
  wails build --nsis
} finally {
  Pop-Location
}

Get-ChildItem (Join-Path $Bin "*installer*.exe") | ForEach-Object {
  Write-Host ("Installer: {0}" -f $_.FullName)
}

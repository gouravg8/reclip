import type { main } from "../wailsjs/go/models";

export type VideoInfo = main.VideoInfo;

export type ItemStatus = "queued" | "downloading" | "probing" | "ready" | "error";

export interface QueueItem {
  id: string;
  source: "link" | "file";
  /** Original URL for link items. */
  url?: string;
  /** Local file path once downloaded / picked. */
  path: string;
  name: string;
  status: ItemStatus;
  /** Last progress line from yt-dlp. */
  progress?: string;
  error?: string;
  info?: VideoInfo;
  edit: EditSpec;
}

/** A [start, end) section (seconds, source timeline) removed on export. */
export interface CutRange {
  start: number;
  end: number;
}

/**
 * Per-video edit decisions. Times are seconds on the source timeline.
 * trimEnd of 0 means "to the end". Zoom is a scale factor around the
 * center; panX/panY are CSS-style translate percentages applied before
 * scaling (mirrored by the ffmpeg crop in the exporter).
 */
export interface EditSpec {
  trimStart: number;
  trimEnd: number;
  cuts: CutRange[];
  zoom: number;
  panX: number;
  panY: number;
}

export function defaultEdit(): EditSpec {
  return { trimStart: 0, trimEnd: 0, cuts: [], zoom: 1, panX: 0, panY: 0 };
}

/** Seconds kept after trim + cuts. */
export function outputDuration(sourceDuration: number, edit: EditSpec): number {
  const end = edit.trimEnd > 0 ? Math.min(edit.trimEnd, sourceDuration) : sourceDuration;
  const start = Math.min(Math.max(0, edit.trimStart), end);
  let out = Math.max(0, end - start);
  for (const c of edit.cuts) {
    const cs = Math.max(c.start, start);
    const ce = Math.min(c.end, end);
    if (ce > cs) out -= ce - cs;
  }
  return Math.max(0, Math.round(out * 10) / 10);
}

export function basename(p: string): string {
  const parts = p.split(/[/\\]/);
  return parts[parts.length - 1] || p;
}

export function formatDuration(sec?: number): string {
  if (sec === undefined || Number.isNaN(sec)) return "—";
  const s = Math.max(0, Math.floor(sec));
  const m = Math.floor(s / 60);
  const r = s % 60;
  return `${m}:${r.toString().padStart(2, "0")}`;
}

export function formatBytes(n?: number): string {
  if (n === undefined) return "—";
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB"];
  let v = n / 1024;
  let u = 0;
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024;
    u += 1;
  }
  return `${v.toFixed(1)} ${units[u]}`;
}

let counter = 0;

export function newId(): string {
  counter += 1;
  return `${Date.now().toString(36)}-${counter}`;
}

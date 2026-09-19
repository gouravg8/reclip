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

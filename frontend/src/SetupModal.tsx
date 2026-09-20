import { useCallback, useEffect, useState } from "react";
import { Button, InputNumber, Modal, Progress, Space, Typography } from "antd";
import { BrowserOpenURL, EventsOn } from "../wailsjs/runtime/runtime";
import {
  CheckDeps,
  DownloadYtDlp,
  GetCookieSetting,
  GetDownloadDelay,
  GetEnginePaths,
  SelectCookiesFile,
  SelectEngineBinary,
  SetCookieSetting,
  SetDownloadDelay,
  SetEnginePath,
} from "../wailsjs/go/main/App";
import type { main } from "../wailsjs/go/models";

interface Props {
  open: boolean;
  onClose: () => void;
  onChanged: () => void;
}

interface EngineMeta {
  key: string;
  label: string;
  desc: string;
  links: { label: string; url: string }[];
  autoDownload?: boolean;
}

const ENGINES: EngineMeta[] = [
  {
    key: "ffmpeg",
    label: "ffmpeg",
    desc: "Does all video work: trim, zoom, credit join, export. Ships with the installer — only fix this if it shows red.",
    links: [
      { label: "Windows build", url: "https://www.gyan.dev/ffmpeg/builds/" },
      { label: "Ubuntu: sudo apt install ffmpeg", url: "https://johnvansickle.com/ffmpeg/" },
    ],
  },
  {
    key: "ytdlp",
    label: "yt-dlp",
    desc: "Downloads reels from pasted links. One click below fetches it — no manual download needed.",
    links: [{ label: "Manual download", url: "https://github.com/yt-dlp/yt-dlp/releases" }],
    autoDownload: true,
  },
];

function shortVersion(v: string): string {
  if (v.startsWith("missing")) return "";
  return v.length > 48 ? `${v.slice(0, 48)}…` : v;
}

export default function SetupModal({ open, onClose, onChanged }: Props) {
  const [deps, setDeps] = useState<main.Deps | null>(null);
  const [saved, setSaved] = useState<Record<string, string>>({});
  const [cookies, setCookies] = useState("");
  const [delay, setDelay] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [d, s, c, dl] = await Promise.all([
        CheckDeps(),
        GetEnginePaths(),
        GetCookieSetting(),
        GetDownloadDelay(),
      ]);
      setDeps(d);
      setSaved(s ?? {});
      setCookies(c ?? "");
      setDelay(dl ?? 0);
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [onChanged]);

  useEffect(() => {
    if (open) {
      setError(null);
      void refresh();
    }
  }, [open, refresh]);

  const locate = useCallback(
    async (tool: string) => {
      setError(null);
      setBusy(true);
      try {
        const file = await SelectEngineBinary();
        if (!file) return;
        await SetEnginePath(tool, file);
        await refresh();
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [refresh],
  );

  const useAuto = useCallback(
    async (tool: string) => {
      setError(null);
      try {
        await SetEnginePath(tool, "");
        await refresh();
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    },
    [refresh],
  );

  const setLogin = useCallback(async (value: string) => {
    setError(null);
    try {
      await SetCookieSetting(value);
      setCookies(value);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const uploadCookies = useCallback(async () => {
    setError(null);
    try {
      const file = await SelectCookiesFile();
      if (!file) return;
      await SetCookieSetting(`file:${file}`);
      setCookies(`file:${file}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const changeDelay = useCallback(async (sec: number | null) => {
    const v = Math.max(0, Math.min(600, Math.round(sec ?? 0)));
    setDelay(v);
    try {
      await SetDownloadDelay(v);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const depValue = (key: string): string => {
    if (!deps) return "missing";
    if (key === "ffmpeg") return deps.ffmpeg;
    return deps.ytDlp;
  };

  const loginLabel = !cookies
    ? "Not logged in — some reels will fail with “login required”."
    : cookies.startsWith("browser:")
      ? `Logged in via ${cookies.slice("browser:".length)} cookies.`
      : `Logged in via cookies file.`;

  // One-click yt-dlp download progress.
  const [dlFrac, setDlFrac] = useState<number | null>(null);
  useEffect(() => {
    const off = EventsOn("reclip:setup-progress", (p: { done: number; total: number }) => {
      setDlFrac(p.total > 0 ? Math.min(1, Math.max(0, p.done / p.total)) : null);
    });
    return () => {
      off();
    };
  }, []);

  const autoDownload = useCallback(async () => {
    setError(null);
    setDlFrac(0);
    try {
      await DownloadYtDlp();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setDlFrac(null);
    }
  }, [refresh]);

  return (
    <Modal
      title="Setup — tools & Instagram login"
      open={open}
      onCancel={onClose}
      footer={null}
      width={600}
    >
      <Typography.Paragraph type="secondary">
        Reclip needs two free helper programs. The installer bundles them, so this screen should
        already be green — if anything shows red, fix it once below and it is remembered on this PC.
      </Typography.Paragraph>
      {error && <Typography.Paragraph type="danger">{error}</Typography.Paragraph>}
      <div className="flex flex-col gap-4">
        {ENGINES.map((eng) => {
          const v = depValue(eng.key);
          const ok = !v.startsWith("missing");
          const path = saved[eng.key] ?? "";
          return (
            <div
              key={eng.key}
              className="rounded-xl border border-[#e7eaf0] dark:border-[#232b36] p-3"
            >
              <div className="flex items-center gap-2">
                <span className={`dot ${deps ? (ok ? "ok" : "bad") : "idle"}`} />
                <Typography.Text strong>{eng.label}</Typography.Text>
                <Typography.Text type={ok ? "success" : "danger"} className="text-xs">
                  {deps ? (ok ? shortVersion(v) || "found" : "not found") : "checking…"}
                </Typography.Text>
              </div>
              <Typography.Paragraph type="secondary" className="text-xs !mb-2">
                {eng.desc}
              </Typography.Paragraph>
              {path && (
                <Typography.Text type="secondary" className="block text-xs pb-2 break-all">
                  Using: {path}
                </Typography.Text>
              )}
              <Space wrap>
                {eng.autoDownload && !ok && (
                  <Button
                    size="small"
                    type="primary"
                    loading={dlFrac !== null}
                    onClick={() => void autoDownload()}
                  >
                    Download automatically
                  </Button>
                )}
                <Button size="small" loading={busy} onClick={() => void locate(eng.key)}>
                  Locate file…
                </Button>
                {path && (
                  <Button size="small" type="link" onClick={() => void useAuto(eng.key)}>
                    Use auto-detect
                  </Button>
                )}
                {eng.links.map((l) => (
                  <Button
                    key={l.url}
                    size="small"
                    type="link"
                    onClick={() => BrowserOpenURL(l.url)}
                  >
                    {l.label} ↗
                  </Button>
                ))}
              </Space>
              {eng.autoDownload && dlFrac !== null && (
                <Progress
                  percent={Math.round(dlFrac * 100)}
                  size="small"
                  showInfo={dlFrac > 0}
                  className="pt-2"
                />
              )}
            </div>
          );
        })}
      </div>
      <Typography.Title level={5} className="!mt-5">
        Instagram login
      </Typography.Title>
      <Typography.Paragraph type="secondary" className="text-xs">
        Instagram rate-limits anonymous downloads and some reels need login. Log into Instagram in
        your browser first, then pick it below — cookies never leave this PC.
      </Typography.Paragraph>
      <Space direction="vertical" className="w-full">
        <Space wrap>
          {["chrome", "firefox", "edge", "brave"].map((b) => (
            <Button
              key={b}
              size="small"
              type={cookies === `browser:${b}` ? "primary" : "default"}
              onClick={() => void setLogin(`browser:${b}`)}
            >
              Use {b} cookies
            </Button>
          ))}
          <Button size="small" onClick={() => void uploadCookies()}>
            Upload cookies.txt…
          </Button>
          {cookies && (
            <Button size="small" type="link" danger onClick={() => void setLogin("")}>
              Remove login
            </Button>
          )}
        </Space>
        <Typography.Text type={cookies ? "success" : "secondary"} className="text-xs">
          {loginLabel}
        </Typography.Text>
      </Space>
      <Typography.Title level={5} className="!mt-5">
        Download pacing
      </Typography.Title>
      <Typography.Paragraph type="secondary" className="text-xs">
        Pause between downloads so batches don&apos;t look automated. A small random jitter is added
        each time. For ~40/day, 10–15s plus login cookies is a safe everyday setup.
      </Typography.Paragraph>
      <Space>
        <InputNumber
          size="small"
          min={0}
          max={600}
          value={delay}
          onChange={(v) => void changeDelay(v)}
          addonAfter="sec"
        />
        <Typography.Text type="secondary" className="text-xs">
          {delay === 0 ? "off — back-to-back" : `≈${delay}s + jitter between downloads`}
        </Typography.Text>
      </Space>
      <div className="pt-3 flex justify-end">
        <Button onClick={() => void refresh()}>Recheck</Button>
      </div>
    </Modal>
  );
}

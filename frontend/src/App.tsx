import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  ConfigProvider,
  Input,
  Layout,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  theme,
} from "antd";
import {
  CheckCircleOutlined,
  DeleteOutlined,
  MoonOutlined,
  SunOutlined,
  DownloadOutlined,
  ExportOutlined,
  FolderOpenOutlined,
  QuestionCircleOutlined,
  ScissorOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import {
  CheckDeps,
  ClearCreditPreset,
  DownloadReel,
  ExportBatch,
  ExportOne,
  GetCreditPreset,
  ProbeVideo,
  SaveCreditPreset,
  SelectCreditFile,
  SelectOutputDir,
  SelectVideoFiles,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import type { main } from "../wailsjs/go/models";
import {
  basename,
  defaultEdit,
  formatBytes,
  formatDuration,
  newId,
  type CreditMode,
  type EditSpec,
  type ItemStatus,
  type QueueItem,
} from "./types";
import Editor from "./Editor";
import HelpModal from "./HelpModal";

const OUTDIR_KEY = "reclip:outdir";
const CREDIT_ALL_KEY = "reclip:creditAll";

const statusColor: Record<ItemStatus, string> = {
  queued: "default",
  downloading: "processing",
  probing: "processing",
  ready: "success",
  error: "error",
};

function backendError(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}

export default function App() {
  const [deps, setDeps] = useState<main.Deps | null>(null);
  const [items, setItems] = useState<QueueItem[]>([]);
  const [links, setLinks] = useState("");
  const [busy, setBusy] = useState(false);
  const [outDir, setOutDir] = useState("");
  const [credit, setCredit] = useState("");
  const [creditInfo, setCreditInfo] = useState<main.VideoInfo | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [addCreditAll, setAddCreditAll] = useState(true);
  const [batchBusy, setBatchBusy] = useState(false);
  const [exporting, setExporting] = useState<string[]>([]);
  const [helpOpen, setHelpOpen] = useState(false);
  const [isDark, setIsDark] = useState(() => localStorage.getItem("reclip:theme") === "dark");

  useEffect(() => {
    document.documentElement.classList.toggle("dark", isDark);
    localStorage.setItem("reclip:theme", isDark ? "dark" : "light");
  }, [isDark]);
  const [notice, setNotice] = useState<{ type: "error" | "warning"; text: string } | null>(null);

  const activeId = useRef<string | null>(null);
  const busyRef = useRef(false);
  const itemsRef = useRef<QueueItem[]>([]);
  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  // Pending download jobs live in a ref (not state) so freshly added
  // links are processed even before the next render commits.
  const pendingRef = useRef<{ id: string; url: string }[]>([]);

  const patchItem = useCallback((id: string, patch: Partial<QueueItem>) => {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  }, []);

  // Backend progress events -> active download item.
  useEffect(() => {
    const off = EventsOn("reclip:download-progress", (line: string) => {
      const id = activeId.current;
      if (!id) return;
      patchItem(id, { progress: String(line) });
    });
    return () => {
      off();
    };
  }, [patchItem]);

  // Backend export progress events -> per-item export state.
  useEffect(() => {
    const off = EventsOn(
      "reclip:export-progress",
      (p: { id: string; phase: string; done: number; total: number }) => {
        const frac = p.total > 0 ? Math.min(1, Math.max(0, p.done / p.total)) : 0;
        setItems((prev) =>
          prev.map((it) =>
            it.id === p.id
              ? { ...it, exportState: { ...it.exportState, phase: p.phase, frac } }
              : it,
          ),
        );
      },
    );
    return () => {
      off();
    };
  }, []);

  // Initial load: deps, saved credit, saved output dir.
  useEffect(() => {
    CheckDeps()
      .then(setDeps)
      .catch(() => setDeps(null));
    GetCreditPreset()
      .then(async (p) => {
        if (!p) return;
        setCredit(p);
        try {
          setCreditInfo(await ProbeVideo(p));
        } catch {
          setCreditInfo(null);
        }
      })
      .catch(() => {});
    setOutDir(localStorage.getItem(OUTDIR_KEY) ?? "");
    setAddCreditAll(localStorage.getItem(CREDIT_ALL_KEY) !== "0");
  }, []);

  const missingDeps = deps
    ? [deps.ffmpeg, deps.ffprobe, deps.ytDlp].filter((v) => v.startsWith("missing"))
    : [];

  const probeAndReady = useCallback(
    async (id: string, path: string) => {
      patchItem(id, { status: "probing", progress: undefined });
      try {
        const info = await ProbeVideo(path);
        patchItem(id, {
          status: "ready",
          info,
          name: basename(path),
          edit: { ...defaultEdit(), trimEnd: info.duration },
        });
      } catch (e) {
        patchItem(id, { status: "error", error: backendError(e) });
      }
    },
    [patchItem],
  );

  // Single download + probe job.
  const downloadOne = useCallback(
    async (job: { id: string; url: string }, dir: string) => {
      patchItem(job.id, { status: "downloading", progress: "starting…" });
      activeId.current = job.id;
      try {
        const file = await DownloadReel(job.url, dir);
        patchItem(job.id, { path: file, name: basename(file) });
        await probeAndReady(job.id, file);
      } catch (e) {
        patchItem(job.id, { status: "error", error: backendError(e) });
      } finally {
        activeId.current = null;
      }
    },
    [patchItem, probeAndReady],
  );

  // Pump drains pendingRef until empty. Safe to call anytime; concurrent
  // calls no-op via busyRef. Links pasted mid-download are picked up.
  const pump = useCallback(async () => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    try {
      const dir = localStorage.getItem(OUTDIR_KEY) ?? "";
      for (;;) {
        const job = pendingRef.current.shift();
        if (!job) break;
        await downloadOne(job, dir);
      }
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }, [downloadOne]);

  const retryStuck = useCallback(() => {
    const jobs = itemsRef.current
      .filter((it) => it.source === "link" && (it.status === "queued" || it.status === "error"))
      .map((it) => ({ id: it.id, url: it.url ?? "" }))
      .filter((j) => j.url !== "");
    if (jobs.length === 0) return;
    setItems((prev) =>
      prev.map((it) =>
        it.source === "link" && (it.status === "queued" || it.status === "error")
          ? { ...it, status: "queued" as const, error: undefined, progress: undefined }
          : it,
      ),
    );
    pendingRef.current.push(...jobs);
    void pump();
  }, [pump]);

  const addLinks = useCallback(() => {
    setNotice(null);
    const dir = outDir.trim();
    if (!dir) {
      setNotice({ type: "warning", text: "Pick a download folder first." });
      return;
    }
    const urls = links
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean);
    if (urls.length === 0) return;
    const seen = new Set(itemsRef.current.map((it) => it.url ?? it.path));
    const fresh: QueueItem[] = [];
    for (const url of urls) {
      if (seen.has(url)) continue;
      seen.add(url);
      fresh.push({
        id: newId(),
        source: "link",
        url,
        path: "",
        name: url,
        status: "queued",
        edit: defaultEdit(),
        credit: "inherit",
      });
    }
    if (fresh.length === 0) {
      setLinks("");
      return;
    }
    setItems((prev) => [...prev, ...fresh]);
    pendingRef.current.push(...fresh.map((f) => ({ id: f.id, url: f.url ?? "" })));
    setLinks("");
    void pump();
  }, [links, outDir, pump]);

  const addFiles = useCallback(async () => {
    setNotice(null);
    try {
      const paths = await SelectVideoFiles();
      if (!paths || paths.length === 0) return;
      const fresh: QueueItem[] = paths.map((p) => ({
        id: newId(),
        source: "file" as const,
        path: p,
        name: basename(p),
        status: "queued" as const,
        edit: defaultEdit(),
        credit: "inherit" as const,
      }));
      setItems((prev) => [...prev, ...fresh]);
      for (const it of fresh) {
        await probeAndReady(it.id, it.path);
      }
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
    }
  }, [probeAndReady]);

  const pickOutDir = useCallback(async () => {
    try {
      const dir = await SelectOutputDir();
      if (!dir) return;
      setOutDir(dir);
      localStorage.setItem(OUTDIR_KEY, dir);
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
    }
  }, []);

  const pickCredit = useCallback(async () => {
    setNotice(null);
    try {
      const file = await SelectCreditFile();
      if (!file) return;
      await SaveCreditPreset(file);
      setCredit(file);
      try {
        setCreditInfo(await ProbeVideo(file));
      } catch {
        setCreditInfo(null);
      }
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
    }
  }, []);

  const clearCredit = useCallback(async () => {
    try {
      await ClearCreditPreset();
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
      return;
    }
    setCredit("");
    setCreditInfo(null);
  }, []);

  const removeItem = useCallback((id: string) => {
    setItems((prev) => prev.filter((it) => it.id !== id));
    setSelectedId((sel) => (sel === id ? null : sel));
  }, []);

  const clearFinished = useCallback(() => {
    setItems((prev) => prev.filter((it) => it.status !== "ready" && it.status !== "error"));
  }, []);

  const changeEdit = useCallback(
    (id: string, edit: EditSpec) => patchItem(id, { edit }),
    [patchItem],
  );

  const setCreditMode = useCallback(
    (id: string, mode: CreditMode) => patchItem(id, { credit: mode }),
    [patchItem],
  );

  const pickCustomCredit = useCallback(async () => {
    try {
      const file = await SelectCreditFile();
      if (!file) return null;
      return file;
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
      return null;
    }
  }, []);

  const toggleCreditAll = useCallback((on: boolean) => {
    setAddCreditAll(on);
    localStorage.setItem(CREDIT_ALL_KEY, on ? "1" : "0");
  }, []);

  function resolveCreditFor(it: QueueItem): string {
    if (it.credit === "none") return "";
    if (it.credit === "custom") return it.customCredit ?? "";
    return addCreditAll ? credit : "";
  }

  function outputNameFor(it: QueueItem): string {
    const base = basename(it.path || it.url || it.id).replace(/\.[a-zA-Z0-9]+$/, "");
    return `${base}_reclip.mp4`;
  }

  function buildJobJSON(it: QueueItem): string {
    return JSON.stringify({
      id: it.id,
      input: it.path,
      output: `${outDir}/${outputNameFor(it)}`,
      edit: {
        trimStart: it.edit.trimStart,
        trimEnd: it.edit.trimEnd,
        cuts: it.edit.cuts,
        zoom: it.edit.zoom,
        panX: it.edit.panX,
        panY: it.edit.panY,
      },
      creditPath: resolveCreditFor(it),
    });
  }

  const exportOneItem = useCallback(
    async (id: string) => {
      const it = itemsRef.current.find((i) => i.id === id);
      if (!it || it.status !== "ready") return;
      if (!outDir) {
        setNotice({ type: "warning", text: "Pick a download/export folder first." });
        return;
      }
      setExporting((prev) => [...prev, id]);
      patchItem(id, { exportState: { phase: "starting", frac: 0 } });
      try {
        const res = await ExportOne(buildJobJSON(it));
        if (res.error) {
          patchItem(id, { exportState: { phase: "error", frac: 0, error: res.error } });
        } else {
          patchItem(id, { exportState: { phase: "done", frac: 1, outPath: res.output } });
        }
      } catch (e) {
        patchItem(id, { exportState: { phase: "error", frac: 0, error: backendError(e) } });
      } finally {
        setExporting((prev) => prev.filter((x) => x !== id));
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [outDir, addCreditAll, credit, patchItem],
  );

  const exportAllReady = useCallback(async () => {
    const ready = itemsRef.current.filter((it) => it.status === "ready");
    if (ready.length === 0 || batchBusy) return;
    if (!outDir) {
      setNotice({ type: "warning", text: "Pick a download/export folder first." });
      return;
    }
    setBatchBusy(true);
    for (const it of ready) {
      patchItem(it.id, { exportState: { phase: "queued", frac: 0 } });
    }
    try {
      const results = await ExportBatch(JSON.stringify(ready.map(buildJobJSON)));
      for (const res of results) {
        if (res.error) {
          patchItem(res.id, { exportState: { phase: "error", frac: 0, error: res.error } });
        } else {
          patchItem(res.id, { exportState: { phase: "done", frac: 1, outPath: res.output } });
        }
      }
    } catch (e) {
      setNotice({ type: "error", text: backendError(e) });
    } finally {
      setBatchBusy(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [batchBusy, outDir, addCreditAll, credit, patchItem]);

  const readyCount = items.filter((it) => it.status === "ready").length;
  const selected = items.find((it) => it.id === selectedId) ?? null;
  const selectedReady = selected?.status === "ready" && selected.info ? selected : null;
  const stuckCount = items.filter(
    (it) => it.source === "link" && (it.status === "queued" || it.status === "error"),
  ).length;

  const engines: { key: string; label: string; ok: boolean; full: string }[] = deps
    ? [
        {
          key: "ffmpeg",
          label: "ffmpeg",
          ok: !deps.ffmpeg.startsWith("missing"),
          full: deps.ffmpeg,
        },
        {
          key: "ffprobe",
          label: "ffprobe",
          ok: !deps.ffprobe.startsWith("missing"),
          full: deps.ffprobe,
        },
        { key: "ytdlp", label: "yt-dlp", ok: !deps.ytDlp.startsWith("missing"), full: deps.ytDlp },
      ]
    : [];

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: "#0284c7",
          borderRadius: 10,
          colorBgLayout: "#f4f6f9",
          fontFamily:
            '"Nunito", "Inter Variable", Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
        },
      }}
    >
      <Layout className="min-h-screen">
        <Layout.Header className="sticky top-0 z-50 !bg-white dark:!bg-[#11161d] border-b border-[#e7eaf0] dark:border-[#232b36] !h-14 flex items-center gap-3 px-5">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-[#0284c7] text-white flex items-center justify-center text-base font-bold shadow-[0_4px_12px_-2px_rgba(2,132,199,0.5)]">
              R
            </div>
            <div>
              <div className="font-bold text-[14px] leading-tight">Reclip</div>
              <div className="text-[10px] text-[#8a94a6] leading-tight">reel republisher</div>
            </div>
          </div>
          <div className="flex-1" />
          <Space>
            <Button
              type="text"
              icon={isDark ? <SunOutlined /> : <MoonOutlined />}
              title={isDark ? "Switch to light mode" : "Switch to dark mode"}
              onClick={() => setIsDark((d) => !d)}
            />
            <Button
              type="text"
              icon={<QuestionCircleOutlined />}
              title="How to use Reclip"
              onClick={() => setHelpOpen(true)}
            />
            <span>
              {engines.length === 0 && (
                <Typography.Text type="secondary" className="text-xs mr-2">
                  checking engines…
                </Typography.Text>
              )}
              {engines.map((e) => (
                <span
                  key={e.key}
                  title={e.full}
                  className="inline-flex items-center gap-1.5 text-xs text-[#3d4759] dark:text-[#aeb6c2] mr-3"
                >
                  <span className={`dot ${e.ok ? "ok" : "bad"}`} />
                  {e.label}
                </span>
              ))}
            </span>
            <Button
              type="primary"
              icon={<ExportOutlined />}
              loading={batchBusy}
              disabled={readyCount === 0}
              onClick={() => void exportAllReady()}
            >
              Export all ({readyCount})
            </Button>
          </Space>
        </Layout.Header>

        <Layout className="bg-[#f4f6f9] dark:bg-[#0b0f14]">
          <div className="aurora-bg" aria-hidden />
          <Layout.Content className="p-6 pb-12 max-w-6xl w-full mx-auto flex flex-col gap-5 relative z-10">
            <div className="flex items-end justify-between flex-wrap gap-3">
              <div>
                <div className="eyebrow">Reclip studio</div>
                <Typography.Title level={2} className="!mb-1 !mt-1 !text-[26px]">
                  Republish reels in minutes
                </Typography.Title>
                <Typography.Text type="secondary">
                  Paste links → polish each clip → append your credit → ship. Everything renders on
                  this machine.
                </Typography.Text>
              </div>
              <Space>
                <Button icon={<FolderOpenOutlined />} onClick={pickOutDir}>
                  {outDir ? basename(outDir) : "Choose folder"}
                </Button>
              </Space>
            </div>

            {missingDeps.length > 0 && (
              <Alert
                type="error"
                showIcon
                message="Missing dependencies"
                description={missingDeps.join(" · ")}
              />
            )}
            {notice && (
              <Alert
                type={notice.type}
                showIcon
                closable
                message={notice.text}
                onClose={() => setNotice(null)}
              />
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5 section-cv">
              <div>
                <div className="eyebrow pb-2">01 · Source</div>
                <Card title="Add reels" className="soft-card">
                  <Space direction="vertical" className="w-full" size="middle">
                    <div>
                      <Typography.Text strong className="text-[13px]">
                        Download folder
                      </Typography.Text>
                      <Space className="w-full pt-1" wrap>
                        <Button icon={<FolderOpenOutlined />} onClick={pickOutDir}>
                          {outDir ? "Change folder" : "Choose folder"}
                        </Button>
                        <Typography.Text type="secondary" ellipsis className="max-w-60">
                          {outDir || "not set"}
                        </Typography.Text>
                      </Space>
                      <Typography.Text type="secondary" className="text-xs">
                        Pasted links download here. Finished exports land here too as
                        {" <name>_reclip.mp4"}.
                      </Typography.Text>
                    </div>
                    <div>
                      <Typography.Text strong className="text-[13px]">
                        Reel links — one per line
                      </Typography.Text>
                      <Input.TextArea
                        rows={4}
                        className="mt-1"
                        placeholder={
                          "Paste reel links, one per line\nhttps://www.instagram.com/reel/…"
                        }
                        value={links}
                        onChange={(e) => setLinks(e.target.value)}
                      />
                      <Typography.Text type="secondary" className="text-xs">
                        Each link downloads automatically and joins the queue below.
                      </Typography.Text>
                    </div>
                    <div>
                      <Space wrap>
                        <Button
                          type="primary"
                          icon={<DownloadOutlined />}
                          loading={busy}
                          onClick={addLinks}
                        >
                          Add & download
                        </Button>
                        <Button
                          icon={<UploadOutlined />}
                          onClick={addFiles}
                          title="Add video files already on your PC — no link needed"
                        >
                          Upload files
                        </Button>
                      </Space>
                      <Typography.Text type="secondary" className="block text-xs pt-1">
                        Already have the MP4 on your PC? Upload it straight to the queue.
                      </Typography.Text>
                    </div>
                  </Space>
                </Card>
              </div>

              <div>
                <div className="eyebrow pb-2">02 · Branding</div>
                <Card
                  title="Credit clip — added once"
                  className="soft-card"
                  extra={
                    credit ? (
                      <Button size="small" danger onClick={clearCredit}>
                        Clear
                      </Button>
                    ) : null
                  }
                >
                  {credit ? (
                    <Space direction="vertical">
                      <Space>
                        <CheckCircleOutlined className="text-green-600" />
                        <Typography.Text ellipsis className="max-w-70" title={credit}>
                          {basename(credit)}
                        </Typography.Text>
                      </Space>
                      <Typography.Text type="secondary">
                        {creditInfo
                          ? `${formatDuration(creditInfo.duration)} · ${creditInfo.width}×${creditInfo.height} · ${formatBytes(creditInfo.sizeBytes)}`
                          : "saved as default for all exports"}
                      </Typography.Text>
                      <Button size="small" onClick={pickCredit}>
                        Replace
                      </Button>
                    </Space>
                  ) : (
                    <Space direction="vertical">
                      <Typography.Text type="secondary">
                        Your 10s company clip. Saved as default, auto-appended on export.
                      </Typography.Text>
                      <Button icon={<UploadOutlined />} onClick={pickCredit}>
                        Select credit video
                      </Button>
                    </Space>
                  )}
                </Card>
              </div>
            </div>

            <div className="section-cv">
              <div className="eyebrow pb-2">03 · Lineup</div>
              <Card
                title={`Queue — ${readyCount} of ${items.length} ready`}
                className="soft-card"
                extra={
                  <Space>
                    <Button size="small" onClick={retryStuck} disabled={stuckCount === 0}>
                      Retry queued ({stuckCount})
                    </Button>
                    <Button size="small" onClick={clearFinished} disabled={items.length === 0}>
                      Clear finished
                    </Button>
                  </Space>
                }
              >
                <Table<QueueItem>
                  size="middle"
                  rowKey="id"
                  pagination={false}
                  dataSource={items}
                  locale={{
                    emptyText: (
                      <div className="py-8 flex flex-col items-center gap-2">
                        <div className="w-11 h-11 rounded-2xl bg-[#f0f9ff] dark:bg-[#0b2b3a] text-[#0284c7] dark:text-[#38bdf8] flex items-center justify-center text-xl">
                          <DownloadOutlined />
                        </div>
                        <Typography.Text strong>Nothing here yet</Typography.Text>
                        <Typography.Text type="secondary" className="text-xs">
                          Paste reel links above or upload files to start your lineup.
                        </Typography.Text>
                      </div>
                    ),
                  }}
                  rowSelection={{
                    type: "radio",
                    selectedRowKeys: selectedId ? [selectedId] : [],
                    onChange: (keys) => setSelectedId((keys[0] as string) ?? null),
                  }}
                  onRow={(it) => ({ onClick: () => setSelectedId(it.id) })}
                  columns={[
                    {
                      title: "Video",
                      dataIndex: "name",
                      ellipsis: true,
                      render: (_, it) => (
                        <Space direction="vertical" size={0}>
                          <Typography.Text strong ellipsis title={it.path || it.url}>
                            {it.name}
                          </Typography.Text>
                          {it.source === "link" && it.path ? (
                            <Typography.Text type="secondary" ellipsis className="text-xs max-w-80">
                              {it.url}
                            </Typography.Text>
                          ) : null}
                        </Space>
                      ),
                    },
                    {
                      title: "Src",
                      width: 70,
                      render: (_, it) => <Tag>{it.source}</Tag>,
                    },
                    {
                      title: "Status",
                      width: 130,
                      render: (_, it) => <Tag color={statusColor[it.status]}>{it.status}</Tag>,
                    },
                    {
                      title: "Detail",
                      ellipsis: true,
                      render: (_, it) => {
                        if (it.status === "error")
                          return <Typography.Text type="danger">{it.error}</Typography.Text>;
                        if (it.status === "downloading")
                          return (
                            <Typography.Text type="secondary">{it.progress ?? "…"}</Typography.Text>
                          );
                        if (it.status === "ready" && it.info)
                          return (
                            <Typography.Text type="secondary">
                              {formatDuration(it.info.duration)} · {it.info.width}×{it.info.height}{" "}
                              · {formatBytes(it.info.sizeBytes)}
                            </Typography.Text>
                          );
                        return <Typography.Text type="secondary">…</Typography.Text>;
                      },
                    },
                    {
                      title: "Credit",
                      width: 150,
                      render: (_, it) => (
                        <Space direction="vertical" size={2} className="w-full">
                          <Select<CreditMode>
                            size="small"
                            className="w-full"
                            value={it.credit}
                            onClick={(e) => e.stopPropagation()}
                            onChange={(v) => setCreditMode(it.id, v)}
                            options={[
                              {
                                value: "inherit",
                                label: addCreditAll ? "Default (on)" : "Default (off)",
                              },
                              { value: "none", label: "None" },
                              { value: "custom", label: "Custom…" },
                            ]}
                          />
                          {it.credit === "custom" && (
                            <Button
                              size="small"
                              type="link"
                              className="!p-0 text-xs"
                              onClick={async (e) => {
                                e.stopPropagation();
                                const f = await pickCustomCredit();
                                if (f) patchItem(it.id, { customCredit: f });
                              }}
                            >
                              {it.customCredit ? basename(it.customCredit) : "pick file…"}
                            </Button>
                          )}
                        </Space>
                      ),
                    },
                    {
                      title: "Export",
                      ellipsis: true,
                      render: (_, it) => {
                        const xs = it.exportState;
                        if (it.status !== "ready")
                          return <Typography.Text type="secondary">…</Typography.Text>;
                        if (xs?.phase === "done" && xs.outPath)
                          return (
                            <Typography.Text type="success" ellipsis title={xs.outPath}>
                              ✓ {basename(xs.outPath)}
                            </Typography.Text>
                          );
                        if (xs && xs.phase !== "error" && (exporting.includes(it.id) || batchBusy))
                          return (
                            <Typography.Text type="secondary">
                              {xs.phase} {Math.round(xs.frac * 100)}%
                            </Typography.Text>
                          );
                        if (xs?.phase === "error")
                          return <Typography.Text type="danger">{xs.error}</Typography.Text>;
                        return <Typography.Text type="secondary">ready</Typography.Text>;
                      },
                    },
                    {
                      title: "",
                      width: 90,
                      render: (_, it) => (
                        <Space size={0}>
                          {it.status === "ready" && (
                            <Button
                              size="small"
                              type="text"
                              icon={<ExportOutlined />}
                              title="Export this video"
                              loading={exporting.includes(it.id)}
                              onClick={(e) => {
                                e.stopPropagation();
                                void exportOneItem(it.id);
                              }}
                            />
                          )}
                          <Button
                            size="small"
                            danger
                            type="text"
                            icon={<DeleteOutlined />}
                            onClick={() => removeItem(it.id)}
                          />
                        </Space>
                      ),
                    },
                  ]}
                />
              </Card>
            </div>

            <div className="section-cv">
              <div className="eyebrow pb-2">04 · Polish</div>
              {selectedReady ? (
                <Editor
                  item={selectedReady}
                  onChange={(edit) => changeEdit(selectedReady.id, edit)}
                />
              ) : (
                <Card className="soft-card">
                  <div className="py-6 flex flex-col items-center gap-2">
                    <div className="w-11 h-11 rounded-2xl bg-[#f0f9ff] dark:bg-[#0b2b3a] text-[#0284c7] dark:text-[#38bdf8] flex items-center justify-center text-xl">
                      <ScissorOutlined />
                    </div>
                    <Typography.Text strong>
                      {selected
                        ? `“${selected.name}” isn’t ready yet (${selected.status})`
                        : "Select a ready video to edit"}
                    </Typography.Text>
                    <Typography.Text type="secondary" className="text-xs">
                      Trim, cut sections, and zoom away usernames — click any ready row in the
                      queue.
                    </Typography.Text>
                  </div>
                </Card>
              )}
            </div>

            <div className="section-cv">
              <div className="eyebrow pb-2">05 · Ship</div>
              <Card
                title="Export all — 1080×1920 + credit"
                className="soft-card"
                extra={
                  <Button
                    type="primary"
                    size="small"
                    icon={<ExportOutlined />}
                    loading={batchBusy}
                    disabled={readyCount === 0}
                    onClick={() => void exportAllReady()}
                  >
                    Export all ({readyCount})
                  </Button>
                }
              >
                <Space direction="vertical" className="w-full">
                  <Checkbox
                    checked={addCreditAll}
                    onChange={(e) => toggleCreditAll(e.target.checked)}
                  >
                    Add credit video to all exports
                  </Checkbox>
                  <Typography.Text type="secondary" className="text-xs">
                    {credit
                      ? `Default credit: ${basename(credit)}. Per-video override in the Credit column. Files land in the download folder as <name>_reclip.mp4.`
                      : "No default credit set — add one in section 2, or pick per-video Custom files."}
                  </Typography.Text>
                </Space>
              </Card>
            </div>
          </Layout.Content>
        </Layout>
      </Layout>
      <HelpModal open={helpOpen} onClose={() => setHelpOpen(false)} />
    </ConfigProvider>
  );
}

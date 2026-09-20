import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Card, InputNumber, List, Slider, Space, Tag, Typography } from "antd";
import { DeleteOutlined, PlayCircleOutlined, ReloadOutlined } from "@ant-design/icons";
import {
  defaultEdit,
  formatDuration,
  outputDuration,
  type CutRange,
  type EditSpec,
  type QueueItem,
} from "./types";

interface Props {
  item: QueueItem;
  previewBase: string;
  onChange: (edit: EditSpec) => void;
}

function clamp(n: number, lo: number, hi: number): number {
  if (Number.isNaN(n)) return lo;
  return Math.min(hi, Math.max(lo, n));
}

export default function Editor({ item, previewBase, onChange }: Props) {
  const duration = item.info?.duration ?? 0;
  const edit = item.edit;
  const end = edit.trimEnd > 0 ? edit.trimEnd : duration;

  const videoRef = useRef<HTMLVideoElement>(null);
  const [now, setNow] = useState(0);
  const [cutStart, setCutStart] = useState(0);
  const [cutEnd, setCutEnd] = useState(0);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Reset playback position when switching videos.
  useEffect(() => {
    setNow(0);
    setCutStart(0);
    setCutEnd(0);
    setLoadError(null);
  }, [item.id, item.path]);

  const set = useCallback(
    (patch: Partial<EditSpec>) => onChange({ ...edit, ...patch }),
    [edit, onChange],
  );

  // Enforce trim + skip cuts live during preview.
  const onTimeUpdate = useCallback(() => {
    const v = videoRef.current;
    if (!v || duration <= 0) return;
    const t = v.currentTime;
    setNow(t);
    for (const c of edit.cuts) {
      if (t >= c.start && t < c.end) {
        v.currentTime = c.end;
        return;
      }
    }
    const stopAt = edit.trimEnd > 0 ? Math.min(edit.trimEnd, duration) : duration;
    if (t >= stopAt || t < edit.trimStart) {
      v.pause();
      if (t >= stopAt) v.currentTime = edit.trimStart;
    }
  }, [duration, edit]);

  const previewSelection = useCallback(() => {
    const v = videoRef.current;
    if (!v) return;
    v.currentTime = edit.trimStart;
    void v.play().catch(() => {});
  }, [edit.trimStart]);

  const addCut = useCallback(() => {
    const s = clamp(Math.min(cutStart, cutEnd), 0, duration);
    const e = clamp(Math.max(cutStart, cutEnd), 0, duration);
    if (e - s < 0.1 || duration <= 0) return;
    const next: CutRange[] = [...edit.cuts, { start: s, end: e }].sort((a, b) => a.start - b.start);
    set({ cuts: next });
  }, [cutStart, cutEnd, duration, edit.cuts, set]);

  const removeCut = useCallback(
    (idx: number) => set({ cuts: edit.cuts.filter((_, i) => i !== idx) }),
    [edit.cuts, set],
  );

  const src = `${previewBase}/localfile?path=${encodeURIComponent(item.path)}`;

  return (
    <Card
      title={`Editing — ${item.name}`}
      className="soft-card"
      extra={<Tag color="#ea580c">output ≈ {formatDuration(outputDuration(duration, edit))}</Tag>}
    >
      <div className="flex flex-col md:flex-row gap-5">
        {/* Phone-frame preview */}
        <div className="flex justify-center">
          <div className="w-45 aspect-[9/16] max-h-105 bg-black rounded-2xl overflow-hidden relative shadow-[0_12px_32px_-12px_rgba(16,24,40,0.35)] ring-1 ring-black/10">
            <video
              key={`${item.id}-${item.path}`}
              ref={videoRef}
              src={src}
              controls
              playsInline
              preload="metadata"
              onTimeUpdate={onTimeUpdate}
              onSeeked={onTimeUpdate}
              onError={() =>
                setLoadError(
                  "Preview failed to load this file. It may be moved, locked, or an unsupported codec — export uses ffmpeg directly and is unaffected.",
                )
              }
              className="w-full h-full object-contain"
              style={{
                transform: `translate(${edit.panX}%, ${edit.panY}%) scale(${edit.zoom})`,
              }}
            />
            {loadError && (
              <div className="absolute inset-x-2 bottom-2 rounded-lg bg-black/80 text-white text-xs p-2">
                {loadError}
              </div>
            )}
          </div>
        </div>

        <div className="flex-1 flex flex-col gap-4 min-w-0">
          <div>
            <Space className="w-full justify-between">
              <Typography.Text strong>
                Trim — keep {formatDuration(edit.trimStart)} → {formatDuration(end)}
              </Typography.Text>
              <Button size="small" icon={<PlayCircleOutlined />} onClick={previewSelection}>
                Preview selection
              </Button>
            </Space>
            <Slider
              range
              min={0}
              max={duration}
              step={0.1}
              value={[edit.trimStart, end]}
              onChange={([s, e]) => set({ trimStart: s, trimEnd: e })}
              tooltip={{ formatter: (v) => formatDuration(v) }}
            />
            <Typography.Text type="secondary" className="text-xs">
              time {formatDuration(now)} / {formatDuration(duration)}
            </Typography.Text>
          </div>

          <div>
            <Typography.Text strong>Cut — remove a middle section</Typography.Text>
            <Space wrap className="w-full mt-1">
              <InputNumber
                size="small"
                min={0}
                max={duration}
                step={0.1}
                value={cutStart}
                onChange={(v) => setCutStart(v ?? 0)}
                addonBefore="from"
              />
              <InputNumber
                size="small"
                min={0}
                max={duration}
                step={0.1}
                value={cutEnd}
                onChange={(v) => setCutEnd(v ?? 0)}
                addonBefore="to"
              />
              <Button size="small" onClick={addCut}>
                Add cut
              </Button>
            </Space>
            {edit.cuts.length > 0 && (
              <List
                size="small"
                className="mt-2"
                dataSource={edit.cuts}
                renderItem={(c, i) => (
                  <List.Item
                    actions={[
                      <Button
                        key="del"
                        size="small"
                        type="text"
                        danger
                        icon={<DeleteOutlined />}
                        onClick={() => removeCut(i)}
                      />,
                    ]}
                  >
                    <Typography.Text className="text-xs">
                      ✂ {formatDuration(c.start)} → {formatDuration(c.end)}
                    </Typography.Text>
                  </List.Item>
                )}
              />
            )}
          </div>

          <div>
            <Space className="w-full justify-between">
              <Typography.Text strong>Zoom — hide username / watermark</Typography.Text>
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() =>
                  onChange({
                    ...defaultEdit(),
                    trimStart: edit.trimStart,
                    trimEnd: edit.trimEnd,
                    cuts: edit.cuts,
                  })
                }
              >
                Reset zoom
              </Button>
            </Space>
            <div className="mt-1">
              <Typography.Text className="text-xs">Scale {edit.zoom.toFixed(2)}×</Typography.Text>
              <Slider
                min={1}
                max={2.5}
                step={0.05}
                value={edit.zoom}
                onChange={(z) => set({ zoom: z })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Typography.Text className="text-xs">Pan X {edit.panX}%</Typography.Text>
                <Slider
                  min={-50}
                  max={50}
                  step={1}
                  value={edit.panX}
                  disabled={edit.zoom <= 1}
                  onChange={(v) => set({ panX: v })}
                />
              </div>
              <div>
                <Typography.Text className="text-xs">Pan Y {edit.panY}%</Typography.Text>
                <Slider
                  min={-50}
                  max={50}
                  step={1}
                  value={edit.panY}
                  disabled={edit.zoom <= 1}
                  onChange={(v) => set({ panX: edit.panX, panY: v })}
                />
              </div>
            </div>
            <Typography.Text type="secondary" className="text-xs">
              Tip: usernames usually sit at the top — zoom to ~1.3× and pan up (negative Y).
            </Typography.Text>
          </div>
        </div>
      </div>
    </Card>
  );
}

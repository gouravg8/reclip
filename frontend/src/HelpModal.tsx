import { Modal, Steps, Typography } from "antd";

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function HelpModal({ open, onClose }: Props) {
  return (
    <Modal title="How to use Reclip" open={open} onCancel={onClose} footer={null} width={560}>
      <Typography.Paragraph type="secondary">
        Turn Instagram reels into finished, credit-tagged videos in five steps. All video processing
        happens on this machine — only link downloads need internet.
      </Typography.Paragraph>
      <Steps
        direction="vertical"
        size="small"
        current={-1}
        items={[
          {
            title: "Add reels",
            description:
              "Pick a download folder first. Paste reel links (one per line) and hit Add & download — each clip joins the queue automatically. Already have the MP4 on your PC? Use Upload files instead, no link needed.",
          },
          {
            title: "Set your credit clip once",
            description:
              "Select your ~10s company video. It is saved as the default and auto-appended at the end of every export. Replace or clear it anytime.",
          },
          {
            title: "Check the queue",
            description:
              "Click a row to select it for editing. Retry queued restarts stuck downloads; the Credit column overrides the default per video (Default / None / Custom file).",
          },
          {
            title: "Polish each video",
            description:
              "Trim the start/end with the range slider, cut out middle sections with from–to + Add cut, and zoom/pan to hide usernames or watermarks (try ~1.3× and pan up). Preview skips cuts live; the output estimate shows kept seconds.",
          },
          {
            title: "Export",
            description:
              "Export one video from its row action, or Export all from the top bar. Files land in your folder as <name>_reclip.mp4 (1080×1920, 30fps) and never overwrite — reruns get _2, _3 suffixes.",
          },
        ]}
      />
      <Typography.Title level={5} className="!mt-4">
        Troubleshooting
      </Typography.Title>
      <Typography.Paragraph type="secondary" className="text-[13px]">
        Red engine dots in the top bar mean ffmpeg or yt-dlp wasn&apos;t found — open Setup from the
        engines badge to download or locate them. If a download fails with “login required” or
        “rate-limit”, add Instagram login in Setup (browser cookies or cookies.txt) and retry.
        Private or deleted reels can&apos;t be downloaded — check the link opens while logged out.
      </Typography.Paragraph>
    </Modal>
  );
}

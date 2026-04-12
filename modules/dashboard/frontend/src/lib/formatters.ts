const BPS_UNITS = ["bps", "Kbps", "Mbps", "Gbps", "Tbps"];
const BYTE_UNITS = ["B", "KiB", "MiB", "GiB", "TiB"];

export function formatBps(bps: number): string {
  if (bps === 0) return "0 bps";
  let idx = 0;
  let val = bps;
  while (val >= 1000 && idx < BPS_UNITS.length - 1) {
    val /= 1000;
    idx++;
  }
  return idx === 0 ? `${Math.round(val)} ${BPS_UNITS[idx]}` : `${val.toFixed(1)} ${BPS_UNITS[idx]}`;
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  let idx = 0;
  let val = bytes;
  while (val >= 1024 && idx < BYTE_UNITS.length - 1) {
    val /= 1024;
    idx++;
  }
  return idx === 0 ? `${Math.round(val)} ${BYTE_UNITS[idx]}` : `${val.toFixed(1)} ${BYTE_UNITS[idx]}`;
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export function formatPercent(n: number): string {
  const pct = n <= 1 ? n * 100 : n;
  return `${Math.round(pct)}%`;
}

export function formatRelativeAgo(iso: string | null | undefined): string {
  if (iso == null) return "never";
  const diffMs = Date.now() - new Date(iso).getTime();
  const diffS = Math.max(0, Math.floor(diffMs / 1000));
  return formatDuration(diffS) + " ago";
}

export function formatAbsoluteTime(iso: string | null | undefined): string {
  if (iso == null) return "—";
  const d = new Date(iso);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

import { useQuery } from "@tanstack/react-query";
import { WifiOff } from "lucide-react";
import {
  Area,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { api } from "@/lib/api";
import type { ClientTraffic, TrafficSample } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { formatAbsoluteTime, formatBps, formatBytes } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";

/* ------------------------------------------------------------------ */
/*  Chart                                                              */
/* ------------------------------------------------------------------ */

type Row = {
  t: string;
  rx_bps: number;
  tx_bps: number;
  label: string;
};

function buildRows(samples: TrafficSample[] | null | undefined): Row[] {
  if (!samples) return [];
  return samples.map((s) => ({
    t: s.t,
    rx_bps: s.rx_bps,
    tx_bps: s.tx_bps,
    label: formatAbsoluteTime(s.t),
  }));
}

function ChartTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: Array<{ payload: Row }>;
}) {
  if (!active || !payload || payload.length === 0) return null;
  const row = payload[0].payload;
  return (
    <div className="bg-surface-high rounded-sm px-2 py-1 text-[11px] font-mono leading-tight">
      <p className="text-on-surface-variant">{row.label}</p>
      <p className="text-emerald">{"\u2193 "}{formatBps(row.rx_bps)}</p>
      <p className="text-info">{"\u2191 "}{formatBps(row.tx_bps)}</p>
    </div>
  );
}

function TrafficChart({ rows }: { rows: Row[] }) {
  return (
    <div className="h-56">
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="client-rx-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="hsl(var(--emerald))" stopOpacity={0.25} />
              <stop offset="100%" stopColor="hsl(var(--emerald))" stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="label"
            tick={{ fill: "hsl(var(--on-surface-variant))", fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            minTickGap={32}
          />
          <YAxis
            tick={{ fill: "hsl(var(--on-surface-variant))", fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            width={56}
            tickFormatter={(v: number) => formatBps(v)}
          />
          <Tooltip
            cursor={{ stroke: "hsl(var(--outline-variant))", strokeDasharray: "3 3" }}
            content={<ChartTooltip />}
          />
          <Area
            type="monotone"
            dataKey="rx_bps"
            stroke="hsl(var(--emerald))"
            fill="url(#client-rx-fill)"
            strokeWidth={1.5}
            isAnimationActive={false}
          />
          <Line
            type="monotone"
            dataKey="tx_bps"
            stroke="hsl(var(--info))"
            strokeWidth={1.5}
            strokeDasharray="4 3"
            dot={false}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Stat cards                                                         */
/* ------------------------------------------------------------------ */

function StatCard({
  label,
  primary,
  secondary,
}: {
  label: string;
  primary: string;
  secondary?: string;
}) {
  return (
    <div className="bg-surface-container rounded-sm p-3">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className="text-base font-semibold">{primary}</MonoText>
      {secondary && (
        <MonoText className="block text-xs text-on-surface-variant mt-0.5">
          {secondary}
        </MonoText>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Panel                                                              */
/* ------------------------------------------------------------------ */

export function TrafficGraph({ ip }: { ip: string }) {
  const q = useQuery({
    queryKey: queryKeys.clientTraffic(ip),
    queryFn: () => api.clientTraffic(ip),
    enabled: !!ip,
    refetchInterval: 3_000,
    retry: (count, error) => {
      if (error instanceof Error && error.message.includes("404")) return false;
      return count < 3;
    },
  });

  if (q.isError && !q.data) return null;
  if (q.isPending || !q.data) {
    return (
      <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant">
        Loading traffic...
      </div>
    );
  }

  const data: ClientTraffic = q.data.data;

  if (data.lease_status === "non-dynamic") {
    return (
      <div className="bg-surface-container rounded-sm p-4 flex items-center gap-2 text-xs text-on-surface-variant">
        <WifiOff size={14} strokeWidth={1.5} />
        <span>
          Per-client accounting unavailable for static leases.
        </span>
      </div>
    );
  }

  const rows = buildRows(data.samples);

  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-4">
      <div className="flex items-center justify-between">
        <p className="label-xs">Traffic</p>
        <div className="flex items-center gap-3 text-[10px] font-mono">
          <span className="flex items-center gap-1.5 text-emerald">
            <span className="inline-block h-1 w-3 bg-emerald rounded-sm" />
            RX (download)
          </span>
          <span className="flex items-center gap-1.5 text-info">
            <span className="inline-block h-[1px] w-3 border-t border-dashed border-info" />
            TX (upload)
          </span>
        </div>
      </div>

      {rows.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono h-56 flex items-center justify-center">
          No samples yet.
        </p>
      ) : (
        <TrafficChart rows={rows} />
      )}

      <div className="grid grid-cols-3 gap-3">
        <StatCard
          label="Current"
          primary={`\u2193 ${formatBps(data.current_rx_bps)}`}
          secondary={`\u2191 ${formatBps(data.current_tx_bps)}`}
        />
        <StatCard
          label="Peak (10m)"
          primary={`\u2193 ${formatBps(data.peak_rx_bps_10m)}`}
          secondary={`\u2191 ${formatBps(data.peak_tx_bps_10m)}`}
        />
        <StatCard
          label="Total (10m)"
          primary={`\u2193 ${formatBytes(data.total_rx_bytes_10m)}`}
          secondary={`\u2191 ${formatBytes(data.total_tx_bytes_10m)}`}
        />
      </div>
    </div>
  );
}

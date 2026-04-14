import { useQuery } from "@tanstack/react-query";
import { WifiOff } from "lucide-react";
import { api } from "@/lib/api";
import type {
  ClientSparklines,
  DnsQpsSample,
  FlowCountSample,
  TrafficSample,
} from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { formatBps, formatDuration } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";
import { Sparkline } from "@/components/Sparkline";

/* ------------------------------------------------------------------ */
/*  Row primitive                                                      */
/* ------------------------------------------------------------------ */

function Row({
  label,
  data,
  tickSeconds,
  renderValue,
  color,
  title,
}: {
  label: string;
  data: number[];
  tickSeconds: number;
  renderValue: () => string;
  color?: string;
  title?: string;
}) {
  const windowSeconds = tickSeconds * data.length;
  const hoverTitle =
    title ??
    (data.length > 0
      ? `${data.length} samples \u00D7 ${tickSeconds}s = last ${formatDuration(windowSeconds)}`
      : "No samples yet");
  const empty = data.length === 0;
  return (
    <div
      title={hoverTitle}
      className="flex items-center gap-3 py-1.5"
    >
      <span className="label-xs w-24 shrink-0">{label}</span>
      <div className="w-[120px] h-6 shrink-0">
        {empty ? (
          <span className="flex h-full items-center text-on-surface-variant text-xs font-mono">
            --
          </span>
        ) : (
          <Sparkline data={data} color={color} className="h-full" />
        )}
      </div>
      <MonoText className="ml-auto text-xs">{renderValue()}</MonoText>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function lastValue<T extends { queries_per_window: number } | { open_flows: number }>(
  arr: T[] | null,
  key: keyof T,
): number {
  if (!arr || arr.length === 0) return 0;
  const last = arr[arr.length - 1];
  return Number(last[key]);
}

function lastTraffic(samples: TrafficSample[] | null): {
  rx: number;
  tx: number;
} {
  if (!samples || samples.length === 0) return { rx: 0, tx: 0 };
  const last = samples[samples.length - 1];
  return { rx: last.rx_bps, tx: last.tx_bps };
}

/* ------------------------------------------------------------------ */
/*  Panel                                                              */
/* ------------------------------------------------------------------ */

export function ActivitySparklines({ ip }: { ip: string }) {
  const q = useQuery({
    queryKey: queryKeys.clientSparklines(ip),
    queryFn: () => api.clientSparklines(ip),
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
        Loading activity...
      </div>
    );
  }

  const data: ClientSparklines = q.data.data;

  if (data.lease_status === "non-dynamic") {
    return (
      <div className="bg-surface-container rounded-sm p-4 flex items-center gap-2 text-xs text-on-surface-variant">
        <WifiOff size={14} strokeWidth={1.5} />
        <span>Activity unavailable for static / neighbor leases.</span>
      </div>
    );
  }

  const tick = data.tick_seconds || 0;
  const traffic: TrafficSample[] = data.traffic ?? [];
  const dnsQps: DnsQpsSample[] = data.dns_qps ?? [];
  const flows: FlowCountSample[] = data.flow_count ?? [];

  const rxSeries = traffic.map((s) => s.rx_bps);
  const txSeries = traffic.map((s) => s.tx_bps);
  const dnsSeries = dnsQps.map((s) => s.queries_per_window);
  const flowSeries = flows.map((s) => s.open_flows);

  const { rx, tx } = lastTraffic(traffic);
  const currentDnsQps = lastValue(dnsQps, "queries_per_window");
  const currentFlows = lastValue(flows, "open_flows");

  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-1">
      <p className="label-xs mb-2">Activity</p>
      <Row
        label="DNS qps"
        data={dnsSeries}
        tickSeconds={tick}
        color="hsl(var(--primary))"
        renderValue={() => `${currentDnsQps.toLocaleString()} /w`}
      />
      <Row
        label="Open flows"
        data={flowSeries}
        tickSeconds={tick}
        color="hsl(var(--info))"
        renderValue={() => currentFlows.toLocaleString()}
      />
      {/* Traffic: two mini sparklines side-by-side (RX / TX) sharing one row */}
      <div
        title={
          traffic.length > 0
            ? `${traffic.length} samples \u00D7 ${tick}s = last ${formatDuration(tick * traffic.length)}`
            : "No samples yet"
        }
        className="flex items-center gap-3 py-1.5"
      >
        <span className="label-xs w-24 shrink-0">{"Traffic \u2193/\u2191"}</span>
        <div className="flex items-center gap-1 w-[120px] h-6 shrink-0">
          {rxSeries.length === 0 ? (
            <span className="flex h-full items-center text-on-surface-variant text-xs font-mono">
              --
            </span>
          ) : (
            <>
              <Sparkline
                data={rxSeries}
                color="hsl(var(--emerald))"
                className="h-full flex-1"
              />
              <Sparkline
                data={txSeries}
                color="hsl(var(--info))"
                className="h-full flex-1"
              />
            </>
          )}
        </div>
        <MonoText className="ml-auto text-xs">
          <span className="text-emerald">{"\u2193 "}{formatBps(rx)}</span>
          <span className="mx-1 text-on-surface-variant">{"\u00B7"}</span>
          <span className="text-info">{"\u2191 "}{formatBps(tx)}</span>
        </MonoText>
      </div>
    </div>
  );
}

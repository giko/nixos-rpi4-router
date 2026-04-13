import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Interface } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { Sparkline } from "@/components/Sparkline";
import { formatBytes, formatBps } from "@/lib/formatters";

const ROLE_ORDER: Record<string, number> = {
  wan: 0,
  lan: 1,
  tunnel: 2,
  "": 3,
};

function sortInterfaces(a: Interface, b: Interface): number {
  const ra = ROLE_ORDER[a.role] ?? 3;
  const rb = ROLE_ORDER[b.role] ?? 3;
  if (ra !== rb) return ra - rb;
  return a.name.localeCompare(b.name);
}

function InterfaceCard({ iface }: { iface: Interface }) {
  const rxSeries = iface.samples_60s.map((s) => s.rx_bps);
  const txSeries = iface.samples_60s.map((s) => s.tx_bps);
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold">
            <MonoText>{iface.name}</MonoText>
          </h2>
          {iface.role && (
            <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
              {iface.role}
            </span>
          )}
        </div>
        <StatusBadge
          kind={iface.operstate === "up" ? "healthy" : iface.operstate === "down" ? "failed" : "muted"}
        >
          {iface.operstate}
        </StatusBadge>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <p className="label-xs mb-1">RX bps (60s)</p>
          <Sparkline data={rxSeries} className="h-10" />
          <div className="flex items-baseline justify-between mt-1">
            <MonoText className="text-xs text-on-surface-variant">
              now {formatBps(iface.rx_bps)}
            </MonoText>
            <MonoText className="text-xs text-on-surface-variant">
              total {formatBytes(iface.rx_bytes_total)}
            </MonoText>
          </div>
        </div>
        <div>
          <p className="label-xs mb-1">TX bps (60s)</p>
          <Sparkline data={txSeries} className="h-10" />
          <div className="flex items-baseline justify-between mt-1">
            <MonoText className="text-xs text-on-surface-variant">
              now {formatBps(iface.tx_bps)}
            </MonoText>
            <MonoText className="text-xs text-on-surface-variant">
              total {formatBytes(iface.tx_bytes_total)}
            </MonoText>
          </div>
        </div>
      </div>
    </div>
  );
}

export function Traffic() {
  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 2_000,
  });

  if (trafficQ.isError && !trafficQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load traffic data — retry shortly.
      </div>
    );
  }
  if (trafficQ.isPending || !trafficQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const interfaces = [...trafficQ.data.data.interfaces].sort(sortInterfaces);
  const refetchFailed = trafficQ.isError;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Traffic</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={trafficQ.data.stale ?? false}
            updatedAt={trafficQ.data.updated_at ?? null}
          />
        </div>
      </div>

      {interfaces.length === 0 ? (
        <p className="text-sm text-on-surface-variant font-mono">
          No interfaces reported.
        </p>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {interfaces.map((iface) => (
            <InterfaceCard key={iface.name} iface={iface} />
          ))}
        </div>
      )}
    </div>
  );
}

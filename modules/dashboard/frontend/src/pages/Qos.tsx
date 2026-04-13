import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { QdiscStats, CAKETin } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatBps } from "@/lib/formatters";

function StatRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-xs font-mono">
      <span className="text-on-surface-variant">{label}</span>
      <MonoText>{value}</MonoText>
    </div>
  );
}

const tinColumns: Column<CAKETin>[] = [
  {
    key: "name",
    label: "Tin",
    render: (r) => <MonoText className="text-xs">{r.name}</MonoText>,
    sortValue: (r) => r.name,
  },
  {
    key: "thresh",
    label: "Thresh",
    render: (r) =>
      r.thresh_kbit > 0 ? (
        <MonoText className="text-xs">{r.thresh_kbit.toLocaleString()} kbit</MonoText>
      ) : (
        <span className="text-on-surface-variant">—</span>
      ),
    sortValue: (r) => r.thresh_kbit,
    className: "text-right",
  },
  {
    key: "bytes",
    label: "Bytes",
    render: (r) => <MonoText className="text-xs">{formatBytes(r.bytes)}</MonoText>,
    sortValue: (r) => r.bytes,
    className: "text-right",
  },
  {
    key: "drops",
    label: "Drops",
    render: (r) => (
      <MonoText className={`text-xs ${r.drops > 0 ? "text-amber" : ""}`}>
        {r.drops.toLocaleString()}
      </MonoText>
    ),
    sortValue: (r) => r.drops,
    className: "text-right",
  },
  {
    key: "marks",
    label: "ECN marks",
    render: (r) => (
      <MonoText className="text-xs">{r.marks.toLocaleString()}</MonoText>
    ),
    sortValue: (r) => r.marks,
    className: "text-right",
  },
];

function EgressCard({ stats }: { stats: QdiscStats }) {
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">WAN egress (CAKE)</h2>
        {stats.bandwidth_bps > 0 && (
          <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
            shaped {formatBps(stats.bandwidth_bps)}
          </span>
        )}
      </div>
      <div className="space-y-1">
        <StatRow label="Sent" value={`${formatBytes(stats.sent_bytes)} / ${stats.sent_packets.toLocaleString()} pkt`} />
        <StatRow label="Dropped" value={stats.dropped.toLocaleString()} />
        <StatRow label="Overlimits" value={stats.overlimits.toLocaleString()} />
        <StatRow label="Requeues" value={stats.requeues.toLocaleString()} />
        <StatRow label="Backlog" value={`${formatBytes(stats.backlog_bytes)} / ${stats.backlog_pkts} pkt`} />
      </div>
      {stats.tins && stats.tins.length > 0 && (
        <div className="space-y-2">
          <p className="label-xs">Per-tin breakdown</p>
          <DataTable columns={tinColumns} rows={stats.tins} rowKey={(r) => r.name} />
        </div>
      )}
    </div>
  );
}

function IngressCard({ stats }: { stats: QdiscStats }) {
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">WAN ingress (HTB + fq_codel)</h2>
      </div>
      <div className="space-y-1">
        <StatRow label="Sent" value={`${formatBytes(stats.sent_bytes)} / ${stats.sent_packets.toLocaleString()} pkt`} />
        <StatRow label="Dropped" value={stats.dropped.toLocaleString()} />
        <StatRow label="Overlimits" value={stats.overlimits.toLocaleString()} />
        <StatRow label="Backlog" value={`${formatBytes(stats.backlog_bytes)} / ${stats.backlog_pkts} pkt`} />
        <StatRow label="ECN marks" value={(stats.ecn_mark ?? 0).toLocaleString()} />
        <StatRow label="Drop overlimit" value={(stats.drop_overlimit ?? 0).toLocaleString()} />
        <StatRow label="New flows seen" value={(stats.new_flow_count ?? 0).toLocaleString()} />
        <StatRow label="Active flows" value={`${(stats.new_flows_len ?? 0).toLocaleString()} new / ${(stats.old_flows_len ?? 0).toLocaleString()} old`} />
      </div>
    </div>
  );
}

export function Qos() {
  const qosQ = useQuery({
    queryKey: queryKeys.qos(),
    queryFn: api.qos,
    refetchInterval: 5_000,
  });

  if (qosQ.isError && !qosQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load QoS data — retry shortly.
      </div>
    );
  }
  if (qosQ.isPending || !qosQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed = qosQ.isError;
  const eg = qosQ.data.data.wan_egress;
  const in_ = qosQ.data.data.wan_ingress;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">QoS</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={qosQ.data.stale ?? false}
            updatedAt={qosQ.data.updated_at ?? null}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {eg ? (
          <EgressCard stats={eg} />
        ) : (
          <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant font-mono">
            WAN egress qdisc unavailable.
          </div>
        )}
        {in_ ? (
          <IngressCard stats={in_} />
        ) : (
          <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant font-mono">
            WAN ingress qdisc unavailable.
          </div>
        )}
      </div>
    </div>
  );
}

import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { api } from "@/lib/api";
import type { TopDomain, TopClient, QueryLogEntry } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { formatPercent, formatAbsoluteTime } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";
import { ClientBadge } from "@/components/ClientBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";

/* ------------------------------------------------------------------ */
/*  Stats tiles                                                        */
/* ------------------------------------------------------------------ */

function StatsTiles({
  queries,
  blocked,
  blockRate,
}: {
  queries: number;
  blocked: number;
  blockRate: number;
}) {
  return (
    <div className="grid grid-cols-3 gap-4">
      <div className="bg-surface-container rounded-sm p-4">
        <p className="label-xs mb-1">Queries (24h)</p>
        <MonoText className="text-2xl font-semibold">
          {queries.toLocaleString()}
        </MonoText>
      </div>
      <div className="bg-surface-container rounded-sm p-4">
        <p className="label-xs mb-1">Blocked (24h)</p>
        <MonoText className="text-2xl font-semibold text-rose">
          {blocked.toLocaleString()}
        </MonoText>
      </div>
      <div className="bg-surface-container rounded-sm p-4">
        <p className="label-xs mb-1">Block Rate</p>
        <MonoText className="text-2xl font-semibold text-primary">
          {formatPercent(blockRate)}
        </MonoText>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Query density chart                                                */
/* ------------------------------------------------------------------ */

function DensityChart({
  data,
}: {
  data: { start_hour: number; queries: number; blocked: number }[];
}) {
  // AdGuard's `blocked` count is already included in `queries`. Stacking
  // them as-is produces a total bar that is inflated by `blocked` — so
  // derive `allowed = queries - blocked` and stack allowed + blocked,
  // which sums back to the true query total.
  const chartData = data.map((bin) => ({
    start_hour: bin.start_hour,
    allowed: Math.max(0, bin.queries - bin.blocked),
    blocked: bin.blocked,
  }));
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-3">Query Density (24h)</p>
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={chartData}
            margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
          >
            <XAxis
              dataKey="start_hour"
              tick={{ fill: "hsl(var(--on-surface-variant))", fontSize: 10 }}
              tickLine={false}
              axisLine={false}
            />
            <YAxis hide />
            <Tooltip
              contentStyle={{
                backgroundColor: "hsl(var(--surface-container-high))",
                border: "none",
                borderRadius: 4,
                fontSize: 11,
                color: "hsl(var(--on-surface))",
              }}
              cursor={{ fill: "hsl(var(--surface-container-high) / 0.5)" }}
            />
            <Bar
              dataKey="allowed"
              stackId="a"
              fill="hsl(var(--primary))"
              isAnimationActive={false}
              radius={[0, 0, 0, 0]}
            />
            <Bar
              dataKey="blocked"
              stackId="a"
              fill="hsl(var(--rose))"
              isAnimationActive={false}
              radius={[2, 2, 0, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Top blocked domains table                                          */
/* ------------------------------------------------------------------ */

const blockedColumns: Column<TopDomain>[] = [
  {
    key: "domain",
    label: "Domain",
    render: (r) => <MonoText className="text-xs">{r.domain}</MonoText>,
    sortValue: (r) => r.domain,
  },
  {
    key: "count",
    label: "Count",
    render: (r) => <MonoText className="text-xs">{r.count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.count,
    className: "text-right",
  },
];

/* ------------------------------------------------------------------ */
/*  Top clients table                                                  */
/* ------------------------------------------------------------------ */

const clientColumns: Column<TopClient>[] = [
  {
    key: "ip",
    label: "Client",
    render: (r) => <ClientBadge ip={r.ip} />,
    sortValue: (r) =>
      r.ip
        .split(".")
        .map((o) => o.padStart(3, "0"))
        .join("."),
  },
  {
    key: "count",
    label: "Queries",
    render: (r) => <MonoText className="text-xs">{r.count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.count,
    className: "text-right",
  },
];

/* ------------------------------------------------------------------ */
/*  Query log entry row                                                */
/* ------------------------------------------------------------------ */

function isBlocked(reason: string): boolean {
  return reason.startsWith("Filtered");
}

function LogRow({ entry }: { entry: QueryLogEntry }) {
  const blocked = isBlocked(entry.reason);
  return (
    <div className="flex items-center gap-3 px-3 py-1">
      <MonoText className="text-[10px] text-on-surface-variant shrink-0">
        {formatAbsoluteTime(entry.time)}
      </MonoText>
      <span
        className={`text-[10px] font-bold uppercase tracking-wider shrink-0 ${blocked ? "text-rose" : "text-emerald"}`}
      >
        {blocked ? "BLOCK" : "ALLOW"}
      </span>
      <MonoText className="text-[10px] truncate">
        {entry.question.name}
        <span className="text-on-surface-variant ml-1">({entry.question.type})</span>
      </MonoText>
      <MonoText className="text-[10px] text-on-surface-variant shrink-0 ml-auto">
        {entry.client}
      </MonoText>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Live query log                                                     */
/* ------------------------------------------------------------------ */

function LiveQueryLog() {
  const [clientFilter, setClientFilter] = useState("");

  const params = useMemo(() => {
    const p: { limit: number; client?: string } = { limit: 50 };
    if (clientFilter.trim()) p.client = clientFilter.trim();
    return p;
  }, [clientFilter]);

  const querylogQ = useQuery({
    queryKey: queryKeys.adguardQueryLog(params),
    queryFn: () => {
      const sp = new URLSearchParams();
      sp.set("limit", String(params.limit));
      if (params.client) sp.set("client", params.client);
      return api.adguardQueryLog(sp);
    },
    refetchInterval: 3_000,
  });

  const entries: QueryLogEntry[] =
    querylogQ.data?.data?.queries ?? [];

  return (
    <div className="bg-surface-lowest rounded-sm p-4">
      <div className="flex items-center justify-between mb-3">
        <p className="label-xs">Live query log</p>
        <input
          type="text"
          value={clientFilter}
          onChange={(e) => setClientFilter(e.target.value)}
          placeholder="Filter by client IP..."
          className="bg-surface-container border-none focus:ring-1 focus:ring-primary/30 rounded-sm py-1.5 px-3 text-[10px] text-on-surface placeholder:text-on-surface-variant/50 font-mono w-48"
        />
      </div>
      <div className="max-h-[400px] overflow-y-auto">
        {entries.length === 0 ? (
          <p className="text-xs text-on-surface-variant font-mono px-3 py-2">
            No queries found.
          </p>
        ) : (
          entries.map((entry, i) => <LogRow key={`${entry.time}-${i}`} entry={entry} />)
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Adguard page                                                       */
/* ------------------------------------------------------------------ */

export function Adguard() {
  const statsQ = useQuery({
    queryKey: queryKeys.adguardStats(),
    queryFn: api.adguardStats,
    refetchInterval: 5_000,
  });

  const stats = statsQ.data?.data;
  const density = stats?.query_density_24h ?? [];
  const topBlocked = stats?.top_blocked ?? [];
  const topClients = stats?.top_clients ?? [];

  if (statsQ.isError && !statsQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load AdGuard stats — retry shortly.
      </div>
    );
  }
  if (statsQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed = statsQ.isError;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">AdGuard DNS</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={statsQ.data?.stale ?? false}
            updatedAt={statsQ.data?.updated_at ?? null}
          />
        </div>
      </div>

      {/* Stats tiles */}
      <StatsTiles
        queries={stats?.queries_24h ?? 0}
        blocked={stats?.blocked_24h ?? 0}
        blockRate={stats?.block_rate ?? 0}
      />

      {/* Query density chart */}
      <DensityChart data={density} />

      {/* Side-by-side tables */}
      <div className="grid grid-cols-2 gap-4">
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-3">Top Blocked Domains</p>
          {topBlocked.length === 0 ? (
            <p className="text-xs text-on-surface-variant font-mono">
              No blocked domains.
            </p>
          ) : (
            <DataTable
              columns={blockedColumns}
              rows={topBlocked}
              rowKey={(r) => r.domain}
            />
          )}
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-3">Top Clients</p>
          {topClients.length === 0 ? (
            <p className="text-xs text-on-surface-variant font-mono">
              No client data.
            </p>
          ) : (
            <DataTable
              columns={clientColumns}
              rows={topClients}
              rowKey={(r) => r.ip}
            />
          )}
        </div>
      </div>

      {/* Live query log */}
      <LiveQueryLog />
    </div>
  );
}

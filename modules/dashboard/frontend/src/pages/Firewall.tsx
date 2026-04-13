import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type {
  PortForward,
  PBRSourceRule,
  PBRDomainRule,
  PBRPooledRule,
  FirewallChain,
  RuleCounter,
  UPnPLease,
} from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes } from "@/lib/formatters";

function StatTile({ label, value, tone }: { label: string; value: string; tone?: "failed" | "degraded" | "healthy" | undefined }) {
  const color =
    tone === "failed"
      ? "text-rose"
      : tone === "degraded"
        ? "text-amber"
        : tone === "healthy"
          ? "text-emerald"
          : "";
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className={`text-lg font-semibold ${color}`}>{value}</MonoText>
    </div>
  );
}

const portForwardColumns: Column<PortForward>[] = [
  {
    key: "ext",
    label: "External",
    render: (r) => <MonoText>{r.protocol}/{r.external_port}</MonoText>,
    sortValue: (r) => r.external_port,
  },
  {
    key: "dest",
    label: "Destination",
    render: (r) => <MonoText>{r.destination}</MonoText>,
    sortValue: (r) => r.destination,
  },
];

const sourceRuleColumns: Column<PBRSourceRule>[] = [
  {
    key: "sources",
    label: "Sources",
    render: (r) => <MonoText className="text-xs">{r.sources.join(", ")}</MonoText>,
    sortValue: (r) => r.sources.join(","),
  },
  {
    key: "tunnel",
    label: "Tunnel",
    render: (r) => <MonoText>{r.tunnel}</MonoText>,
    sortValue: (r) => r.tunnel,
  },
];

const domainRuleColumns: Column<PBRDomainRule>[] = [
  {
    key: "tunnel",
    label: "Tunnel",
    render: (r) => <MonoText>{r.tunnel}</MonoText>,
    sortValue: (r) => r.tunnel,
  },
  {
    key: "domains",
    label: "Domains",
    render: (r) => (
      <MonoText className="text-xs">{r.domains.join(", ")}</MonoText>
    ),
    sortValue: (r) => r.domains.join(","),
  },
];

const pooledRuleColumns: Column<PBRPooledRule>[] = [
  {
    key: "pool",
    label: "Pool",
    render: (r) => <MonoText>{r.pool}</MonoText>,
    sortValue: (r) => r.pool,
  },
  {
    key: "sources",
    label: "Sources",
    render: (r) => <MonoText className="text-xs">{r.sources.join(", ")}</MonoText>,
    sortValue: (r) => r.sources.join(","),
  },
];

const upnpColumns: Column<UPnPLease>[] = [
  {
    key: "external",
    label: "External",
    render: (r) => <MonoText>{r.protocol}/{r.external_port}</MonoText>,
    sortValue: (r) => r.external_port,
  },
  {
    key: "internal",
    label: "Internal target",
    render: (r) => <MonoText>{r.internal_addr}:{r.internal_port}</MonoText>,
    sortValue: (r) => r.internal_addr,
  },
  {
    key: "description",
    label: "Description",
    render: (r) =>
      r.description || <span className="text-on-surface-variant">—</span>,
    sortValue: (r) => r.description ?? "",
  },
];

const counterColumns: Column<RuleCounter>[] = [
  {
    key: "handle",
    label: "Rule",
    render: (r) => <MonoText className="text-xs">#{r.handle}</MonoText>,
    sortValue: (r) => r.handle,
  },
  {
    key: "comment",
    label: "Comment",
    render: (r) =>
      r.comment ? (
        <MonoText className="text-xs">{r.comment}</MonoText>
      ) : (
        <span className="text-on-surface-variant">—</span>
      ),
    sortValue: (r) => r.comment ?? "",
  },
  {
    key: "packets",
    label: "Packets",
    render: (r) => <MonoText>{r.packets.toLocaleString()}</MonoText>,
    sortValue: (r) => r.packets,
    className: "text-right",
  },
  {
    key: "bytes",
    label: "Bytes",
    render: (r) => <MonoText>{formatBytes(r.bytes)}</MonoText>,
    sortValue: (r) => r.bytes,
    className: "text-right",
  },
];

function ChainCountersCard({ chain }: { chain: FirewallChain }) {
  const rows = [...chain.counters].sort((a, b) => b.bytes - a.bytes);
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">
          <MonoText>{chain.family}/{chain.table}/{chain.name}</MonoText>
        </h3>
        <div className="flex items-center gap-2">
          {chain.hook && (
            <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
              {chain.hook}
            </span>
          )}
          {chain.policy && (
            <StatusBadge kind={chain.policy === "drop" ? "failed" : "info"}>
              {chain.policy}
            </StatusBadge>
          )}
        </div>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono">
          No counter rules in this chain.
        </p>
      ) : (
        <DataTable
          columns={counterColumns}
          rows={rows}
          rowKey={(r) => String(r.handle)}
        />
      )}
    </div>
  );
}

export function Firewall() {
  const rulesQ = useQuery({
    queryKey: queryKeys.firewallRules(),
    queryFn: api.firewallRules,
    refetchInterval: 30_000,
  });
  const countersQ = useQuery({
    queryKey: queryKeys.firewallCounters(),
    queryFn: api.firewallCounters,
    refetchInterval: 5_000,
  });
  const upnpQ = useQuery({
    queryKey: queryKeys.upnp(),
    queryFn: api.upnp,
    refetchInterval: 15_000,
  });

  const noData = !rulesQ.data || !countersQ.data || !upnpQ.data;
  if ((rulesQ.isError || countersQ.isError || upnpQ.isError) && noData) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load firewall data — retry shortly.
      </div>
    );
  }
  if (rulesQ.isPending || countersQ.isPending || upnpQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }
  if (!rulesQ.data || !countersQ.data || !upnpQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed =
    rulesQ.isError || countersQ.isError || upnpQ.isError;
  const rules = rulesQ.data.data;
  const chains = countersQ.data.data.chains;
  const leases = upnpQ.data.data.leases;

  const combinedStale =
    (rulesQ.data.stale ?? false) ||
    (countersQ.data.stale ?? false) ||
    (upnpQ.data.stale ?? false);
  const updatedAts = [
    rulesQ.data.updated_at,
    countersQ.data.updated_at,
    upnpQ.data.updated_at,
  ].filter((u): u is string => !!u);
  const combinedUpdatedAt =
    updatedAts.length === 0
      ? null
      : updatedAts.reduce((a, b) => (a < b ? a : b));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Firewall &amp; UPnP</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatTile label="Port forwards" value={String(rules.port_forwards.length)} />
        <StatTile label="PBR rules" value={String(rules.pbr.source_rules.length + rules.pbr.domain_rules.length + rules.pbr.pooled_rules.length)} />
        <StatTile label="Allowed MACs" value={String(rules.allowed_macs.length)} />
        <StatTile
          label="Blocked forwards (1h)"
          value={rules.blocked_forward_count_1h.toLocaleString()}
          tone={rules.blocked_forward_count_1h > 0 ? "degraded" : undefined}
        />
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Port forwards</h2>
        {rules.port_forwards.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No port forwards configured.
          </p>
        ) : (
          <DataTable
            columns={portForwardColumns}
            rows={rules.port_forwards}
            rowKey={(r) => `${r.protocol}/${r.external_port}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — source rules</h2>
        {rules.pbr.source_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No source-based PBR rules.
          </p>
        ) : (
          <DataTable
            columns={sourceRuleColumns}
            rows={rules.pbr.source_rules}
            rowKey={(r) => `${r.tunnel}|${r.sources.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — domain rules</h2>
        {rules.pbr.domain_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No domain-based PBR rules.
          </p>
        ) : (
          <DataTable
            columns={domainRuleColumns}
            rows={rules.pbr.domain_rules}
            rowKey={(r) => `${r.tunnel}|${r.domains.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — pooled rules</h2>
        {rules.pbr.pooled_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No pooled PBR rules.
          </p>
        ) : (
          <DataTable
            columns={pooledRuleColumns}
            rows={rules.pbr.pooled_rules}
            rowKey={(r) => `${r.pool}|${r.sources.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Allowlisted MACs ({rules.allowed_macs.length})</h2>
        {rules.allowed_macs.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            Allowlist is empty or disabled.
          </p>
        ) : (
          <div className="bg-surface-container rounded-sm p-4 flex flex-wrap gap-2">
            {rules.allowed_macs.map((m) => (
              <MonoText key={m} className="text-xs bg-surface-high px-2 py-1 rounded-sm">
                {m}
              </MonoText>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-3">
        <h2 className="label-xs">Counters ({chains.length} chains)</h2>
        {chains.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No chains reported.
          </p>
        ) : (
          chains
            .slice()
            .sort((a, b) => (a.family + a.table + a.name).localeCompare(b.family + b.table + b.name))
            .map((c) => (
              <ChainCountersCard key={`${c.family}/${c.table}/${c.name}`} chain={c} />
            ))
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">UPnP leases ({leases.length})</h2>
        {leases.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No active UPnP leases.
          </p>
        ) : (
          <DataTable
            columns={upnpColumns}
            rows={leases}
            rowKey={(r) => `${r.protocol}/${r.external_port}`}
          />
        )}
      </div>
    </div>
  );
}

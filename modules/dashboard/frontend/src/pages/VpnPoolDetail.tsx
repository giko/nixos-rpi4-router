import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Pool, Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { ClientBadge } from "@/components/ClientBadge";
import { DataTable, type Column } from "@/components/DataTable";

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className="text-lg font-semibold">{value}</MonoText>
    </div>
  );
}

type RoutedClient = {
  hostname: string;
  ip: string;
  conn_count: number;
};

// ScopedMember is the pool-page variant of PoolMember where `conn_count`
// is summed from client.tunnel_conns restricted to this pool's clients.
// Using pool.members[].flow_count directly would include any non-pool
// traffic on the tunnel (e.g. a shared source-rule or domain-rule).
type ScopedMember = {
  tunnel: string;
  fwmark: string;
  healthy: boolean;
  conn_count: number;
};

const memberColumns: Column<ScopedMember>[] = [
  {
    key: "tunnel",
    label: "Tunnel",
    render: (r) => <MonoText>{r.tunnel}</MonoText>,
    sortValue: (r) => r.tunnel,
  },
  {
    key: "fwmark",
    label: "Fwmark",
    render: (r) => <MonoText>{r.fwmark}</MonoText>,
    sortValue: (r) => r.fwmark,
  },
  {
    key: "health",
    label: "Health",
    render: (r) => (
      <StatusBadge kind={r.healthy ? "healthy" : "failed"}>
        {r.healthy ? "healthy" : "down"}
      </StatusBadge>
    ),
    sortValue: (r) => (r.healthy ? 0 : 1),
  },
  {
    key: "conns",
    label: "Connections",
    render: (r) => <MonoText>{r.conn_count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.conn_count,
    className: "text-right",
  },
];

const clientColumns: Column<RoutedClient>[] = [
  {
    key: "hostname",
    label: "Hostname",
    render: (r) => r.hostname || <span className="text-on-surface-variant">--</span>,
    sortValue: (r) => r.hostname.toLowerCase(),
  },
  {
    key: "ip",
    label: "IP",
    render: (r) => <ClientBadge ip={r.ip} />,
    sortValue: (r) => r.ip,
  },
  {
    key: "conns",
    label: "Connections",
    render: (r) => <MonoText>{r.conn_count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.conn_count,
    className: "text-right",
  },
];

export function VpnPoolDetail() {
  const { name } = useParams<{ name: string }>();

  const poolsQ = useQuery({
    queryKey: queryKeys.pools(),
    queryFn: api.pools,
    refetchInterval: 5_000,
  });

  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 5_000,
  });

  // Loading guard: avoid 404 flash on cold navigation
  if (poolsQ.isPending || !poolsQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const pool: Pool | undefined = poolsQ.data.data.pools.find(
    (p) => p.name === name,
  );

  if (!pool) {
    return (
      <div className="space-y-4">
        <Link
          to="/vpn/pools"
          className="text-sm text-primary hover:underline"
        >
          &larr; Back to pools
        </Link>
        <p className="text-sm text-on-surface-variant">Pool not found.</p>
      </div>
    );
  }

  const healthy = pool.members.filter((m) => m.healthy).length;

  const allClients: Client[] = clientsQ.data?.data.clients ?? [];
  const poolClientIps = new Set(pool.client_ips);
  // Filter once, then derive every pool-scoped count from the SAME source
  // (client.tunnel_conns restricted to this pool's fwmarks). This keeps
  // per-client totals, per-member totals, and the "Total Connections" tile
  // arithmetically consistent even when a tunnel is shared with a non-pool
  // source or domain rule.
  const poolClients = allClients.filter((c) => poolClientIps.has(c.ip));

  // Per-member: sum across this pool's clients of tunnel_conns[member.fwmark].
  const scopedMembers: ScopedMember[] = pool.members.map((m) => {
    const count = poolClients.reduce(
      (sum, c) => sum + ((c.tunnel_conns ?? {})[m.fwmark] ?? 0),
      0,
    );
    return {
      tunnel: m.tunnel,
      fwmark: m.fwmark,
      healthy: m.healthy,
      conn_count: count,
    };
  });

  // Per-client: sum across this pool's fwmarks of client.tunnel_conns[mark].
  const poolFwmarks = pool.members.map((m) => m.fwmark);
  const routedClients: RoutedClient[] = poolClients.map((c) => {
    const tunnelConns = c.tunnel_conns ?? {};
    const poolScopedConns = poolFwmarks.reduce(
      (sum, mark) => sum + (tunnelConns[mark] ?? 0),
      0,
    );
    return {
      hostname: c.hostname,
      ip: c.ip,
      conn_count: poolScopedConns,
    };
  });

  // Both aggregations use the same data so per-member total == per-client total.
  const totalConns = scopedMembers.reduce((s, m) => s + m.conn_count, 0);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/vpn/pools"
            className="text-sm text-primary hover:underline"
          >
            &larr; Pools
          </Link>
          <h1 className="text-lg font-semibold">{pool.name}</h1>
        </div>
        <StaleIndicator
          stale={poolsQ.data.stale}
          updatedAt={poolsQ.data.updated_at}
        />
      </div>

      {/* Stat tiles */}
      <div className="grid grid-cols-3 gap-4">
        <StatTile label="Total Connections" value={totalConns.toLocaleString()} />
        <StatTile
          label="Healthy Members"
          value={`${healthy} / ${pool.members.length}`}
        />
        <StatTile
          label="Routed Clients"
          value={String(pool.client_ips.length)}
        />
      </div>

      {/* Member distribution */}
      <div className="space-y-2">
        <h2 className="label-xs">Member Distribution</h2>
        <DataTable
          columns={memberColumns}
          rows={scopedMembers}
          rowKey={(r) => r.tunnel}
        />
      </div>

      {/* Routed clients */}
      <div className="space-y-2">
        <h2 className="label-xs">Routed Clients</h2>
        {routedClients.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            {clientsQ.isPending ? "Loading clients..." : "No routed clients."}
          </p>
        ) : (
          <DataTable
            columns={clientColumns}
            rows={routedClients}
            rowKey={(r) => r.ip}
          />
        )}
      </div>
    </div>
  );
}

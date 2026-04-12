import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Pool, PoolMember, Client } from "@/lib/api";
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

const memberColumns: Column<PoolMember>[] = [
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
    render: (r) => <MonoText>{r.flow_count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.flow_count,
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
  const totalConns = pool.members.reduce((s, m) => s + m.flow_count, 0);

  const allClients: Client[] = clientsQ.data?.data.clients ?? [];
  const poolClientIps = new Set(pool.client_ips);
  const routedClients: RoutedClient[] = allClients
    .filter((c) => poolClientIps.has(c.ip))
    .map((c) => ({
      hostname: c.hostname,
      ip: c.ip,
      conn_count: c.conn_count,
    }));

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
          rows={pool.members}
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

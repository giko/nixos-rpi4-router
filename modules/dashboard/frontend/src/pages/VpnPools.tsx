import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Pool, Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";

// poolScopedConns sums client.tunnel_conns for every client assigned to
// this pool, restricted to the pool's tunnel fwmarks. Kept in sync with
// the pool detail page so both surfaces report the same totals even if
// a tunnel is shared with non-pool routing rules.
function poolScopedConns(pool: Pool, clients: Client[]): number {
  const clientIps = new Set(pool.client_ips);
  const fwmarks = pool.members.map((m) => m.fwmark);
  let total = 0;
  for (const c of clients) {
    if (!clientIps.has(c.ip)) continue;
    const tc = c.tunnel_conns ?? {};
    for (const mark of fwmarks) total += tc[mark] ?? 0;
  }
  return total;
}

function PoolCard({ pool, clients }: { pool: Pool; clients: Client[] }) {
  const healthy = pool.members.filter((m) => m.healthy).length;
  const total = pool.members.length;
  const totalConns = poolScopedConns(pool, clients);
  const allHealthy = healthy === total && total > 0;
  const noneHealthy = healthy === 0 && total > 0;

  return (
    <Link
      to={`/vpn/pools/${encodeURIComponent(pool.name)}`}
      className="block bg-surface-container rounded-sm p-4 hover:bg-surface-high transition-colors"
    >
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold">{pool.name}</h2>
        <StatusBadge
          kind={noneHealthy ? "failed" : allHealthy ? "healthy" : "degraded"}
        >
          {healthy} / {total} healthy
        </StatusBadge>
      </div>

      <div className="flex flex-wrap gap-1.5 mb-3">
        {pool.members.map((m) => (
          <StatusBadge key={m.tunnel} kind={m.healthy ? "healthy" : "failed"}>
            {m.tunnel}
          </StatusBadge>
        ))}
      </div>

      <div className="flex gap-4 text-xs text-on-surface-variant">
        <span>
          Connections:{" "}
          <MonoText>{totalConns.toLocaleString()}</MonoText>
        </span>
        <span>
          Clients:{" "}
          <MonoText>{pool.client_ips.length}</MonoText>
        </span>
      </div>
    </Link>
  );
}

export function VpnPools() {
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

  // Wait for BOTH queries — the cards' connection counts are derived from
  // client.tunnel_conns, so rendering before clients resolve would flash
  // zeroes even on busy pools. The clients query also surfaces errors
  // (e.g. /api/clients unavailable) that would otherwise silently
  // degrade every count to zero.
  if (poolsQ.isPending || clientsQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }
  if (clientsQ.isError) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load client data — pool counts unavailable.
      </div>
    );
  }

  const pools = poolsQ.data?.data.pools ?? [];
  const clients = clientsQ.data?.data.clients ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">VPN Pools</h1>
        <StaleIndicator
          stale={poolsQ.data?.stale ?? false}
          updatedAt={poolsQ.data?.updated_at ?? null}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {pools.map((pool) => (
          <PoolCard key={pool.name} pool={pool} clients={clients} />
        ))}
        {pools.length === 0 && (
          <p className="text-sm text-on-surface-variant font-mono col-span-full">
            No pools configured.
          </p>
        )}
      </div>
    </div>
  );
}

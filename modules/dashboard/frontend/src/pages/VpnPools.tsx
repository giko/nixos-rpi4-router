import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Pool } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";

function PoolCard({ pool }: { pool: Pool }) {
  const healthy = pool.members.filter((m) => m.healthy).length;
  const total = pool.members.length;
  const totalFlows = pool.members.reduce((s, m) => s + m.flow_count, 0);
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
          <MonoText>{totalFlows.toLocaleString()}</MonoText>
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

  if (poolsQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const pools = poolsQ.data?.data.pools ?? [];

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
          <PoolCard key={pool.name} pool={pool} />
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

import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Tunnel, Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatDuration } from "@/lib/formatters";

type Row = {
  name: string;
  fwmark: string;
  endpoint: string;
  healthy: boolean;
  handshakeAgo: number;
  rxBytes: number;
  txBytes: number;
  clientCount: number;
};

const columns: Column<Row>[] = [
  {
    key: "name",
    label: "Tunnel",
    render: (r) => (
      <Link
        to={`/vpn/tunnels/${encodeURIComponent(r.name)}`}
        className="text-primary hover:underline"
      >
        <MonoText>{r.name}</MonoText>
      </Link>
    ),
    sortValue: (r) => r.name,
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
    key: "fwmark",
    label: "Fwmark",
    render: (r) => <MonoText>{r.fwmark}</MonoText>,
    sortValue: (r) => r.fwmark,
  },
  {
    key: "endpoint",
    label: "Endpoint",
    render: (r) => <MonoText className="text-xs">{r.endpoint || "—"}</MonoText>,
    sortValue: (r) => r.endpoint,
  },
  {
    key: "handshake",
    label: "Last handshake",
    render: (r) =>
      r.handshakeAgo > 0 ? (
        <MonoText>{formatDuration(r.handshakeAgo)} ago</MonoText>
      ) : (
        <span className="text-on-surface-variant">never</span>
      ),
    sortValue: (r) => r.handshakeAgo,
  },
  {
    key: "rx",
    label: "RX",
    render: (r) => <MonoText>{formatBytes(r.rxBytes)}</MonoText>,
    sortValue: (r) => r.rxBytes,
    className: "text-right",
  },
  {
    key: "tx",
    label: "TX",
    render: (r) => <MonoText>{formatBytes(r.txBytes)}</MonoText>,
    sortValue: (r) => r.txBytes,
    className: "text-right",
  },
  {
    key: "clients",
    label: "Routed clients",
    render: (r) => <MonoText>{r.clientCount}</MonoText>,
    sortValue: (r) => r.clientCount,
    className: "text-right",
  },
];

function buildRows(tunnels: Tunnel[], clients: Client[]): Row[] {
  return tunnels.map((t) => {
    let clientCount = 0;
    for (const c of clients) {
      const tc = c.tunnel_conns ?? {};
      if ((tc[t.fwmark] ?? 0) > 0) clientCount++;
    }
    return {
      name: t.name,
      fwmark: t.fwmark,
      endpoint: t.endpoint,
      healthy: t.healthy,
      handshakeAgo: t.latest_handshake_seconds_ago,
      rxBytes: t.rx_bytes,
      txBytes: t.tx_bytes,
      clientCount,
    };
  });
}

export function VpnTunnels() {
  const tunnelsQ = useQuery({
    queryKey: queryKeys.tunnels(),
    queryFn: api.tunnels,
    refetchInterval: 5_000,
  });
  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 5_000,
  });

  const noData = !tunnelsQ.data || !clientsQ.data;
  if ((tunnelsQ.isError || clientsQ.isError) && noData) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load tunnel data — retry shortly.
      </div>
    );
  }
  if (tunnelsQ.isPending || clientsQ.isPending || !tunnelsQ.data || !clientsQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed = tunnelsQ.isError || clientsQ.isError;
  const tunnels = tunnelsQ.data.data.tunnels;
  const clients = clientsQ.data.data.clients;
  const rows = buildRows(tunnels, clients);

  const combinedStale =
    (tunnelsQ.data.stale ?? false) || (clientsQ.data.stale ?? false);
  const tunnelsUpdated = tunnelsQ.data.updated_at ?? null;
  const clientsUpdated = clientsQ.data.updated_at ?? null;
  const combinedUpdatedAt =
    tunnelsUpdated && clientsUpdated
      ? tunnelsUpdated < clientsUpdated
        ? tunnelsUpdated
        : clientsUpdated
      : (tunnelsUpdated ?? clientsUpdated);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">VPN Tunnels</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-on-surface-variant font-mono">
          No tunnels configured.
        </p>
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(r) => r.name} />
      )}
    </div>
  );
}

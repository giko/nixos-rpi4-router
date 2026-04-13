import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Tunnel, Client, Interface } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { ClientBadge } from "@/components/ClientBadge";
import { DataTable, type Column } from "@/components/DataTable";
import { Sparkline } from "@/components/Sparkline";
import { formatBytes, formatBps, formatDuration } from "@/lib/formatters";

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className="text-lg font-semibold">{value}</MonoText>
    </div>
  );
}

type ClientRow = {
  hostname: string;
  ip: string;
  conn_count: number;
};

const clientColumns: Column<ClientRow>[] = [
  {
    key: "hostname",
    label: "Hostname",
    render: (r) =>
      r.hostname || <span className="text-on-surface-variant">—</span>,
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

export function VpnTunnelDetail() {
  const { name } = useParams<{ name: string }>();
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
  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 2_000,
  });

  const noData =
    !tunnelsQ.data || !clientsQ.data || !trafficQ.data;
  if (
    (tunnelsQ.isError || clientsQ.isError || trafficQ.isError) &&
    noData
  ) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load tunnel data — retry shortly.
      </div>
    );
  }
  if (tunnelsQ.isPending || clientsQ.isPending || trafficQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }
  if (!tunnelsQ.data || !clientsQ.data || !trafficQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const tunnel: Tunnel | undefined = tunnelsQ.data.data.tunnels.find(
    (t) => t.name === name,
  );
  if (!tunnel) {
    return (
      <div className="space-y-4">
        <Link
          to="/vpn/tunnels"
          className="text-sm text-primary hover:underline"
        >
          &larr; Back to tunnels
        </Link>
        <p className="text-sm text-on-surface-variant">Tunnel not found.</p>
      </div>
    );
  }

  const refetchFailed =
    tunnelsQ.isError || clientsQ.isError || trafficQ.isError;
  const allClients: Client[] = clientsQ.data.data.clients;
  const routedClients: ClientRow[] = allClients
    .map((c) => {
      const conns = (c.tunnel_conns ?? {})[tunnel.fwmark] ?? 0;
      return { hostname: c.hostname, ip: c.ip, conn_count: conns };
    })
    .filter((r) => r.conn_count > 0);

  const totalConns = routedClients.reduce((s, r) => s + r.conn_count, 0);

  const iface: Interface | undefined = trafficQ.data.data.interfaces.find(
    (i) => i.name === tunnel.interface,
  );
  const rxSeries = iface?.samples_60s.map((s) => s.rx_bps) ?? [];
  const txSeries = iface?.samples_60s.map((s) => s.tx_bps) ?? [];

  const combinedStale =
    (tunnelsQ.data.stale ?? false) ||
    (clientsQ.data.stale ?? false) ||
    (trafficQ.data.stale ?? false);
  const updatedAts = [
    tunnelsQ.data.updated_at,
    clientsQ.data.updated_at,
    trafficQ.data.updated_at,
  ].filter((u): u is string => !!u);
  const combinedUpdatedAt =
    updatedAts.length === 0
      ? null
      : updatedAts.reduce((a, b) => (a < b ? a : b));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/vpn/tunnels"
            className="text-sm text-primary hover:underline"
          >
            &larr; Tunnels
          </Link>
          <h1 className="text-lg font-semibold">{tunnel.name}</h1>
          <StatusBadge kind={tunnel.healthy ? "healthy" : "failed"}>
            {tunnel.healthy ? "healthy" : "down"}
          </StatusBadge>
        </div>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <StatTile
          label="Total Connections"
          value={totalConns.toLocaleString()}
        />
        <StatTile
          label="RX / TX"
          value={`${formatBytes(tunnel.rx_bytes)} / ${formatBytes(tunnel.tx_bytes)}`}
        />
        <StatTile
          label="Last Handshake"
          value={
            tunnel.latest_handshake_seconds_ago > 0
              ? `${formatDuration(tunnel.latest_handshake_seconds_ago)} ago`
              : "never"
          }
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-2">Endpoint</p>
          <MonoText className="text-sm">{tunnel.endpoint || "—"}</MonoText>
          <p className="label-xs mt-3 mb-1">Public key</p>
          <MonoText className="text-xs break-all text-on-surface-variant">
            {tunnel.public_key}
          </MonoText>
          <p className="label-xs mt-3 mb-1">Fwmark · Routing table</p>
          <MonoText className="text-sm">
            {tunnel.fwmark} · {tunnel.routing_table}
          </MonoText>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-2">RX bps (60s)</p>
          <Sparkline data={rxSeries} className="h-12" />
          <p className="text-xs text-on-surface-variant mt-1">
            now {formatBps(iface?.rx_bps ?? 0)}
          </p>
          <p className="label-xs mt-4 mb-2">TX bps (60s)</p>
          <Sparkline data={txSeries} className="h-12" />
          <p className="text-xs text-on-surface-variant mt-1">
            now {formatBps(iface?.tx_bps ?? 0)}
          </p>
        </div>
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Routed Clients</h2>
        {routedClients.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No clients are routing through this tunnel right now.
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

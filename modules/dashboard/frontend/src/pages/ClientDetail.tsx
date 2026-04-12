import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { PoolBadge } from "@/components/PoolBadge";
import { StaleIndicator } from "@/components/StaleIndicator";

/* ------------------------------------------------------------------ */
/*  Route rendering                                                    */
/* ------------------------------------------------------------------ */

function RouteValue({ route }: { route: string }) {
  if (route.startsWith("pool:")) {
    return <PoolBadge name={route.slice(5)} />;
  }
  if (route.startsWith("tunnel:")) {
    return <StatusBadge kind="info">{route.slice(7)}</StatusBadge>;
  }
  if (route === "wan") {
    return <StatusBadge kind="muted">WAN DIRECT</StatusBadge>;
  }
  return <MonoText>{route || "--"}</MonoText>;
}

/* ------------------------------------------------------------------ */
/*  Allowlist status badge                                             */
/* ------------------------------------------------------------------ */

function AllowlistBadge({ status }: { status: string }) {
  const kind =
    status === "allowed"
      ? "healthy"
      : status === "blocked"
        ? "failed"
        : "degraded";
  return <StatusBadge kind={kind}>{status || "unknown"}</StatusBadge>;
}

/* ------------------------------------------------------------------ */
/*  Lease type badge                                                   */
/* ------------------------------------------------------------------ */

function LeaseTypeBadge({ type }: { type: string }) {
  const kind =
    type === "static"
      ? "info"
      : type === "neighbor"
        ? "degraded"
        : "muted";
  return <StatusBadge kind={kind}>{type || "unknown"}</StatusBadge>;
}

/* ------------------------------------------------------------------ */
/*  Detail row                                                         */
/* ------------------------------------------------------------------ */

function DetailRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between py-2 border-b border-outline-variant/10 last:border-b-0">
      <span className="label-xs">{label}</span>
      <div>{children}</div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Client detail page                                                 */
/* ------------------------------------------------------------------ */

export function ClientDetail() {
  const { ip } = useParams<{ ip: string }>();

  const clientQ = useQuery({
    queryKey: queryKeys.client(ip ?? ""),
    queryFn: () => api.client(ip ?? ""),
    enabled: !!ip,
    refetchInterval: 3_000,
    retry: (count, error) => {
      if (error instanceof Error && error.message.includes("404")) return false;
      return count < 3;
    },
  });

  /* Loading */
  if (clientQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  /* Not found */
  if (clientQ.isError || !clientQ.data) {
    return (
      <div className="space-y-4">
        <Link
          to="/clients"
          className="text-sm text-primary hover:underline"
        >
          &larr; Back to clients
        </Link>
        <p className="text-sm text-on-surface-variant">Client not found.</p>
      </div>
    );
  }

  const client = clientQ.data.data;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/clients"
            className="text-sm text-primary hover:underline"
          >
            &larr; Clients
          </Link>
          <h1 className="text-lg font-semibold">
            {client.hostname || client.ip}
          </h1>
        </div>
        <StaleIndicator
          stale={clientQ.data.stale}
          updatedAt={clientQ.data.updated_at}
        />
      </div>

      {/* Two-column detail grid */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
        {/* Identity */}
        <div className="lg:col-span-6 bg-surface-container rounded-sm p-4">
          <h2 className="label-xs mb-3">Identity</h2>
          <DetailRow label="IP Address">
            <MonoText>{client.ip}</MonoText>
          </DetailRow>
          <DetailRow label="MAC Address">
            <MonoText className="text-on-surface-variant">
              {client.mac || "--"}
            </MonoText>
          </DetailRow>
          <DetailRow label="Lease Type">
            <LeaseTypeBadge type={client.lease_type} />
          </DetailRow>
          <DetailRow label="Allowlist">
            <AllowlistBadge status={client.allowlist_status} />
          </DetailRow>
        </div>

        {/* Routing */}
        <div className="lg:col-span-6 bg-surface-container rounded-sm p-4">
          <h2 className="label-xs mb-3">Routing</h2>
          <DetailRow label="Route">
            <RouteValue route={client.route} />
          </DetailRow>
          <DetailRow label="Current Tunnel">
            <MonoText>{client.current_tunnel || "--"}</MonoText>
          </DetailRow>
          <DetailRow label="Flows">
            <MonoText>{client.flow_count.toLocaleString()}</MonoText>
          </DetailRow>
        </div>
      </div>
    </div>
  );
}

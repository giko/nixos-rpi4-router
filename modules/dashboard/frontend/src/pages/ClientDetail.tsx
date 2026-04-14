import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { PoolBadge } from "@/components/PoolBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { ClientDetailFilterProvider } from "@/components/client-detail/useClientDetailFilter";
import { ActivitySparklines } from "@/components/client-detail/ActivitySparklines";
import { TrafficGraph } from "@/components/client-detail/TrafficGraph";
import { TopDestinations } from "@/components/client-detail/TopDestinations";
import { DnsQueries } from "@/components/client-detail/DnsQueries";
import { OpenConnections } from "@/components/client-detail/OpenConnections";

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

  /* Cold-failure guard: distinguish a real 404 ("no such client") from
   * transient fetch failures (5xx, network drops) so users don't see a
   * misleading "not found" during backend hiccups. fetchEnvelope throws
   * Error with "<path> → <status>" — match on 404 in the message. Only
   * replace the whole view when there is NO cached data yet; refetch
   * failures after a successful first load fall through to the main
   * render with a small "Refetch failed" chip in the header. */
  if (clientQ.isError && !clientQ.data) {
    const msg =
      clientQ.error instanceof Error ? clientQ.error.message : "";
    const notFound = msg.includes("404");
    return (
      <div className="space-y-4">
        <Link
          to="/clients"
          className="text-sm text-primary hover:underline"
        >
          &larr; Back to clients
        </Link>
        <p className="text-sm text-on-surface-variant">
          {notFound
            ? "Client not found."
            : "Failed to load client — retry shortly."}
        </p>
      </div>
    );
  }

  /* Loading */
  if (clientQ.isPending || !clientQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const client = clientQ.data.data;
  const refetchFailed = clientQ.isError;

  return (
    <ClientDetailFilterProvider>
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
          <div className="flex items-center gap-2">
            {refetchFailed && (
              <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
                Refetch failed
              </span>
            )}
            <StaleIndicator
              stale={clientQ.data.stale}
              updatedAt={clientQ.data.updated_at}
            />
          </div>
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
            <DetailRow label="Connections">
              <MonoText>{client.conn_count.toLocaleString()}</MonoText>
            </DetailRow>
          </div>
        </div>

        {/* Activity overview (highest-density at-a-glance) */}
        <ActivitySparklines ip={client.ip} />

        {/* Traffic graph — full width */}
        <TrafficGraph ip={client.ip} />

        {/* Top destinations + DNS queries — side-by-side on wide screens */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <TopDestinations ip={client.ip} />
          <DnsQueries ip={client.ip} />
        </div>

        {/* Open connections — last, full width (longest table) */}
        <OpenConnections ip={client.ip} />
      </div>
    </ClientDetailFilterProvider>
  );
}

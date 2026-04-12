export type Envelope<T> = {
  data: T;
  updated_at: string | null;
  stale: boolean;
};

// Type definitions for every API response shape:
export type Health = { ok: boolean; version: string; started_at: string };
export type CPU = {
  percent_user: number;
  percent_system: number;
  percent_idle: number;
  percent_iowait: number;
};
export type Memory = {
  total_bytes: number;
  available_bytes: number;
  used_bytes: number;
  percent_used: number;
};
export type Service = { name: string; active: boolean; raw_state: string };
export type System = {
  cpu: CPU;
  memory: Memory;
  temperature_c: number;
  throttled: string;
  throttled_flag: boolean;
  uptime_seconds: number;
  boot_time: string;
  services: Service[] | null;
};
export type InterfaceSample = { rx_bps: number; tx_bps: number };
export type Interface = {
  name: string;
  operstate: string; // "up" | "down" | "unknown"
  rx_bps: number;
  tx_bps: number;
  rx_bytes_total: number;
  tx_bytes_total: number;
  samples_60s: InterfaceSample[];
};
export type Traffic = { interfaces: Interface[] };
export type Tunnel = {
  name: string;
  interface: string;
  fwmark: string;
  routing_table: number;
  public_key: string;
  endpoint: string;
  latest_handshake_seconds_ago: number;
  rx_bytes: number;
  tx_bytes: number;
  healthy: boolean;
  consecutive_failures: number;
};
export type PoolMember = {
  tunnel: string;
  fwmark: string;
  healthy: boolean;
  flow_count: number;
};
export type Pool = {
  name: string;
  members: PoolMember[];
  client_ips: string[];
  failsafe_drop_active: boolean;
};
export type Client = {
  hostname: string;
  ip: string;
  mac: string;
  lease_type: string;
  last_seen: string;
  route: string;
  allowlist_status: string;
  conn_count: number;
  dns_queries_1h: number;
};
export type TopDomain = { domain: string; count: number };
export type TopClient = { ip: string; count: number };
export type DensityBin = {
  start_hour: number;
  queries: number;
  blocked: number;
};
export type AdguardStats = {
  queries_24h: number;
  blocked_24h: number;
  block_rate: number;
  top_blocked: TopDomain[] | null;
  top_clients: TopClient[] | null;
  query_density_24h: DensityBin[] | null;
};
export type QueryLogEntry = {
  time: string;
  question: { class: string; name: string; type: string };
  client: string;
  upstream: string;
  reason: string;
  elapsedMs: string;
};

async function fetchEnvelope<T>(path: string): Promise<Envelope<T>> {
  const res = await fetch(path);
  if (!res.ok) throw new Error(`${path} → ${res.status}`);
  return (await res.json()) as Envelope<T>;
}

// /api/health returns bare JSON, not an envelope
async function fetchHealthRaw(): Promise<Health> {
  const res = await fetch("/api/health");
  if (!res.ok) throw new Error(`/api/health → ${res.status}`);
  return (await res.json()) as Health;
}

export const api = {
  health: fetchHealthRaw,
  system: () => fetchEnvelope<System>("/api/system"),
  traffic: () => fetchEnvelope<Traffic>("/api/traffic"),
  tunnels: () => fetchEnvelope<{ tunnels: Tunnel[] }>("/api/tunnels"),
  pools: () => fetchEnvelope<{ pools: Pool[] }>("/api/pools"),
  clients: () => fetchEnvelope<{ clients: Client[] }>("/api/clients"),
  client: (ip: string) =>
    fetchEnvelope<Client>(`/api/clients/${encodeURIComponent(ip)}`),
  adguardStats: () => fetchEnvelope<AdguardStats>("/api/adguard/stats"),
  adguardQueryLog: (params: URLSearchParams) =>
    fetchEnvelope<{ queries: QueryLogEntry[] }>(
      "/api/adguard/querylog?" + params.toString(),
    ),
};

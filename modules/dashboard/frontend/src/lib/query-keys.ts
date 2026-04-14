export const queryKeys = {
  health: () => ["health"] as const,
  system: () => ["system"] as const,
  traffic: () => ["traffic"] as const,
  tunnels: () => ["tunnels"] as const,
  pools: () => ["pools"] as const,
  pool: (name: string) => ["pools", name] as const,
  clients: () => ["clients"] as const,
  client: (ip: string) => ["clients", ip] as const,
  clientTraffic: (ip: string) => ["clients", ip, "traffic"] as const,
  clientDns: (ip: string) => ["clients", ip, "dns"] as const,
  clientConnections: (ip: string) => ["clients", ip, "connections"] as const,
  clientTopDestinations: (ip: string) =>
    ["clients", ip, "top-destinations"] as const,
  clientSparklines: (ip: string) => ["clients", ip, "sparklines"] as const,
  adguardStats: () => ["adguard", "stats"] as const,
  adguardQueryLog: (filters: {
    limit: number;
    client?: string;
    domain?: string;
  }) => ["adguard", "querylog", filters] as const,
  firewallRules: () => ["firewall", "rules"] as const,
  firewallCounters: () => ["firewall", "counters"] as const,
  upnp: () => ["upnp"] as const,
  qos: () => ["qos"] as const,
};

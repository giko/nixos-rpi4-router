{ config, lib, pkgs, ... }:

let
  cfg = config.router.dashboard;

  routerLib = import ../lib.nix { inherit lib; };
  tunnelMeta = routerLib.tunnelMeta config.router.wireguard.tunnels;

  # Guard allowedMacs — it's declared nullOr in nftables.nix.
  # Dereferencing .macs on null is a Nix eval error.
  allowlistEnabled = config.router.nftables.allowedMacs != null;
  allowlistMacs =
    if allowlistEnabled
    then config.router.nftables.allowedMacs.macs
    else [];

  dashboardConfigJson = builtins.toJSON {
    tunnels = map (t: {
      name = t.name;
      interface = t.name;
      fwmark = t.fwmarkHex;
      routing_table = t.routingTable;
    }) tunnelMeta;
    pools = lib.mapAttrsToList (name: members: {
      inherit name members;
    }) config.router.pbr.pools;
    pooled_rules = map (r: {
      sources = r.sources;
      pool = r.pool;
    }) config.router.pbr.pooledRules;
    static_leases = map (l: {
      mac = l.mac;
      ip = l.ip;
      name = if l.name != null then l.name else "";
    }) config.router.dhcp.staticLeases;
    allowlist_enabled = allowlistEnabled;
    allowed_macs = allowlistMacs;
    lan_interface = config.router.lan.interface;
  };

  dashboardConfigFile = pkgs.writeText "dashboard-config.json" dashboardConfigJson;
in
{
  options.router.dashboard = {
    enable = lib.mkEnableOption "the router dashboard";

    allowedSources = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      example = [ "192.168.1.117" "192.168.1.119" ];
      description = ''
        Source IPv4 addresses or CIDR ranges permitted to reach the dashboard
        port. Anything not listed here is dropped by an nftables rule generated
        in the router's input chain, before any HTTP handler runs.

        Must be non-empty for the module to activate (enforced by assertion).
        This is the primary trust boundary for the dashboard — pick only admin
        devices you actually use to view it, and give them static DHCP leases
        via `router.dhcp.staticLeases` so their IPs are stable.

        IPv6 is not currently supported (the router has IPv6 disabled end-to-end).
      '';
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 9090;
      description = "TCP port to bind the dashboard HTTP server on.";
    };

    bindAddress = lib.mkOption {
      type = lib.types.str;
      default = (builtins.head config.router.lan.addresses).address;
      description = "Address to bind. Defaults to the primary LAN address.";
    };

    adguardUrl = lib.mkOption {
      type = lib.types.str;
      default = "http://127.0.0.1:3000";
      description = "Base URL for the AdGuard Home REST API.";
    };

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      description = "The dashboard package to run.";
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [ "debug" "info" "warn" "error" ];
      default = "info";
      description = "slog level.";
    };
  };

  # Activation gate: enabled AND the resolved package is not the bootstrap
  # placeholder. The default package (./package.nix) inherits its version
  # from version.json, so in bootstrap state `cfg.package.version` equals
  # "bootstrap" and the gate blocks activation — a silent no-op that lets
  # users pull a branch whose CI hasn't yet bumped version.json without
  # tripping package.nix's `throw`. A local-dev override
  # (`router.dashboard.package = pkgs.callPackage ./local.nix {}`) uses
  # the overridden version string (not "bootstrap"), so the gate passes
  # and the module activates against the hand-built binary. Reading
  # `cfg.package.version` does not force `src`, so pure eval stays clean.
  config = lib.mkIf (cfg.enable && (cfg.package.version or "") != "bootstrap") {
    assertions = [
      {
        assertion = cfg.allowedSources != [ ];
        message = ''
          router.dashboard.enable = true requires router.dashboard.allowedSources
          to be non-empty. The dashboard aggregates sensitive data (per-client DNS
          history, MAC/IP mapping, routing state) and must not be reachable
          LAN-wide. Declare the IPs or CIDR ranges of admin devices that should
          reach the dashboard — everything else is dropped at the firewall.
        '';
      }
    ];

    router.nftables.extraInputRules =
      let
        srcSet = lib.concatStringsSep ", " cfg.allowedSources;
        lanIf = config.router.lan.interface;
      in
      ''
        iifname "${lanIf}" tcp dport ${toString cfg.port} ip saddr { ${srcSet} } accept
        iifname "${lanIf}" tcp dport ${toString cfg.port} drop
      '';

    systemd.services.router-dashboard = {
      description = "Router observability dashboard";
      wantedBy = [ "multi-user.target" ];
      after = [
        "network-online.target"
        "nftables.service"
        "adguardhome.service"
      ];
      wants = [ "network-online.target" ];
      path = [
        pkgs.nftables
        pkgs.wireguard-tools
        pkgs.iproute2
        pkgs.conntrack-tools
        pkgs.iputils
      ];

      serviceConfig = {
        ExecStart = "${cfg.package}/bin/dashboard"
          + " --bind=${cfg.bindAddress}:${toString cfg.port}"
          + " --adguard-url=${cfg.adguardUrl}"
          + " --log-level=${cfg.logLevel}"
          + " --config-file=${dashboardConfigFile}";
        Restart = "on-failure";
        RestartSec = 3;

        DynamicUser = true;
        AmbientCapabilities = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];
        CapabilityBoundingSet = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictNamespaces = true;
        RestrictRealtime = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";

        # The `-` prefix makes each path optional: systemd silently skips
        # any that don't exist at service start. Required because:
        #   - /run/wg-pool-health only exists when router.pbr.pooledRules
        #     is non-empty (it's the RuntimeDirectory of wg-pool-health.service).
        #   - /var/lib/dnsmasq only exists when services.dnsmasq is enabled.
        #   - /sys/devices/virtual/thermal may be absent on non-RPi4 hosts.
        # Without the prefix, systemd refuses to start the unit if any
        # listed path is missing.
        ReadOnlyPaths = [
          "${dashboardConfigFile}"
          "-/run/wg-pool-health"
          "-/var/lib/dnsmasq"
          "-/sys/class/thermal"
          "-/sys/devices/virtual/thermal"
        ];
      };
    };
  };
}

{ config, lib, pkgs, ... }:

let
  cfg = config.router.dashboard;
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

  # Note on the activation gate:
  # Only `cfg.enable` gates the module. When `version.json` is still the
  # bootstrap placeholder AND the user has not overridden `cfg.package`,
  # realization of `cfg.package` triggers package.nix's `throw` with a clear
  # error message — but only at build time. Pure eval (`nix flake check
  # --no-build`) remains clean because the thrown `src` is lazy. This shape
  # lets a developer doing local testing set `router.dashboard.package =
  # pkgs.callPackage ./local-build.nix {};` and have the module fully
  # activate against a hand-built binary before CI has released anything.
  config = lib.mkIf cfg.enable {
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
          + " --log-level=${cfg.logLevel}";
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

        ReadOnlyPaths = [
          "/run/wg-pool-health"
          "/var/lib/dnsmasq"
          "/sys/class/thermal"
          "/sys/devices/virtual/thermal"
        ];
      };
    };
  };
}

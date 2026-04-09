{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  routerLib = import ./lib.nix { inherit lib; };
  allMeta = routerLib.allMeta cfg.wireguard.tunnels;
  tunnelMeta = routerLib.tunnelMeta cfg.wireguard.tunnels;
  wanMeta = routerLib.wanMeta;

  wanIf = cfg.wan.interface;

  hasPbr = cfg.pbr.sourceRules != [] || cfg.pbr.domainSets != {} || cfg.pbr.sourceDomainRules != [];

  refreshWanRoute = pkgs.writeShellScript "refresh-wan-route" ''
    set -eu
    IP=${pkgs.iproute2}/bin/ip
    WAN_GW="''${1:-}"
    if [ -z "$WAN_GW" ]; then
      set -- $($IP route show default dev ${wanIf} 2>/dev/null || true)
      if [ "''${1:-}" = "default" ] && [ "''${2:-}" = "via" ] && [ -n "''${3:-}" ]; then
        WAN_GW="$3"
      fi
    fi
    if [ -n "$WAN_GW" ]; then
      $IP route replace default via "$WAN_GW" dev ${wanIf} table ${toString wanMeta.routingTable}
    fi
  '';

  # ip route lines for each VPN tunnel
  tunnelRouteLines = lib.concatMapStringsSep "\n      " (t:
    ''ip route replace default dev ${t.name} table ${toString t.routingTable} 2>/dev/null || true''
  ) tunnelMeta;

  # ip rule del + add for all marks (wan + tunnels)
  ruleDelLines = lib.concatMapStringsSep "\n      " (m:
    ''ip rule del fwmark ${m.fwmarkHex}/${wanMeta.fwmarkMask} 2>/dev/null || true''
  ) allMeta;
  ruleAddLines = lib.concatMapStringsSep "\n      " (m:
    ''ip rule add fwmark ${m.fwmarkHex}/${wanMeta.fwmarkMask} table ${toString m.routingTable} priority ${toString m.routingTable}''
  ) allMeta;

  # WireGuard service names for After= dependency
  wgServiceNames = map (t: "wireguard-${t.name}.service") tunnelMeta;

  # Domain resolver: resolve_to_set calls per domainSet
  domainSetNames = builtins.attrNames cfg.pbr.domainSets;
  resolverCalls = lib.concatMapStringsSep "\n      " (setName:
    let
      domains = cfg.pbr.domainSets.${setName};
      nftSetName = "${builtins.replaceStrings ["-"] ["_"] setName}_domains";
      domainArgs = lib.concatMapStringsSep " \\\n        " (d: d) domains;
    in
    ''resolve_to_set mangle ${nftSetName} \
        ${domainArgs}''
  ) domainSetNames;
in
{
  options.router.pbr = {
    sourceRules = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          sources = lib.mkOption { type = lib.types.listOf lib.types.str; description = "Client IPs."; };
          tunnel = lib.mkOption { type = lib.types.str; description = "Tunnel name or \"wan\"."; };
        };
      });
      default = [];
      description = "Source-based PBR rules.";
    };

    domainSets = lib.mkOption {
      type = lib.types.attrsOf (lib.types.listOf lib.types.str);
      default = {};
      description = "Domain sets for destination-based PBR. Key = tunnel name or \"wan\".";
    };

    sourceDomainRules = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          source = lib.mkOption { type = lib.types.str; };
          domainSet = lib.mkOption { type = lib.types.str; };
          tunnel = lib.mkOption { type = lib.types.str; };
        };
      });
      default = [];
      description = "Source + domain combination PBR rules.";
    };
  };

  config = lib.mkIf (cfg.enable && (tunnelMeta != [] || hasPbr)) {
    systemd.services.policy-routing = {
      description = "Policy-based routing tables and rules";
      after = [ "network-online.target" ] ++ wgServiceNames;
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.iproute2 ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };

      script = ''
        ${tunnelRouteLines}

        ${refreshWanRoute}

        ${ruleDelLines}

        ${ruleAddLines}
      '';
    };

    systemd.services.pbr-domains = lib.mkIf (cfg.pbr.domainSets != {}) {
      description = "Resolve PBR domains to IPs for nftables sets";
      after = [ "network-online.target" "nftables.service" "adguardhome.service" ];
      wants = [ "network-online.target" "adguardhome.service" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.coreutils pkgs.dig pkgs.gnugrep pkgs.nftables ];

      serviceConfig = {
        Type = "oneshot";
      };

      script = ''
        resolve_to_set() {
          local table="$1" set="$2"
          shift 2
          local elements
          elements="$(
            for domain in "$@"; do
              dig +short +timeout=5 @127.0.0.1 A "$domain" 2>/dev/null
            done | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | sort -u | paste -sd, -
          )"
          if [ -n "$elements" ]; then
            {
              printf 'flush set ip %s %s\n' "$table" "$set"
              printf 'add element ip %s %s { %s }\n' "$table" "$set" "$elements"
            } | nft -f -
          else
            echo "pbr-domains: no IPv4 addresses resolved for $table/$set; keeping existing entries" >&2
          fi
        }

        ${resolverCalls}
      '';
    };

    systemd.timers.pbr-domains = lib.mkIf (cfg.pbr.domainSets != {}) {
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnBootSec = "1min";
        OnUnitActiveSec = "30min";
        RandomizedDelaySec = "5min";
      };
    };
  };
}

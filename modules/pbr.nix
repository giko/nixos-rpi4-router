{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  routerLib = import ./lib.nix { inherit lib; };
  allMeta = routerLib.allMeta cfg.wireguard.tunnels;
  tunnelMeta = routerLib.tunnelMeta cfg.wireguard.tunnels;
  wanMeta = routerLib.wanMeta;

  wanIf = cfg.wan.interface;

  hasPbr = cfg.pbr.sourceRules != [] || cfg.pbr.domainSets != {} || cfg.pbr.sourceDomainRules != [] || cfg.pbr.pooledRules != [];

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

  # wg-pool-health: Nix-generated bash constants for the watchdog script.
  # Whitespace-separated list of all declared tunnel names (probe order).
  poolTunnelOrderStr = lib.concatStringsSep " " (map (t: t.name) tunnelMeta);
  # Associative array initializer: [wg_sw]=0x20000 [wg_tr]=0x30000 ...
  poolTunnelFwmarksStr = lib.concatMapStringsSep " "
    (t: "[${t.name}]=${t.fwmarkHex}")
    tunnelMeta;
  # Indexed array of "sources|tunnels" pool definitions, each element quoted.
  # Pool order inside the pool definition controls the numgen index → fwmark
  # mapping. If two pooledRules reference the same pool, each gets its own
  # bash entry (separate nftables rules, same tunnel list).
  poolEntriesStr = lib.concatMapStringsSep "\n      " (rule:
    let
      srcs = lib.concatStringsSep "," rule.sources;
      tuns = lib.concatStringsSep "," cfg.pbr.pools.${rule.pool};
    in
    ''"${srcs}|${tuns}"''
  ) cfg.pbr.pooledRules;
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

    pools = lib.mkOption {
      type = lib.types.attrsOf (lib.types.listOf lib.types.str);
      default = {};
      example = {
        all_vpns = [ "wg_sw" "wg_us" "wg_tr" ];
        europe = [ "wg_sw" "wg_tr" ];
      };
      description = ''
        Named tunnel pools for per-connection round-robin. Each pool is a
        list of WireGuard tunnel names (declared in router.wireguard.tunnels)
        to round-robin across. Referenced by name from router.pbr.pooledRules.

        The order of tunnels inside a pool matters: it controls the numgen
        index → fwmark mapping deterministically, so a given bash re-run
        produces the same rule output. Distribution is still fair because
        numgen inc increments every new connection regardless of order.
      '';
    };

    pooledRules = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          sources = lib.mkOption {
            type = lib.types.listOf lib.types.str;
            description = "Source IPs that share this pool.";
          };
          pool = lib.mkOption {
            type = lib.types.str;
            description = "Name of a pool declared in router.pbr.pools.";
          };
        };
      });
      default = [];
      description = ''
        Per-connection round-robin pooling. New flows from each rule's
        sources are distributed across the tunnels of the referenced pool
        via nftables numgen; subsequent packets of the same flow stay
        sticky on the chosen tunnel via conntrack (the existing mangle
        prerouting chain saves meta mark to ct mark on the first packet
        and restores it on follow-ups).

        An active watchdog (wg-pool-health.service) probes each tunnel
        every 5 s with HTTPS through the interface, atomically removes
        unhealthy tunnels from each pool, and drops traffic (fail-closed)
        when all tunnels in a pool are down. State is exposed at
        /run/wg-pool-health/state.json.

        Sources listed here MUST NOT overlap with router.pbr.sourceRules —
        an eval-time assertion enforces this to avoid ambiguous routing.
      '';
    };
  };

  config = lib.mkIf (cfg.enable && (tunnelMeta != [] || hasPbr)) {
    assertions = [
      {
        assertion = let
          srcIps = builtins.concatLists (map (r: r.sources) cfg.pbr.sourceRules);
          pooledIps = builtins.concatLists (map (r: r.sources) cfg.pbr.pooledRules);
        in (lib.intersectLists srcIps pooledIps) == [];
        message = "router.pbr.pooledRules and router.pbr.sourceRules share source IPs (${
          toString (lib.intersectLists
            (builtins.concatLists (map (r: r.sources) cfg.pbr.sourceRules))
            (builtins.concatLists (map (r: r.sources) cfg.pbr.pooledRules)))
        }). Each client IP must be in exactly one of the two.";
      }
      {
        # pooledRules.pool must exist in pbr.pools
        assertion = let
          declaredPools = builtins.attrNames cfg.pbr.pools;
          refs = lib.unique (map (r: r.pool) cfg.pbr.pooledRules);
        in (lib.subtractLists declaredPools refs) == [];
        message = "router.pbr.pooledRules references undeclared pool(s): ${
          toString (lib.subtractLists
            (builtins.attrNames cfg.pbr.pools)
            (lib.unique (map (r: r.pool) cfg.pbr.pooledRules)))
        }. Declared pools: ${toString (builtins.attrNames cfg.pbr.pools)}.";
      }
      {
        # Every tunnel named inside any pool must exist in wireguard.tunnels
        assertion = let
          declaredTunnels = builtins.attrNames cfg.wireguard.tunnels;
          tunnelRefs = lib.unique (builtins.concatLists (builtins.attrValues cfg.pbr.pools));
        in (lib.subtractLists declaredTunnels tunnelRefs) == [];
        message = "router.pbr.pools references unknown tunnel(s): ${
          toString (lib.subtractLists
            (builtins.attrNames cfg.wireguard.tunnels)
            (lib.unique (builtins.concatLists (builtins.attrValues cfg.pbr.pools))))
        }. Declared tunnels: ${toString (builtins.attrNames cfg.wireguard.tunnels)}.";
      }
    ];

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

    systemd.services.wg-pool-health = lib.mkIf (cfg.pbr.pooledRules != []) {
      description = "WireGuard tunnel health monitor and pool chain updater";
      after = [ "nftables.service" "network-online.target" ] ++ wgServiceNames;
      wants = [ "network-online.target" ];
      partOf = [ "nftables.service" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.curl pkgs.nftables pkgs.coreutils ];

      serviceConfig = {
        Type = "simple";
        Restart = "always";
        RestartSec = "5";
        RuntimeDirectory = "wg-pool-health";
        RuntimeDirectoryMode = "0755";
      };

      script = ''
        set -u

        STATE_DIR=/run/wg-pool-health
        STATE_FILE=$STATE_DIR/state.json
        POOL_FILE=$STATE_DIR/pool.nft

        TUNNEL_ORDER=(${poolTunnelOrderStr})
        declare -A TUNNEL_FWMARKS=(${poolTunnelFwmarksStr})
        POOLS=(
          ${poolEntriesStr}
        )

        PROBE_URL=https://1.1.1.1/
        PROBE_TIMEOUT=3
        LOOP_INTERVAL=5
        FAILURE_THRESHOLD=2

        declare -A HEALTHY FAILS
        for t in "''${TUNNEL_ORDER[@]}"; do
          HEALTHY[$t]=1
          FAILS[$t]=0
        done

        probe() {
          curl --interface "$1" --max-time "$PROBE_TIMEOUT" -sS -o /dev/null "$PROBE_URL"
        }

        build_pool_nft() {
          printf 'flush chain ip mangle pool\n'
          for p in "''${POOLS[@]}"; do
            IFS='|' read -r sources tunnels <<< "$p"
            IFS=',' read -ra tun_arr <<< "$tunnels"
            healthy_tuns=()
            for t in "''${tun_arr[@]}"; do
              [ "''${HEALTHY[$t]}" = "1" ] && healthy_tuns+=("$t")
            done
            src_set="''${sources//,/, }"
            if [ ''${#healthy_tuns[@]} -eq 0 ]; then
              printf 'add rule ip mangle pool ct state new ip saddr { %s } counter drop\n' "$src_set"
              continue
            fi
            map_str=""
            for i in "''${!healthy_tuns[@]}"; do
              mark="''${TUNNEL_FWMARKS[''${healthy_tuns[$i]}]}"
              [ -n "$map_str" ] && map_str+=", "
              map_str+="$i : $mark"
            done
            n=''${#healthy_tuns[@]}
            printf 'add rule ip mangle pool ct state new ip saddr { %s } meta mark set numgen inc mod %d map { %s }\n' \
              "$src_set" "$n" "$map_str"
          done
        }

        write_state_json() {
          now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
          {
            printf '{\n'
            printf '  "updated_at": "%s",\n' "$now"
            printf '  "tunnels": {\n'
            first=1
            for t in "''${TUNNEL_ORDER[@]}"; do
              [ "$first" -eq 0 ] && printf ',\n'
              first=0
              h=false
              [ "''${HEALTHY[$t]}" = "1" ] && h=true
              printf '    "%s": { "healthy": %s, "consecutive_failures": %d }' \
                "$t" "$h" "''${FAILS[$t]}"
            done
            printf '\n  }\n}\n'
          } > "$STATE_FILE.tmp"
          mv "$STATE_FILE.tmp" "$STATE_FILE"
        }

        reload_pool() {
          build_pool_nft > "$POOL_FILE.tmp"
          mv "$POOL_FILE.tmp" "$POOL_FILE"
          if ! nft -f "$POOL_FILE" 2>&1; then
            echo "wg-pool-health: nft -f failed" >&2
          fi
        }

        reload_pool
        write_state_json

        while :; do
          changed=0
          for t in "''${TUNNEL_ORDER[@]}"; do
            if probe "$t" 2>/dev/null; then
              FAILS[$t]=0
              if [ "''${HEALTHY[$t]}" = "0" ]; then
                HEALTHY[$t]=1
                changed=1
              fi
            else
              FAILS[$t]=$((FAILS[$t] + 1))
              if [ "''${HEALTHY[$t]}" = "1" ] && [ "''${FAILS[$t]}" -ge "$FAILURE_THRESHOLD" ]; then
                HEALTHY[$t]=0
                changed=1
              fi
            fi
          done
          [ "$changed" -eq 1 ] && reload_pool
          write_state_json
          sleep "$LOOP_INTERVAL"
        done
      '';
    };
  };
}

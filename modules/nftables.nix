{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  routerLib = import ./lib.nix { inherit lib; };
  tunnelMeta = routerLib.tunnelMeta cfg.wireguard.tunnels;
  tunnelNames = map (t: t.name) tunnelMeta;

  lanIf = cfg.lan.interface;
  wanIf = cfg.wan.interface;

  # First LAN address is the "primary" (used for DNS hijack target, etc.)
  primaryAddr = (builtins.head cfg.lan.addresses).address;
  hasMultipleSubnets = builtins.length cfg.lan.addresses > 1;

  # Port forward DNAT rules
  dnatRules = lib.concatMapStringsSep "\n          " (pf:
    ''iifname "${wanIf}" ${pf.proto} dport ${toString pf.externalPort} dnat to ${pf.destination}''
  ) cfg.portForwards;

  # SNAT rules (explicit, user-configured)
  snatLines = lib.concatMapStringsSep "\n          " (rule:
    ''oifname "${lanIf}" ip saddr ${rule.sourceNet} ip daddr ${rule.destNet} snat to ${rule.snatAddress}''
  ) cfg.nftables.snatRules;

  # Masquerade lines for WAN + all tunnels
  masqueradeLines = lib.concatMapStringsSep "\n          "
    (name: ''oifname "${name}" masquerade'')
    ([ wanIf ] ++ tunnelNames);

  # Tunnel names as nftables set for forward rules
  tunnelSet = lib.concatMapStringsSep ", " (n: ''"${n}"'') tunnelNames;

  # Mangle: source PBR rules
  sourcePbrLines = lib.concatMapStringsSep "\n          " (rule:
    let
      meta = routerLib.tunnelMeta cfg.wireguard.tunnels;
      allMeta = routerLib.allMeta cfg.wireguard.tunnels;
      target = lib.findFirst (m: m.name == rule.tunnel) (builtins.throw "Unknown tunnel: ${rule.tunnel}") allMeta;
      srcSet = lib.concatMapStringsSep ", " (s: s) rule.sources;
    in
    ''ip saddr { ${srcSet} } meta mark set ${target.fwmarkHex}''
  ) cfg.pbr.sourceRules;

  # Mangle: domain set PBR rules
  domainSetNames = builtins.attrNames cfg.pbr.domainSets;
  domainPbrLines = lib.concatMapStringsSep "\n          " (setName:
    let
      allMeta = routerLib.allMeta cfg.wireguard.tunnels;
      target = lib.findFirst (m: m.name == setName) (builtins.throw "Unknown tunnel for domain set: ${setName}") allMeta;
      nftSetName = "${builtins.replaceStrings ["-"] ["_"] setName}_domains";
    in
    ''ip daddr @${nftSetName} meta mark set ${target.fwmarkHex}''
  ) domainSetNames;

  # Mangle: source+domain rules (validate domainSet references at eval time)
  srcDomainPbrLines = lib.concatMapStringsSep "\n          " (rule:
    let
      allMeta = routerLib.allMeta cfg.wireguard.tunnels;
      target = lib.findFirst (m: m.name == rule.tunnel) (builtins.throw "Unknown tunnel: ${rule.tunnel}") allMeta;
      _ = if !(builtins.hasAttr rule.domainSet cfg.pbr.domainSets)
          then builtins.throw "sourceDomainRules references unknown domainSet '${rule.domainSet}'. Declared sets: ${toString (builtins.attrNames cfg.pbr.domainSets)}"
          else true;
      nftSetName = "${builtins.replaceStrings ["-"] ["_"] rule.domainSet}_domains";
    in
    ''ip saddr ${rule.source} ip daddr @${nftSetName} meta mark set ${target.fwmarkHex}''
  ) cfg.pbr.sourceDomainRules;

  # Mangle: domain IP set declarations
  domainSetDecls = lib.concatMapStringsSep "\n        " (setName:
    let nftSetName = "${builtins.replaceStrings ["-"] ["_"] setName}_domains";
    in ''
        set ${nftSetName} {
          type ipv4_addr
          flags interval
        }''
  ) domainSetNames;

  # Mangle: pool chain initial content (optimistic — assumes all tunnels
  # healthy). The wg-pool-health watchdog rewrites this chain atomically
  # as tunnels come and go.
  allTunnelMeta = routerLib.allMeta cfg.wireguard.tunnels;
  lookupFwmark = tName:
    (lib.findFirst
      (m: m.name == tName)
      (builtins.throw "pooledRules references unknown tunnel: ${tName}")
      allTunnelMeta).fwmarkHex;
  poolInitLines = lib.concatMapStringsSep "\n            " (rule:
    let
      poolTunnels = cfg.pbr.pools.${rule.pool};
      indexedMarks = lib.imap0 (i: t: "${toString i} : ${lookupFwmark t}") poolTunnels;
      mapStr = lib.concatStringsSep ", " indexedMarks;
      srcSet = lib.concatStringsSep ", " rule.sources;
      n = toString (builtins.length poolTunnels);
    in
    ''ct state new ip saddr { ${srcSet} } meta mark set numgen inc mod ${n} map { ${mapStr} }''
  ) cfg.pbr.pooledRules;

  # Union of all source IPs across all pooled rules — used for the
  # failsafe drop rule in prerouting (catches packets from pooled clients
  # that somehow reached the end of the pool chain without getting a mark,
  # e.g. if the watchdog flushed pool without repopulating).
  pooledSources = lib.unique (builtins.concatLists (map (r: r.sources) cfg.pbr.pooledRules));
  pooledSourcesSet = lib.concatStringsSep ", " pooledSources;

  # Drop all forwarded traffic from these MACs (internet + VPN + cross-subnet).
  # Intra-subnet traffic is unaffected since it is not routed through forward.
  blockMacLines = lib.concatMapStringsSep "\n          " (mac:
    ''ether saddr ${mac} counter drop''
  ) cfg.nftables.blockedMacs;

  # Default-deny MAC allowlist scoped to a source subnet. Survives iOS
  # Private Wi-Fi Address rotation because it flips the policy: any
  # MAC not in the set is dropped instead of trying to enumerate bad MACs.
  # MACs from router.dhcp.staticLeases are auto-merged so adding a static
  # lease implicitly trusts the device.
  hasAllowlist = cfg.nftables.allowedMacs != null;
  allowedMacsFinal =
    if hasAllowlist then
      lib.unique (
        (map lib.toLower cfg.nftables.allowedMacs.macs)
        ++ (map (l: lib.toLower l.mac) cfg.dhcp.staticLeases)
      )
    else [];
  allowedMacsSet = lib.concatMapStringsSep ", " (m: m) allowedMacsFinal;

  # UPnP pre-accept deny rules (block UPnP ports from secondary subnets)
  upnpDenyLines =
    if cfg.upnp.enable && hasMultipleSubnets then
      let
        secondaryAddrs = builtins.tail cfg.lan.addresses;
        secondarySubnets = map routerLib.addrToSubnet secondaryAddrs;
      in lib.concatMapStringsSep "\n          " (subnet: ''
          ip saddr ${subnet} udp dport { 1900, 5351 } drop
          ip saddr ${subnet} tcp dport 5000 drop'') secondarySubnets
    else "";
in
{
  options.router = {
    portForwards = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          proto = lib.mkOption { type = lib.types.str; description = "Protocol (tcp or udp)."; };
          externalPort = lib.mkOption { type = lib.types.int; description = "Port on WAN side."; };
          destination = lib.mkOption { type = lib.types.str; description = "Internal ip:port target."; };
        };
      });
      default = [];
      description = "Port forwarding rules (DNAT from WAN).";
    };

    nftables = {
      snatRules = lib.mkOption {
        type = lib.types.listOf (lib.types.submodule {
          options = {
            sourceNet = lib.mkOption { type = lib.types.str; };
            destNet = lib.mkOption { type = lib.types.str; };
            snatAddress = lib.mkOption { type = lib.types.str; };
          };
        });
        default = [];
        description = "Explicit inter-subnet SNAT rules. Empty by default — plain routing.";
      };

      extraInputRules = lib.mkOption {
        type = lib.types.lines;
        default = "";
        description = "Raw nftables inserted in input chain before LAN accept.";
      };

      extraForwardRules = lib.mkOption {
        type = lib.types.lines;
        default = "";
        description = "Raw nftables inserted in forward chain before inter-subnet accept.";
      };

      blockedMacs = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [];
        example = [ "aa:bb:cc:dd:ee:ff" ];
        description = ''
          MAC addresses to drop in the forward chain. Blocks internet,
          VPN tunnels, and cross-subnet access. Intra-subnet LAN traffic
          is unaffected since it is not forwarded by the router.
        '';
      };

      allowedMacs = lib.mkOption {
        type = lib.types.nullOr (lib.types.submodule {
          options = {
            sourceNet = lib.mkOption {
              type = lib.types.str;
              example = "192.168.1.0/24";
              description = "Source IP subnet the allowlist is scoped to.";
            };
            macs = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [];
              example = [ "aa:bb:cc:dd:ee:ff" ];
              description = ''
                Explicit allowlist of source MACs within sourceNet. MACs from
                router.dhcp.staticLeases are auto-merged into this list, so
                only dynamic devices without a static lease need listing here.
              '';
            };
          };
        });
        default = null;
        description = ''
          Default-deny MAC allowlist for forwarded traffic originating from
          sourceNet. Any source MAC in sourceNet that is not in the allowlist
          (macs + router.dhcp.staticLeases) has its forwarded traffic dropped.

          Use this instead of blockedMacs when you need to block a device
          that rotates its MAC address (e.g. iOS Private Wi-Fi Address +
          "Forget This Network"). Other source subnets (e.g. server subnets)
          are unaffected; intra-subnet LAN traffic is also unaffected since
          it is not forwarded.
        '';
      };

      extraPreroutingRules = lib.mkOption {
        type = lib.types.lines;
        default = "";
        description = "Raw nftables inserted in prerouting NAT chain.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    networking.nftables = {
      enable = true;
      checkRuleset = false;
      ruleset = ''
        table inet filter {

          chain input {
            type filter hook input priority filter; policy drop;

            ct state established,related accept
            ct state invalid drop

            iif "lo" accept

            ${upnpDenyLines}
            ${lib.optionalString (cfg.nftables.extraInputRules != "") cfg.nftables.extraInputRules}

            iifname "${lanIf}" accept

            iifname "${wanIf}" udp dport 68 accept
            iifname "${wanIf}" icmp type echo-request limit rate 10/second accept

            counter drop
          }

          chain forward {
            type filter hook forward priority filter; policy drop;

            ${lib.optionalString (cfg.nftables.blockedMacs != []) blockMacLines}

            ${lib.optionalString hasAllowlist ''iifname "${lanIf}" ip saddr ${cfg.nftables.allowedMacs.sourceNet} ether saddr != { ${allowedMacsSet} } counter drop''}

            ct state established,related accept
            ct state invalid drop

            ${lib.optionalString (cfg.nftables.extraForwardRules != "") cfg.nftables.extraForwardRules}

            ${lib.optionalString hasMultipleSubnets ''iifname "${lanIf}" oifname "${lanIf}" accept''}

            iifname "${lanIf}" oifname "${wanIf}" accept

            ${lib.optionalString (tunnelNames != []) ''iifname "${lanIf}" oifname { ${tunnelSet} } accept''}

            ${lib.optionalString (tunnelNames != []) ''iifname { ${tunnelSet} } oifname "${wanIf}" accept''}

            iifname "${wanIf}" oifname "${lanIf}" ct status dnat accept

            counter drop
          }

          chain output {
            type filter hook output priority filter; policy accept;
          }
        }

        table ip nat {

          chain prerouting {
            type nat hook prerouting priority dstnat; policy accept;

            iifname "${lanIf}" ip daddr != ${primaryAddr} udp dport 53 dnat to ${primaryAddr}
            iifname "${lanIf}" ip daddr != ${primaryAddr} tcp dport 53 dnat to ${primaryAddr}

            ${lib.optionalString (cfg.nftables.extraPreroutingRules != "") cfg.nftables.extraPreroutingRules}

            ${dnatRules}
          }

          chain postrouting {
            type nat hook postrouting priority srcnat; policy accept;

            ${snatLines}

            ${masqueradeLines}
          }
        }

        ${lib.optionalString (cfg.pbr.sourceRules != [] || cfg.pbr.domainSets != {} || cfg.pbr.sourceDomainRules != [] || cfg.pbr.pooledRules != []) ''
        table ip mangle {

          ${domainSetDecls}

          ${lib.optionalString (cfg.pbr.pooledRules != []) ''
          chain pool {
            ${poolInitLines}
          }
          ''}

          chain prerouting {
            type filter hook prerouting priority mangle; policy accept;

            iifname != "${lanIf}" accept

            ct state established,related meta mark set ct mark return

            ${sourcePbrLines}

            ${lib.optionalString (cfg.pbr.pooledRules != []) ''jump pool''}

            ${lib.optionalString (cfg.pbr.pooledRules != []) ''ct state new ip saddr { ${pooledSourcesSet} } meta mark 0 counter drop''}

            ${domainPbrLines}

            ${srcDomainPbrLines}

            meta mark != 0 ct mark set meta mark
          }
        }
        ''}
      '';
    };
  };
}

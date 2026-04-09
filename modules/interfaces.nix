{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  routerLib = import ./lib.nix { inherit lib; };
in
{
  options.router = {
    lan = {
      interface = lib.mkOption {
        type = lib.types.str;
        default = "eth0";
        description = "LAN interface name.";
      };
      macAddress = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "If set, generates a udev rule to pin this MAC to the LAN interface name.";
      };
      addresses = lib.mkOption {
        type = lib.types.listOf (lib.types.submodule {
          options = {
            address = lib.mkOption { type = lib.types.str; };
            prefixLength = lib.mkOption { type = lib.types.int; };
          };
        });
        default = [{ address = "192.168.1.1"; prefixLength = 24; }];
        description = "LAN IP addresses and subnets.";
      };
    };

    wan = {
      interface = lib.mkOption {
        type = lib.types.str;
        default = "eth1";
        description = "WAN interface name.";
      };
      macAddress = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "If set, generates a udev rule to pin this MAC to the WAN interface name.";
      };
      useDHCP = lib.mkOption {
        type = lib.types.bool;
        default = true;
        description = "Use DHCP on WAN interface.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    # Global network guards
    networking = {
      useDHCP = false;
      usePredictableInterfaceNames = false;
      enableIPv6 = false;
      firewall.enable = false;

      interfaces = {
        ${cfg.lan.interface}.ipv4.addresses = cfg.lan.addresses;
        ${cfg.wan.interface}.useDHCP = cfg.wan.useDHCP;
      };

      # Refresh WAN routing table on DHCP lease changes.
      dhcpcd.runHook = let
        refreshScript = pkgs.writeShellScript "refresh-wan-route" ''
          set -eu
          IP=${pkgs.iproute2}/bin/ip
          WAN_GW="''${1:-}"
          if [ -z "$WAN_GW" ]; then
            set -- $($IP route show default dev ${cfg.wan.interface} 2>/dev/null || true)
            if [ "''${1:-}" = "default" ] && [ "''${2:-}" = "via" ] && [ -n "''${3:-}" ]; then
              WAN_GW="$3"
            fi
          fi
          if [ -n "$WAN_GW" ]; then
            $IP route replace default via "$WAN_GW" dev ${cfg.wan.interface} table ${toString routerLib.wanMeta.routingTable}
          fi
        '';
      in ''
        case "$interface:$reason" in
          ${cfg.wan.interface}:BOUND|${cfg.wan.interface}:REBIND|${cfg.wan.interface}:RENEW|${cfg.wan.interface}:REBOOT|${cfg.wan.interface}:TIMEOUT)
            ${refreshScript} "''${new_routers%% *}" || true
            ;;
        esac
      '';
    };

    services.resolved.enable = false;

    # Optional udev rules for MAC-based interface naming
    services.udev.extraRules = lib.concatStringsSep "\n" (
      lib.optional (cfg.lan.macAddress != null)
        ''SUBSYSTEM=="net", ATTR{address}=="${cfg.lan.macAddress}", NAME="${cfg.lan.interface}"''
      ++ lib.optional (cfg.wan.macAddress != null)
        ''SUBSYSTEM=="net", ATTR{address}=="${cfg.wan.macAddress}", NAME="${cfg.wan.interface}"''
    );
  };
}

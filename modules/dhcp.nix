{ config, lib, ... }:
let
  cfg = config.router;
  dcfg = cfg.dhcp;
  primaryAddr = (builtins.head cfg.lan.addresses).address;
  primaryPrefix = (builtins.head cfg.lan.addresses).prefixLength;

  # Convert prefix length to subnet mask string
  prefixToMask = p:
    if p == 24 then "255.255.255.0"
    else if p == 16 then "255.255.0.0"
    else if p == 8 then "255.0.0.0"
    else builtins.throw "Unsupported prefix length: ${toString p}";

  leaseEntries = map (lease:
    if lease ? name && lease.name != null
    then "${lease.mac},${lease.name},${lease.ip}"
    else "${lease.mac},${lease.ip}"
  ) dcfg.staticLeases;
in
{
  options.router.dhcp = {
    range = {
      start = lib.mkOption { type = lib.types.str; default = "192.168.1.100"; };
      end = lib.mkOption { type = lib.types.str; default = "192.168.1.250"; };
      leaseTime = lib.mkOption { type = lib.types.str; default = "12h"; };
    };
    domain = lib.mkOption { type = lib.types.str; default = "lan"; };
    staticLeases = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          mac = lib.mkOption { type = lib.types.str; };
          ip = lib.mkOption { type = lib.types.str; };
          name = lib.mkOption { type = lib.types.nullOr lib.types.str; default = null; };
        };
      });
      default = [];
      description = "Static DHCP reservations.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.dnsmasq = {
      enable = true;
      settings = {
        port = 0;
        interface = cfg.lan.interface;
        bind-interfaces = true;

        dhcp-range = [ "${dcfg.range.start},${dcfg.range.end},${prefixToMask primaryPrefix},${dcfg.range.leaseTime}" ];
        dhcp-option = [
          "6,${primaryAddr}"
          "3,${primaryAddr}"
        ];

        dhcp-host = leaseEntries;

        dhcp-authoritative = true;
        domain = dcfg.domain;
        expand-hosts = true;
      };
    };
  };
}

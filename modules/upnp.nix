{ config, lib, ... }:
let
  cfg = config.router;
  ucfg = cfg.upnp;

  denyLines = lib.concatMapStringsSep "\n" (r: "deny ${r}") ucfg.denyRules;
  allowLines = lib.concatMapStringsSep "\n" (r: "allow ${r}") ucfg.allowRules;
in
{
  options.router.upnp = {
    enable = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Enable UPnP/NAT-PMP (miniupnpd).";
    };
    natpmp = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Enable NAT-PMP alongside UPnP.";
    };
    denyRules = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "miniupnpd deny rules (without 'deny' prefix). Evaluated before allow rules.";
    };
    allowRules = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "miniupnpd allow rules (without 'allow' prefix).";
    };
  };

  config = lib.mkIf (cfg.enable && ucfg.enable) {
    services.miniupnpd = {
      enable = true;
      externalInterface = cfg.wan.interface;
      internalIPs = [ cfg.lan.interface ];
      natpmp = ucfg.natpmp;
      appendConfig = ''
        http_port=5000
        secure_mode=yes
        system_uptime=yes
        notify_interval=900

        ${denyLines}
        ${allowLines}
        deny 0-65535 0.0.0.0/0 0-65535
      '';
    };

    systemd.services.miniupnpd = {
      after = [ "network-online.target" "nftables.service" ];
      wants = [ "network-online.target" "nftables.service" ];
    };
  };
}

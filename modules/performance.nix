{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  pcfg = cfg.performance;
  lanIf = cfg.lan.interface;
  wanIf = cfg.wan.interface;
in
{
  options.router.performance = {
    flowOffload = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Enable software flow offloading. Service uses partOf=nftables.service.";
    };
    rps = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Enable Receive Packet Steering across all CPU cores.";
    };
    zram = {
      enable = lib.mkOption { type = lib.types.bool; default = true; };
      memoryPercent = lib.mkOption { type = lib.types.int; default = 25; };
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.flow-offload = lib.mkIf pcfg.flowOffload {
      description = "Add nftables flowtable for software flow offloading";
      after = [ "network-online.target" "nftables.service" ];
      wants = [ "network-online.target" "nftables.service" ];
      wantedBy = [ "multi-user.target" ];
      partOf = [ "nftables.service" ];
      path = [ pkgs.nftables ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };

      script = ''
        nft add flowtable inet filter f { hook ingress priority 0\; devices = { ${lanIf}, ${wanIf} }\; } 2>/dev/null || true
        nft add rule inet filter forward position 0 ip protocol { tcp, udp } ct state established flow add @f 2>/dev/null || true
      '';
    };

    systemd.services.packet-steering = lib.mkIf pcfg.rps {
      description = "Enable Receive Packet Steering (RPS) on all interfaces";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.coreutils ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };

      script = ''
        for iface in ${lanIf} ${wanIf}; do
          for rxq in /sys/class/net/$iface/queues/rx-*/rps_cpus; do
            [ -f "$rxq" ] && echo f > "$rxq" 2>/dev/null || true
          done
          for rxq in /sys/class/net/$iface/queues/rx-*/rps_flow_cnt; do
            [ -f "$rxq" ] && echo 4096 > "$rxq" 2>/dev/null || true
          done
        done
      '';
    };

    powerManagement.cpuFreqGovernor = "performance";

    zramSwap = lib.mkIf pcfg.zram.enable {
      enable = true;
      memoryPercent = pcfg.zram.memoryPercent;
    };
  };
}

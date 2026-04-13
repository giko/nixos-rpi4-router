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
      restartTriggers = [ config.systemd.services.flow-offload.script ];
      path = [ pkgs.nftables pkgs.gawk ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };

      script = ''
        # Idempotent rebuild: remove any existing definition, then recreate with current config.
        # Tolerates both fresh boot (nothing to delete) and upgrade (existing counter-less flowtable).
        for handle in $(nft -a list chain inet filter forward 2>/dev/null | awk '/^[[:space:]]*ip protocol \{ tcp, udp \} ct state established flow add @f([[:space:]]|$)/ { for (i = 1; i <= NF; i++) if ($i == "handle") print $(i + 1) }'); do
          nft delete rule inet filter forward handle "$handle" 2>/dev/null || true
        done
        nft delete flowtable inet filter f 2>/dev/null || true
        nft add flowtable inet filter f { hook ingress priority 0\; devices = { ${lanIf}, ${wanIf} }\; counter\; }
        nft add rule inet filter forward position 0 ip protocol { tcp, udp } ct state established flow add @f comment \"flow-offload-oneshot-owned\"
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

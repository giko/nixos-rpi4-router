{ config, lib, pkgs, ... }:
let
  cfg = config.router;
  lanIf = cfg.lan.interface;
  wanIf = cfg.wan.interface;
  ifbDev = "ifb4${wanIf}";
in
{
  options.router.qos = {
    uploadMbit = lib.mkOption {
      type = lib.types.int;
      default = 100;
      description = "WAN upload bandwidth for CAKE (Mbit).";
    };
    downloadMbit = lib.mkOption {
      type = lib.types.int;
      default = 900;
      description = "WAN download bandwidth for HTB+fq_codel via IFB (Mbit).";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.cake-qos = {
      description = "CAKE QoS traffic shaping (piece_of_cake)";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];
      path = [ pkgs.iproute2 pkgs.gnugrep ];

      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };

      script = ''
        TC=${pkgs.iproute2}/bin/tc
        IP=${pkgs.iproute2}/bin/ip

        # Fix LAN qdisc: if kernel created ${lanIf} before sysctl set
        # default_qdisc=fq_codel, its hardware queues get CAKE.
        # CAKE on LAN exhausts flow-tracking memory under heavy load.
        if $TC qdisc show dev ${lanIf} | grep -q cake; then
          $TC qdisc replace dev ${lanIf} root mq 2>/dev/null || true
        fi

        # Egress (upload) shaping on WAN
        $TC qdisc replace dev ${wanIf} root cake \
          bandwidth ${toString cfg.qos.uploadMbit}mbit overhead 44 mpu 84 noatm

        # Ingress (download) shaping via IFB device
        $IP link add name ${ifbDev} type ifb 2>/dev/null || true
        $IP link set dev ${ifbDev} up

        $TC qdisc del dev ${wanIf} handle ffff: ingress 2>/dev/null || true
        $TC qdisc add dev ${wanIf} handle ffff: ingress
        $TC filter add dev ${wanIf} parent ffff: protocol all prio 10 u32 \
          match u32 0 0 flowid 1:1 action mirred egress redirect dev ${ifbDev}

        # HTB rate-limits + fq_codel AQM
        $TC qdisc replace dev ${ifbDev} root handle 1: htb default 1
        $TC class add dev ${ifbDev} parent 1: classid 1:1 htb rate ${toString cfg.qos.downloadMbit}mbit quantum 1514
        $TC qdisc replace dev ${ifbDev} parent 1:1 fq_codel
      '';

      preStop = ''
        TC=${pkgs.iproute2}/bin/tc
        IP=${pkgs.iproute2}/bin/ip
        $TC qdisc del dev ${wanIf} root 2>/dev/null || true
        $TC qdisc del dev ${wanIf} handle ffff: ingress 2>/dev/null || true
        $IP link del ${ifbDev} 2>/dev/null || true
      '';
    };
  };
}

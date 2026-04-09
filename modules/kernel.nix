{ config, lib, pkgs, ... }:
let
  cfg = config.router;
in
{
  config = lib.mkIf cfg.enable {
    boot = {
      loader = {
        grub.enable = false;
        generic-extlinux-compatible.enable = true;
      };

      kernelParams = [
        "net.core.default_qdisc=fq_codel"
        "cpufreq.default_governor=performance"
      ];

      kernelModules = [
        "sch_cake"
        "act_mirred"
        "ifb"
        "tcp_bbr"
        "nf_conntrack"
      ];

      blacklistedKernelModules = [
        "brcmfmac" "brcmutil" "cfg80211"
        "snd_bcm2835" "snd" "soundcore"
        "ppp_generic" "ppp_async" "pppoe"
        "v4l2" "bcm2835_v4l2" "videobuf2"
      ];

      kernel.sysctl = {
        "net.ipv4.ip_forward" = 1;

        "net.ipv4.tcp_congestion_control" = "bbr";
        "net.ipv4.tcp_fastopen" = 3;
        "net.ipv4.tcp_tw_reuse" = 1;
        "net.ipv4.tcp_max_syn_backlog" = 4096;

        "net.core.rmem_max" = 16777216;
        "net.core.wmem_max" = 16777216;
        "net.core.rmem_default" = 1048576;
        "net.core.wmem_default" = 1048576;
        "net.ipv4.tcp_rmem" = "4096 1048576 16777216";
        "net.ipv4.tcp_wmem" = "4096 1048576 16777216";

        "net.ipv4.conf.all.send_redirects" = 0;
        "net.ipv4.conf.default.send_redirects" = 0;

        "net.core.netdev_max_backlog" = 5000;
        "net.core.somaxconn" = 4096;
        "net.core.default_qdisc" = "fq_codel";

        "net.ipv4.tcp_fin_timeout" = 30;
        "net.ipv4.tcp_mtu_probing" = 1;
        "net.ipv4.ip_local_port_range" = "10000 65535";

        "net.netfilter.nf_conntrack_max" = 131072;
        "net.netfilter.nf_conntrack_tcp_timeout_established" = 3600;
        "net.netfilter.nf_conntrack_tcp_timeout_time_wait" = 30;
        "net.netfilter.nf_conntrack_tcp_timeout_close_wait" = 15;
        "net.netfilter.nf_conntrack_udp_timeout" = 10;
        "net.netfilter.nf_conntrack_udp_timeout_stream" = 30;

        "net.core.rps_sock_flow_entries" = 32768;
      };
    };

    hardware.enableRedistributableFirmware = true;

    environment.systemPackages = with pkgs; [
      wireguard-tools
      iproute2
      nftables
      tcpdump
      htop
      dig
      ethtool
      usbutils
      nano
      libraspberrypi
    ];
  };
}

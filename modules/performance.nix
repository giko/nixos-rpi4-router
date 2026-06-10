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
      description = "Enable Receive Packet Steering onto CPUs 1-3 (CPU0 keeps the GICv2-pinned NIC IRQ work).";
    };
    threadedNapi = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Run NAPI polling in kernel threads. GICv2 on the RPi4 cannot move NIC hardirqs off CPU0, so without this all driver RX + GRO work is stuck there too.";
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
        for handle in $(nft -a list chain inet filter forward 2>/dev/null | awk '/ip protocol \{ tcp, udp \} ct state established flow add @f([[:space:]]|$)/ { for (i = 1; i <= NF; i++) if ($i == "handle") print $(i + 1) }'); do
          nft delete rule inet filter forward handle "$handle" 2>/dev/null || true
        done
        nft delete flowtable inet filter f 2>/dev/null || true
        nft add flowtable inet filter f { hook ingress priority 0\; devices = { ${lanIf}, ${wanIf} }\; counter\; }

        # The fastpath rule must sit BEFORE "ct state established,related accept"
        # (established packets stop evaluating there, so anything after it is dead
        # code) and AFTER any MAC-blacklist drops (so a flow established before a
        # MAC was blocked cannot keep flowing via the fastpath). "insert position
        # $h" inserts before the rule with handle $h. NEVER use "add rule ...
        # position 0": position takes a handle, 0 means "unset", and the rule is
        # silently appended after the chain's final drop where it never matches.
        ea=$(nft -a list chain inet filter forward | awk '/ct state established,related accept/ { for (i = 1; i <= NF; i++) if ($i == "handle") { print $(i + 1); exit } }')
        if [ -z "$ea" ]; then
          # Fail closed: without the anchor we cannot guarantee placement after
          # the MAC-blacklist drops, and guessing could let blocked-MAC flows
          # ride the fastpath past the drop rule.
          echo "no 'ct state established,related accept' rule in inet filter forward; refusing to place fastpath rule" >&2
          exit 1
        fi
        nft insert rule inet filter forward position "$ea" ip protocol { tcp, udp } ct state established flow add @f comment \"flow-offload-oneshot-owned\"
      '';
    };

    # nixos-rebuild reloads (not restarts) nftables on ruleset changes, which
    # wipes the runtime flowtable + fastpath rule without propagating to
    # flow-offload (partOf only follows stop/restart). RemainAfterExit keeps
    # flow-offload "active" the whole time, so service checks can't see it.
    # This watchdog re-asserts the fastpath within 2 minutes of any wipe.
    systemd.services.flow-offload-ensure = lib.mkIf pcfg.flowOffload {
      description = "Re-assert nftables flowtable if a ruleset reload wiped it";
      path = [ pkgs.nftables pkgs.gawk ];
      serviceConfig.Type = "oneshot";
      script = ''
        state=$(nft -a list chain inet filter forward 2>/dev/null | awk '
          /flow-offload-oneshot-owned/ { fo = NR }
          /ct state established,related accept/ { ea = NR }
          END { print ((fo && ea && fo < ea) ? "ok" : "broken") }')
        if [ "$state" != ok ] || ! nft list flowtable inet filter f >/dev/null 2>&1; then
          echo "flowtable or fastpath rule missing/misplaced; restarting flow-offload"
          ${config.systemd.package}/bin/systemctl restart flow-offload.service
        fi
      '';
    };
    systemd.timers.flow-offload-ensure = lib.mkIf pcfg.flowOffload {
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnBootSec = "3min";
        OnUnitActiveSec = "2min";
      };
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
          ${lib.optionalString pcfg.threadedNapi ''
          # NAPI polling (driver RX + GRO) as schedulable kthreads instead of
          # softirqs pinned to CPU0 (GICv2 cannot rebalance the hardirqs away).
          echo 1 > /sys/class/net/$iface/threaded 2>/dev/null || true
          ''}
          for rxq in /sys/class/net/$iface/queues/rx-*/rps_cpus; do
            # Mask e = CPUs 1-3. CPU0 already does all NIC hardirq work; with
            # mask f a flow can hash onto CPU0 and contend with it (measured
            # 86% of NET_RX on CPU0 under parallel load with mask f).
            [ -f "$rxq" ] && echo e > "$rxq" 2>/dev/null || true
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

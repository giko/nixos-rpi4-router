{ config, lib, ... }:
let
  cfg = config.router;

  tunnelOpts = { name, ... }: {
    options = {
      ip = lib.mkOption {
        type = lib.types.str;
        description = "Tunnel IP address with prefix (e.g., 10.2.0.2/32).";
      };
      privateKeyFile = lib.mkOption {
        type = lib.types.str;
        description = "Absolute path to WireGuard private key. Must be a string, NOT a Nix path.";
      };
      publicKey = lib.mkOption {
        type = lib.types.str;
        description = "Peer public key.";
      };
      endpoint = lib.mkOption {
        type = lib.types.str;
        description = "Peer endpoint (host:port).";
      };
      persistentKeepalive = lib.mkOption {
        type = lib.types.int;
        default = 25;
        description = "Keepalive interval in seconds.";
      };
    };
  };
in
{
  options.router.wireguard.tunnels = lib.mkOption {
    type = lib.types.attrsOf (lib.types.submodule tunnelOpts);
    default = {};
    description = "WireGuard VPN tunnels. Keys are tunnel names (e.g., wg_sw).";
  };

  config = lib.mkIf (cfg.enable && cfg.wireguard.tunnels != {}) {
    networking.wireguard.interfaces = lib.mapAttrs (name: tun: {
      ips = [ tun.ip ];
      privateKeyFile = tun.privateKeyFile;
      allowedIPsAsRoutes = false;
      peers = [{
        publicKey = tun.publicKey;
        endpoint = tun.endpoint;
        allowedIPs = [ "0.0.0.0/0" ];
        persistentKeepalive = tun.persistentKeepalive;
      }];
    }) cfg.wireguard.tunnels;
  };
}

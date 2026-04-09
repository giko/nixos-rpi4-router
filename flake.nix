{
  description = "NixOS router modules for Raspberry Pi 4 (2-port, WireGuard, AdGuard, QoS)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

  outputs = { self, nixpkgs, ... }: {
    nixosModules = {
      default     = import ./modules;
      kernel      = import ./modules/kernel.nix;
      interfaces  = import ./modules/interfaces.nix;
      wireguard   = import ./modules/wireguard.nix;
      nftables    = import ./modules/nftables.nix;
      pbr         = import ./modules/pbr.nix;
      dns         = import ./modules/dns.nix;
      dhcp        = import ./modules/dhcp.nix;
      qos         = import ./modules/qos.nix;
      upnp        = import ./modules/upnp.nix;
      performance = import ./modules/performance.nix;
      services    = import ./modules/services.nix;
    };
  };
}

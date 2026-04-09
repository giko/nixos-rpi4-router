{ config, lib, ... }:
{
  imports = [
    ./kernel.nix
    ./interfaces.nix
    ./wireguard.nix
    ./nftables.nix
    ./pbr.nix
    ./dns.nix
    ./dhcp.nix
    ./qos.nix
    ./upnp.nix
    ./performance.nix
    ./services.nix
  ];

  options.router.enable = lib.mkEnableOption "NixOS RPi4 router";
}

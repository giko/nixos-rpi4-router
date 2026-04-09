# Minimal working 2-port router config.
# Import the default module from this flake, then set these options.
{ config, pkgs, ... }:
{
  router = {
    enable = true;

    lan = {
      interface = "eth0";
      addresses = [{ address = "192.168.1.1"; prefixLength = 24; }];
    };
    wan.interface = "eth1";
  };

  users.users.root.openssh.authorizedKeys.keys = [
    "ssh-ed25519 AAAA..."  # Replace with your key
  ];
  networking.hostName = "router";
  system.stateVersion = "25.11";
}

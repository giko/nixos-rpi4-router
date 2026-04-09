# nixos-rpi4-router

NixOS modules for turning a Raspberry Pi 4 into a full-featured home router.

## Features

- **WireGuard VPN with policy-based routing** -- per-client or per-domain VPN steering via fwmark and ip rules
- **AdGuard Home DNS** -- DNS-over-TLS upstreams with DNSSEC, ad blocking, and local rewrites
- **QoS** -- CAKE on WAN egress, HTB + fq_codel on ingress (optimized for RPi4 CPU)
- **DHCP** -- dnsmasq with static leases and automatic gateway/DNS injection
- **UPnP** -- miniupnpd with configurable allow/deny rules
- **Performance tuning** -- flow offload, RPS, zram swap, CPU governor, kernel sysctl

## Quick Start

Create a `flake.nix` for your router:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    router.url = "github:nikitachukov/nixos-rpi4-router";
  };

  outputs = { nixpkgs, router, ... }: {
    nixosConfigurations.router = nixpkgs.lib.nixosSystem {
      system = "aarch64-linux";
      modules = [
        router.nixosModules.default
        ./configuration.nix
      ];
    };
  };
}
```

Then write a minimal `configuration.nix`:

```nix
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
```

## Deploy

```bash
nixos-rebuild switch --flake .#router --target-host root@192.168.1.1
```

## Examples

See [examples/minimal.nix](examples/minimal.nix) for a complete minimal configuration.

# Shared helpers used by nftables.nix and pbr.nix.
# Keeps fwmark/table assignment consistent across modules.
{ lib }:
let
  markBase = 65536; # 0x10000 in decimal — Nix has no hex literals
in
{
  # Reserved for forced-WAN routing (tunnel name "wan" in PBR options).
  wanMeta = {
    fwmark = markBase;
    fwmarkHex = "0x10000";
    fwmarkMask = "0xff0000";
    routingTable = 100;
  };

  # Given an attrset of tunnels, returns a sorted list of:
  # { name, fwmark, fwmarkHex, routingTable }
  # Sorted alphabetically by tunnel name; sequential assignment.
  tunnelMeta = tunnels:
    lib.imap1 (i: name: {
      inherit name;
      fwmark = (i + 1) * markBase;
      fwmarkHex = "0x${lib.toHexString ((i + 1) * markBase)}";
      routingTable = 100 + i;
    }) (builtins.sort builtins.lessThan (builtins.attrNames tunnels));

  # All tunnel meta including wan, for ip-rule generation.
  allMeta = tunnels:
    let
      wan = {
        name = "wan";
        fwmark = markBase;
        fwmarkHex = "0x10000";
        routingTable = 100;
      };
      tuns = lib.imap1 (i: name: {
        inherit name;
        fwmark = (i + 1) * markBase;
        fwmarkHex = "0x${lib.toHexString ((i + 1) * markBase)}";
        routingTable = 100 + i;
      }) (builtins.sort builtins.lessThan (builtins.attrNames tunnels));
    in [ wan ] ++ tuns;

  # Convert { address, prefixLength } to CIDR subnet string.
  # E.g., { address = "192.168.1.1"; prefixLength = 24; } -> "192.168.1.0/24"
  addrToSubnet = { address, prefixLength, ... }:
    let
      parts = lib.splitString "." address;
      octets = map lib.toInt parts;
      mask = if prefixLength == 24 then [ 255 255 255 0 ]
             else if prefixLength == 16 then [ 255 255 0 0 ]
             else if prefixLength == 8 then [ 255 0 0 0 ]
             else builtins.throw "addrToSubnet: only /8, /16, /24 supported, got /${toString prefixLength}";
      masked = lib.zipListsWith (o: m: toString (lib.bitAnd o m)) octets mask;
    in "${lib.concatStringsSep "." masked}/${toString prefixLength}";
}

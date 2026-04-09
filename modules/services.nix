{ config, lib, ... }:
let
  cfg = config.router;
  scfg = cfg.ssh;
in
{
  options.router.ssh = {
    enable = lib.mkOption { type = lib.types.bool; default = true; };
    permitRootLogin = lib.mkOption { type = lib.types.bool; default = true; };
    passwordAuth = lib.mkOption { type = lib.types.bool; default = true; };
    authorizedKeys = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "SSH authorized public keys for root.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.openssh = lib.mkIf scfg.enable {
      enable = true;
      settings = {
        PermitRootLogin = if scfg.permitRootLogin then "yes" else "no";
        PasswordAuthentication = scfg.passwordAuth;
      };
    };

    users.users.root.openssh.authorizedKeys.keys = lib.mkIf (scfg.authorizedKeys != []) scfg.authorizedKeys;

    services.chrony.enable = true;
    services.chrony.extraConfig = ''
      makestep 1 -1
    '';
  };
}

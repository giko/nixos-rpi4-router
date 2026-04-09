{ config, lib, ... }:
let
  cfg = config.router;
  dcfg = cfg.dns;

  # Build rewrites list: user rewrites + blocked domains as 0.0.0.0
  allRewrites =
    (map (r: { domain = r.domain; answer = r.answer; enabled = true; }) dcfg.rewrites)
    ++ (map (d: { domain = d; answer = "0.0.0.0"; enabled = true; }) dcfg.blockedDomains);
in
{
  options.router.dns = {
    upstreams = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "tls://dns.quad9.net" "tls://one.one.one.one" ];
      description = "DoT upstream DNS servers.";
    };
    bootstrap = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "9.9.9.9" "1.1.1.1" ];
      description = "Bootstrap DNS for resolving upstream hostnames.";
    };
    fallback = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "tls://dns.google" "tls://dns.mullvad.net" ];
      description = "Last-resort fallback DNS.";
    };
    rewrites = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          domain = lib.mkOption { type = lib.types.str; };
          answer = lib.mkOption { type = lib.types.str; };
        };
      });
      default = [];
      description = "Local DNS rewrites (.lan domains).";
    };
    blockedDomains = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ "mask.icloud.com" "mask-h2.icloud.com" "use-application-dns.net" ];
      description = "Domains sinkholed to 0.0.0.0.";
    };
    filters = lib.mkOption {
      type = lib.types.listOf (lib.types.submodule {
        options = {
          enabled = lib.mkOption { type = lib.types.bool; default = true; };
          url = lib.mkOption { type = lib.types.str; };
          name = lib.mkOption { type = lib.types.str; };
          id = lib.mkOption { type = lib.types.int; };
        };
      });
      default = [];
      description = "AdGuard filter/blocklist subscriptions.";
    };
    cacheSize = lib.mkOption {
      type = lib.types.int;
      default = 10000000;
      description = "DNS cache size in bytes.";
    };
    dnssec = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Enable DNSSEC validation.";
    };
    mutableSettings = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "If false, AdGuard config is fully declarative.";
    };
    queryLog = {
      enable = lib.mkOption { type = lib.types.bool; default = true; };
      interval = lib.mkOption { type = lib.types.str; default = "168h"; };
    };
    statistics = {
      enable = lib.mkOption { type = lib.types.bool; default = true; };
      interval = lib.mkOption { type = lib.types.str; default = "24h"; };
    };
  };

  config = lib.mkIf cfg.enable {
    services.adguardhome = {
      enable = true;
      mutableSettings = dcfg.mutableSettings;

      settings = {
        http.address = "0.0.0.0:3000";

        dns = {
          bind_hosts = [ "0.0.0.0" ];
          port = 53;
          refuse_any = true;
          blocked_hosts = [ "version.bind" "id.server" "hostname.bind" ];

          upstream_dns = dcfg.upstreams;
          bootstrap_dns = dcfg.bootstrap;
          upstream_mode = "parallel";
          fallback_dns = dcfg.fallback;
          enable_dnssec = dcfg.dnssec;

          cache_enabled = true;
          cache_size = dcfg.cacheSize;
          cache_ttl_min = 300;
          cache_ttl_max = 86400;
          handle_ddr = true;
          hostsfile_enabled = true;
          pending_requests.enabled = true;
        };

        querylog = {
          enabled = dcfg.queryLog.enable;
          file_enabled = dcfg.queryLog.enable;
          interval = dcfg.queryLog.interval;
          size_memory = 1000;
        };

        statistics = {
          enabled = dcfg.statistics.enable;
          interval = dcfg.statistics.interval;
        };

        filters = dcfg.filters;

        filtering = {
          blocking_mode = "default";
          filters_update_interval = 24;
          blocked_response_ttl = 10;
          filtering_enabled = true;
          rewrites_enabled = true;
          protection_enabled = true;
          rewrites = allRewrites;
        };
      };
    };
  };
}

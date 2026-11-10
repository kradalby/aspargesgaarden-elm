# NixOS module for asparges. Import via the flake's nixosModules.default.
#
#   services.asparges = {
#     enable = true;
#     baseURL = "https://beta.aspargesgaarden.no";
#     tailscaleKeyPath = config.age.secrets.asparges-tskey.path;
#   };
self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.services.asparges;
  stateRoot = "/var/lib/";
in
{
  options.services.asparges = {
    enable = lib.mkEnableOption "asparges, the aspargesgaarden.no web server";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "asparges flake package";
      description = "The asparges package to run. It must find vips on its own, as the flake's wrapped binary does.";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "asparges";
      description = "User to run as. A static user (not DynamicUser) so litestream can join its group to back up the database.";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "asparges";
      description = "Group owning the state directory and database.";
    };

    dataDir = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/asparges";
      description = "State directory: database, photos and the tsnet identity. Must live under /var/lib so systemd's StateDirectory manages it.";
    };

    hostname = lib.mkOption {
      type = lib.types.str;
      default = "asparges";
      description = "Tailnet hostname of the admin node (its own tsnet node).";
    };

    listenAddr = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:8090";
      description = "Local listener for the public site; point the nginx vhost here.";
    };

    baseURL = lib.mkOption {
      type = lib.types.strMatching "https?://[^/?#]+";
      default = "https://aspargesgaarden.no";
      description = "Public origin used for absolute links, without a trailing slash.";
    };

    noindex = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Keep the site out of search engines, for the beta site: every response
        carries X-Robots-Tag: noindex, robots.txt disallows everything and no
        IndexNow pings are sent. Turn off at cutover.
      '';
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [
        "debug"
        "info"
        "warn"
        "error"
      ];
      default = "info";
      description = "Log level of the service.";
    };

    umamiSiteId = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "168bd9f6-fec6-40dc-95b5-b0e4082c7a98";
      description = ''
        The site's id on the Umami server (umami.kradalby.no). Null puts no
        tracker on the pages and keeps Umami out of the CSP. The tracker
        reports only from the host of baseURL, so beta can have its own site.
      '';
    };

    tailscaleKeyPath = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = ''
        File holding the Tailscale auth key, read through LoadCredential so it
        need not be readable by the service user. Alternatively set TS_AUTHKEY
        in environmentFile.
      '';
    };

    environmentFile = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = ''
        Path to an EnvironmentFile sourced by the service, for secrets such as
        TS_AUTHKEY. Keep it out of the Nix store (e.g. an agenix/ragenix secret).
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = lib.hasPrefix stateRoot cfg.dataDir && cfg.dataDir != stateRoot;
        message = "services.asparges.dataDir must be a directory under ${stateRoot}, got ${cfg.dataDir}";
      }
    ];

    users.users.${cfg.user} = {
      isSystemUser = true;
      group = cfg.group;
    };
    users.groups.${cfg.group} = { };

    # import seeds the state directory: run it as the service user, e.g.
    # sudo -u asparges asparges import --data-dir /var/lib/asparges <checkout>.
    environment.systemPackages = [ cfg.package ];

    # The service creates the database 0660 and SQLite gives -wal and -shm
    # its mode; these put back what a restore or a seed copied in otherwise.
    systemd.tmpfiles.rules = map (f: "z ${cfg.dataDir}/${f} 0660 ${cfg.user} ${cfg.group} -") [
      "asparges.db"
      "asparges.db-wal"
      "asparges.db-shm"
    ];

    systemd.services.asparges = {
      description = "asparges, the aspargesgaarden.no web server";
      wantedBy = [ "multi-user.target" ];
      after = [
        "network-online.target"
        "nss-lookup.target"
      ];
      wants = [
        "network-online.target"
        "nss-lookup.target"
      ];

      serviceConfig = {
        # vips children count against this: two builds of a large photo plus
        # the server, with room left for the rest of the host.
        MemoryMax = lib.mkDefault "2G";

        ExecStart = lib.escapeShellArgs (
          [
            (lib.getExe cfg.package)
            "serve"
            "--log-level=${cfg.logLevel}"
            "--data-dir=${cfg.dataDir}"
            "--listen-addr=${cfg.listenAddr}"
            "--hostname=${cfg.hostname}"
            "--base-url=${cfg.baseURL}"
          ]
          # systemd expands %d inside the quoted argument.
          ++ lib.optional (cfg.tailscaleKeyPath != null) "--tailscale-auth-key-path=%d/tskey"
          ++ lib.optional cfg.noindex "--noindex"
          ++ lib.optional (cfg.umamiSiteId != null) "--umami-site-id=${cfg.umamiSiteId}"
        );

        User = cfg.user;
        Group = cfg.group;
        StateDirectory = lib.removePrefix stateRoot cfg.dataDir;
        # setgid so litestream-created -wal/-shm inherit the asparges group
        # (litestream joins it to replicate the db); UMask keeps the
        # database's 0660 on the files SQLite creates beside it.
        StateDirectoryMode = "2770";
        UMask = "0007";
        Restart = "always";
        RestartSec = "30s";

        # Graceful shutdown: SIGTERM to the main pid so it flushes and drains;
        # SIGKILL only mops up stragglers at the stop timeout.
        KillMode = "mixed";
        TimeoutStopSec = "30s";

        # tsnet is a Tailscale node: it needs CAP_NET_ADMIN to set the bypass
        # socket mark (SO_MARK), or it breaks on multi-homed hosts.
        AmbientCapabilities = [ "CAP_NET_ADMIN" ];
        CapabilityBoundingSet = [ "CAP_NET_ADMIN" ];

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        # AF_UNIX is required for tsnet's local API socket.
        RestrictAddressFamilies = [
          "AF_UNIX"
          "AF_INET"
          "AF_INET6"
          "AF_NETLINK"
        ];
        RestrictNamespaces = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        # vips decodes uploads: give a compromised decoder as little as the
        # server needs. Not ProcSubset=pid: tsnet reads /proc/net.
        ProtectKernelLogs = true;
        ProtectClock = true;
        ProtectHostname = true;
        ProtectProc = "invisible";
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        PrivateIPC = true;
        RemoveIPC = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = [ "@system-service" ];
        SystemCallErrorNumber = "EPERM";
      }
      // lib.optionalAttrs (cfg.tailscaleKeyPath != null) {
        LoadCredential = "tskey:${cfg.tailscaleKeyPath}";
      }
      // lib.optionalAttrs (cfg.environmentFile != null) {
        EnvironmentFile = cfg.environmentFile;
      };
    };
  };
}

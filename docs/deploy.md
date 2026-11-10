# Deploying asparges

How `asparges` runs in production, and the runbook for its first home: a
beta on `core.oracldn` at `https://beta.aspargesgaarden.no`, laid out exactly
as production will be, so going live later is renaming, not moving.

Each step is marked:

- **[you]**: needs the owner's credentials (tofu apply, secrets, colmena).
- **[PR]**: a change prepared for review in `infrastructure` or `dotfiles`.

The other repositories change; refer to files and patterns there, not to line
numbers.

## Shape

```
visitor ──▶ nginx vhost (TLS, ACME) ──▶ 127.0.0.1:<listenAddr>   public site
admin   ──▶ tailnet ──▶ tsnet node <hostname>:80/443            /admin/, /metrics, /debug
state: /var/lib/asparges/{asparges.db, originals/, variants/, tmp/, tsnet/}
```

- One systemd unit, `asparges.service`, from `services.asparges`
  (`nixosModules.default` of this flake). Static user and group `asparges`,
  `StateDirectory` with mode `2770`. The service creates `asparges.db`
  `0660` and SQLite gives `-wal` and `-shm` its mode (`UMask=0007` keeps
  it), so litestream (in the group) can replicate the database; the
  module's `z` tmpfiles rules put `0660 asparges:asparges` back on all
  three at boot and on every deploy. `MemoryMax=2G` covers the server and two
  vips children.
- The public listener is plain HTTP on loopback; nginx terminates TLS.
  Uploads go over the tailnet, never through nginx, so the vhost needs no
  body-size setting.
- The admin answers only on the tsnet node, and only to callers with the app
  capability `kradalby.no/cap/asparges` = `{"admin": true}`. Reaching the
  node is not enough.
- The service makes no backups and schedules no snapshots. litestream and
  restic, configured on the host, do.

|                      | Beta                              | Production (cutover)         |
| -------------------- | --------------------------------- | ---------------------------- |
| Host                 | `core.oracldn` (aarch64-linux)    | same, unless moved           |
| `baseURL`            | `https://beta.aspargesgaarden.no` | `https://aspargesgaarden.no` |
| `hostname` (tailnet) | `asparges-beta`                   | `asparges`                   |
| `noindex`            | `true`                            | `false`                      |
| `umamiSiteId`        | unset                             | the site's id                |
| `dataDir`            | `/var/lib/asparges`               | `/var/lib/asparges`          |

## Module options

Every `asparges serve` flag is an option (a test fails otherwise):

| Option             | Default                      |                                                                                                    |
| ------------------ | ---------------------------- | -------------------------------------------------------------------------------------------------- |
| `enable`           | `false`                      |                                                                                                    |
| `package`          | this flake's package         | Must find `vips`; the flake's binary carries it.                                                   |
| `user`, `group`    | `asparges`                   | Created by the module. Do not define them again.                                                   |
| `dataDir`          | `/var/lib/asparges`          | Must be under `/var/lib`.                                                                          |
| `listenAddr`       | `127.0.0.1:8090`             | Public site; the vhost proxies here.                                                               |
| `baseURL`          | `https://aspargesgaarden.no` | Canonical links, sitemap, share images. No trailing slash.                                         |
| `hostname`         | `asparges`                   | tsnet node name.                                                                                   |
| `tailscaleKeyPath` | `null`                       | Auth key file, read through `LoadCredential` (no owner needed). Used only while `tsnet/` is empty. |
| `environmentFile`  | `null`                       | Alternative: `TS_AUTHKEY=…` in an env file.                                                        |
| `noindex`          | `false`                      | `X-Robots-Tag: noindex`, `robots.txt` disallows all, IndexNow off.                                 |
| `umamiSiteId`      | `null`                       | Unset: no tracker and no Umami origin in the CSP.                                                  |
| `logLevel`         | `info`                       |                                                                                                    |

Worked example, the beta (a new file `machines/core.oracldn/asparges.nix` in
`dotfiles`):

```nix
{ config, ... }:
let
  cfg = config.services.asparges;
in
{
  age.secrets.asparges-tskey.file = ../../secrets/asparges-tskey.age;

  services.asparges = {
    enable = true;
    hostname = "asparges-beta";
    baseURL = "https://beta.aspargesgaarden.no";
    listenAddr = "127.0.0.1:63464";
    noindex = true;
    tailscaleKeyPath = config.age.secrets.asparges-tskey.path;
  };

  services.vhost."beta.aspargesgaarden.no".proxyPass = "http://${cfg.listenAddr}";
}
```

## Runbook: the beta

The order matters in three places: the tag and grant exist before the node
joins; the data is on the host before the first start; the auth key is
minted just before the deploy (single-use, 24 h).

### 1. Tailscale policy — [PR] in `infrastructure`, [you] apply

In `tailscale/policy.hujson`, following the `tag:hvor` and `tsnixcache`
entries:

- `tagOwners`: `"tag:asparges": ["kradalby@kradalby.no"]`.
- `groups`: `"group:asparges-admin"` with the owner, Kristine, Anders and
  Imma. No existing group is exactly these four (`group:family` has a fifth
  member and not the owner).
- The admin grant:

  ```hujson
  {
    "src": ["group:asparges-admin"],
    "dst": ["tag:asparges"],
    "ip":  ["tcp:80", "tcp:443"],
    "app": { "kradalby.no/cap/asparges": [{ "admin": true }] },
  },
  ```

- Monitoring: add `"tag:asparges"` to the `dst` of the "tsnet app nodes"
  grant from `tag:monitoring` (tcp:80, tcp:443, icmp), and to its ACL test.
- Tests: a reach test for one admin user to `tag:asparges:80`. ACL tests
  cannot check app capabilities; this repository's `vm-tsnet` test checks
  that the grant opens `/admin/` and that network reach alone gets `403`.

```bash
cd ~/git/infrastructure
tofu -chdir=tailscale plan     # only the ACL changes
tofu -chdir=tailscale apply
git commit -am "tailscale: state after asparges tag and grant"
```

Check that Kristine, Anders and Imma have logged in to the tailnet at least
once; the grant does nothing for an identity that does not exist yet.

### 2. DNS — [PR] in `infrastructure`, [you] apply

`beta.aspargesgaarden.no` already exists in `dns/aspargesgaarden_no.tf` as a
proxied CNAME to GitHub Pages. Change it, do not add a second one:

```hcl
"beta" = { value = "oracldn.fap.no", type = "CNAME", proxied = false }
```

Leave alone: the apex and `www` (still the live site on GitHub Pages), the
iCloud MX/SPF/verification records (not managed by tofu; do not import or
replace them), and the second zone `aspargesgården.no`.

```bash
tofu -chdir=dns plan     # must show exactly one change: beta
tofu -chdir=dns apply
git commit -am "dns: state after beta.aspargesgaarden.no to core.oracldn"
```

Then check that the ACME token in `dotfiles` (`secrets/cloudflare-token.age`,
used by `common/acme.nix` for DNS-01) may edit the `aspargesgaarden.no` zone.
If not, the vhost serves a self-signed placeholder until it can.

### 3. Host configuration — [PR] in `dotfiles`

Follow `hvor` and `ghdl`, which run the same way on the same host:

- `flake.nix`: the input, and the module in `core.oracldn`'s module list.

  ```nix
  asparges = {
    url = "github:kradalby/aspargesgaarden-elm/asparges";
    inputs.nixpkgs.follows = "nixpkgs-unstable";
    inputs.flake-utils.follows = "flake-utils";
  };
  # core.oracldn modules:
  asparges.nixosModules.default
  ```

  The module's `package` defaults to the flake's own build, so no overlay is
  needed. If the host's nixpkgs is older than this flake's lock and the
  build fails on the Go version, drop the `nixpkgs` follows.

- `machines/core.oracldn/asparges.nix`: the worked example above, imported
  from `machines/core.oracldn/default.nix`.
- `secrets/secrets.nix`: `"asparges-tskey.age".publicKeys = u ++ [ hosts.core-oracldn ];`
  (the `secrets-sync` check fails until the `.age` file exists, step 5).
- `machines/core.oracldn/litestream.nix`:
  - `{ name = "asparges.db"; path = "${config.services.asparges.dataDir}/asparges.db"; }`
    in `my.litestream.databases`. This also opts the database into the
    hourly `sqlite-backup` snapshot (`VACUUM INTO`, integrity check, xz into
    `/var/backup/sqlite`) and the weekly litestream restore test.
  - `"asparges"` in the litestream user's `extraGroups`.
  - `asparges.service` in litestream's `after`/`wants`, as for ghdl (the
    wal/shm permission race at boot). The module brings the `z` tmpfiles
    rules itself.
- `machines/core.oracldn/restic.nix`: `config.services.asparges.dataDir` in
  `paths`. The live `asparges.db`, `-wal` and `-shm` are excluded
  automatically (every litestream database is); restic takes the snapshot
  from `/var/backup/sqlite` instead, and `originals/`, `variants/` and
  `tsnet/` from the data directory. Built sizes are backed up on purpose, so
  a restore needs no rebuild.
- `machines/core.oracldn/monitoring.nix`:
  - `(scrapeJob "asparges-beta" [ "asparges-beta:80" ])` beside the ghdl job.
  - `"asparges-beta" = "core-oracldn";` in the VIP backing map.
  - `"https://beta.aspargesgaarden.no"` in the HTTPS probes.
  - Alerts, each with a promtool test in `checks/prometheus-rules/tests.yaml`:
    `increase(asparges_images_build_errors_total[1h]) > 0` and
    `asparges_images_queue_length > 0` for three hours (a full rebuild, after
    a new pipeline version or a restore without `variants/`, keeps the queue
    busy for more than one).

### 4. Seed data — [you], before the first deploy

The seed directory (`~/.local/share/asparges-seed`: the database with the
old site's events and gallery, edited, plus `originals/` and `variants/`)
goes to the host before the service ever starts:

- The `sqlite-backup` unit for `asparges.db` fails if the file is missing,
  and every restic job on `core.oracldn` requires every snapshot unit: no
  database, no backups for the whole host.
- Copied after a first start, `rsync --delete` would remove `tsnet/`, the
  node's identity; the single-use auth key is spent by then.

```bash
rsync -a --exclude '*-wal' --exclude '*-shm' --exclude 'tmp/' \
  ~/.local/share/asparges-seed/ root@core-oracldn:/var/lib/asparges/
ssh root@core-oracldn 'rm -f /var/lib/asparges/README.md'
```

The `asparges` user does not exist yet; systemd's `StateDirectory` gives the
directory to it on the first start. Never use `--delete` against a data
directory that has a `tsnet/` folder.

### 5. Auth key — [you], just before the deploy

From a `dotfiles` checkout, after step 1 is applied:

```bash
authkey kradalby -tags tag:asparges -secret asparges-tskey.age -rotate
git add secrets/asparges-tskey.age && git commit -m "secrets: add asparges-tskey"
```

The key is single-use and expires in 24 hours. It is read only while
`/var/lib/asparges/tsnet/` is empty; once the node has joined, the key file
no longer matters.

### 6. Deploy — [you]

Merge the `dotfiles` PR to `main` and push. `core.oracldn` is aarch64: wait
until the `cache-hosts` workflow is green for that commit (garnix does not
build it), then:

```bash
colmena apply dry-activate --on core.oracldn --nix-option narinfo-cache-negative-ttl 0
colmena apply --on core.oracldn
```

A `building '…asparges…'` line in the dry run is a cache miss; stop and let
CI build it rather than build under emulation.

### 7. Verify — the first real ARM deploy

The binary is built and tested on aarch64 in CI (`build`, `gotest` with the
real vips, and the unit's sandbox on a real kernel), but has never run on
this host. Check, in order:

```bash
ssh root@core-oracldn
systemctl status asparges litestream
journalctl -u asparges -b --no-pager | tail -50   # no "images: photo not prepared"
```

- [ ] The unit is active; `tailscale status` lists `asparges-beta` with
      `tag:asparges`.
- [ ] TLS: `curl -sI https://beta.aspargesgaarden.no/` returns `200` with a
      valid certificate and `X-Robots-Tag: noindex`;
      `curl -s https://beta.aspargesgaarden.no/robots.txt` disallows all.
- [ ] Old addresses: `curl -sI https://beta.aspargesgaarden.no/events` is a
      `301` to `/arrangementer`.
- [ ] Admin: `https://asparges-beta.dalby.ts.net/admin/` opens for each of
      the four admins ("Logget inn som …"); another tailnet user gets `403`.
- [ ] Monitoring: the `asparges-beta` target is up in Prometheus;
      `/admin/` from the monitoring host is `403`; the HTTPS probe is green.
- [ ] HTTPS on the tailnet node: `https://asparges-beta.dalby.ts.net/`
      gets a certificate (never tested outside the VM test, which uses
      plain HTTP).
- [ ] vips under the unit's hardening on ARM (`MemoryDenyWriteExecute`,
      `SystemCallFilter`): upload a large phone photo in the admin, crop it,
      and see "Bildene klargjøres …" finish;
      `asparges_images_build_errors_total` stays `0`. Note the build time on
      2 OCPU and the unit's memory peak
      (`systemctl show asparges -p MemoryPeak`) against `MemoryMax=2G`.
- [ ] Cold image request: after a restart, time the first request of a
      large variant and a second one:
      `curl -so /dev/null -w '%{time_total}\n' https://beta.aspargesgaarden.no/img/…`.
      There is no image cache in the process; the kernel's page cache is
      the cache.
- [ ] Backups: `systemctl start sqlite-backup-asparges.db` succeeds;
      `systemctl start litestream-restore-test` passes with `asparges.db` in
      it; the next restic run includes `/var/lib/asparges`.
- [ ] Kristine makes a test event, previews it, publishes it and deletes
      it. What confuses her becomes a fix.

## Backups and restore

| What                | How                                                                    | Restores             |
| ------------------- | ---------------------------------------------------------------------- | -------------------- |
| `asparges.db`       | litestream, continuous, to the garage bucket for `core.oracldn`        | `litestream restore` |
| `asparges.db`       | `sqlite-backup`, hourly safe copy to `/var/backup/sqlite`, then restic | `xz -dc` the archive |
| `/var/lib/asparges` | restic, hourly, to tjoda, ldn and Jotta (live db files excluded)       | `restic restore`     |

Restore: stop the service, put back `originals/` and `variants/` from restic
and the database from litestream or a `sqlite-backup` archive, fix
ownership, start:

```bash
systemctl stop asparges
# restic restore … --target / --include /var/lib/asparges
litestream restore -o /var/lib/asparges/asparges.db <replica of asparges.db>
chown -R asparges:asparges /var/lib/asparges
systemctl start asparges
```

Sizes missing from `variants/` (not restored, or a new pipeline version) are
built after start, one photo at a time; until then those pictures are
missing on the pages. `asparges_images_queue_length` returns to 0 when done.

A database and `originals/` alone are enough; `vm-basic` tests exactly that.
Keep `tsnet/` if it is restored; without it the node needs a new auth key
(step 5).

## Monitoring

Metrics are on the tsnet node at `http://<hostname>/metrics`, beside the
Go runtime's:

| Metric                                                                  |                                                  |
| ----------------------------------------------------------------------- | ------------------------------------------------ |
| `asparges_images_build_errors_total`                                    | Failed variant builds. Should stay 0.            |
| `asparges_images_queue_length`                                          | Photos waiting for their sizes; returns to 0.    |
| `asparges_images_variants_built_total`, `asparges_images_build_seconds` | Build count and time.                            |
| `asparges_bot_requests_total{agent,purpose}`                            | Search and AI crawlers.                          |
| `asparges_referrals_total{source}`                                      | Visits from search, AI answers and social sites. |
| `asparges_indexnow_submissions_total`                                   | IndexNow pings (off with `noindex`).             |

The host already alerts on a down target, the HTTPS probe, nginx, cert
expiry, litestream and `sqlite-backup`.

## Cutover of the main domain (later)

On the owner's word, once the beta has been used for real:

1. **[PR]** `dotfiles`: set `baseURL = "https://aspargesgaarden.no"`,
   `hostname = "asparges"`, `noindex = false`, and `umamiSiteId` to the
   site's id on umami.kradalby.no (look it up there, or add the site). Move
   the vhost to `aspargesgaarden.no`, same `listenAddr`. The binary answers
   every host name alike and redirects no hosts, so `www` and `beta` need
   nginx vhosts of their own that redirect to the main domain
   (`globalRedirect`; the `services.vhost` module has no option for it).
   Rename the scrape job, VIP backing and probe. The data directory stays
   where it is.
2. **[you]** Tailscale: the node keeps its identity in `tsnet/`, so a new
   `hostname` needs no new key. Check the name in `tailscale status` and
   rename the machine in the admin console if it kept the old one.
3. **[PR] / [you]** `infrastructure`: point the apex and `www` at
   `oracldn.fap.no`, DNS only; `tofu -chdir=dns apply`. Then remove the
   GitHub Pages wiring (`github/pages.tf`, `github/aspargesgaarden.tf`) and
   the Pages deploy workflow on `main`; merge `asparges` into `main`.
4. **[you]** Deploy, then check on the live domain: every old address
   `301`s to its Norwegian one; no `X-Robots-Tag`; `robots.txt` allows and
   names the sitemap; canonical links and the sitemap use the main domain.
5. **[you]** Submit `https://aspargesgaarden.no/sitemap.xml` in Google
   Search Console and Bing Webmaster Tools; run one event page through the
   Facebook Sharing Debugger.

The redirects from the old addresses are in the binary; nothing in nginx is
needed for them.

## Operations

```bash
journalctl -u asparges -f
```

Run `import` only into an empty data directory, with the service stopped.
Never start `serve --e2e-fixtures` against real data: its `/__reset` hook
deletes everything (the binary refuses it without `--dev`).

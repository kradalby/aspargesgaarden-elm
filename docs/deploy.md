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
admin   ──▶ tailnet ──▶ tsnet node <hostname>:80/443            /admin/, /static/, /img/, kra's /metrics, /debug, …
state: /var/lib/asparges/{asparges.db, originals/, variants/, tmp/, tsnet/, calendar.json}
```

- One systemd unit, `asparges.service`, from `services.asparges`
  (`nixosModules.default` of this flake). Static user and group `asparges`,
  `StateDirectory` with mode `2770`. The service makes `asparges.db` `0660`
  at every start and SQLite gives `-wal` and `-shm` its mode (`UMask=0007` keeps
  it), so litestream (in the group) can replicate the database; the
  module's `z` tmpfiles rules put `0660 asparges:asparges` back on all
  three at boot and on every deploy. `MemoryMax=2G` covers the server and two
  vips children.
- The public listener is plain HTTP on loopback; nginx terminates TLS.
  Uploads go over the tailnet, never through nginx, so the vhost needs no
  body-size setting.
- The admin answers only on the tsnet node, and only to callers with the app
  capability `kradalby.no/cap/asparges` = `{"admin": true}`. Reaching the
  node is not enough. Beside the admin, the node serves only `/static/` and
  `/img/` (what the app loads) and kra's own routes (`/metrics`, `/debug`,
  `/who`, `/quitquitquit`); every other path is `404` there.
- The service makes no backups and schedules no snapshots. litestream and
  restic, configured on the host, do.

|                      | Beta                              | Production (cutover)         |
| -------------------- | --------------------------------- | ---------------------------- |
| Host                 | `core.oracldn` (aarch64-linux)    | same, unless moved           |
| `baseURL`            | `https://beta.aspargesgaarden.no` | `https://aspargesgaarden.no` |
| `hostname` (tailnet) | `asparges-beta`                   | `asparges`                   |
| `noindex`            | `true`                            | `false`                      |
| `umamiSiteId`        | unset                             | the site's id                |
| `calendarURL`        | the farm's Google feed            | same                         |
| `dataDir`            | `/var/lib/asparges`               | `/var/lib/asparges`          |

## Module options

Every `asparges serve` flag is an option (a test fails otherwise):

| Option             | Default                      |                                                                                                                        |
| ------------------ | ---------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `enable`           | `false`                      |                                                                                                                        |
| `package`          | this flake's package         | Must find `vips`; the flake's binary carries it.                                                                       |
| `user`, `group`    | `asparges`                   | Created by the module. Do not define them again.                                                                       |
| `dataDir`          | `/var/lib/asparges`          | Must be under `/var/lib`.                                                                                              |
| `listenAddr`       | `127.0.0.1:8090`             | Public site; the vhost proxies here.                                                                                   |
| `baseURL`          | `https://aspargesgaarden.no` | Canonical links, sitemap, share images. No trailing slash.                                                             |
| `hostname`         | `asparges`                   | tsnet node name.                                                                                                       |
| `tailscaleKeyPath` | `null`                       | Auth key file, read through `LoadCredential` (no owner needed). Used only while `tsnet/` is empty.                     |
| `environmentFile`  | `null`                       | Alternative: `TS_AUTHKEY=…` in an env file.                                                                            |
| `noindex`          | `false`                      | `X-Robots-Tag: noindex`, `robots.txt` disallows all, IndexNow off.                                                     |
| `umamiSiteId`      | `null`                       | Unset: no tracker and no Umami origin in the CSP.                                                                      |
| `calendarURL`      | `null`                       | Public iCal feed of the booking calendar (Google's "public address in iCal format"). Unset: no calendar on `/kontakt`. |
| `logLevel`         | `info`                       |                                                                                                                        |

### The booking calendar

No calendar, and an alert, is better than a wrong one. The service reads
the feed every 5 minutes and keeps the last whole read, with its time, in
`calendar.json`. While that read is under 30 minutes old (six failed reads
in a row), `/kontakt` shows the calendar and `/llms.txt` the free weekends;
older, or never read, `/kontakt` says "Kalenderen er ikke tilgjengelig
akkurat nå. Ring … for å spørre om en dato." and `/llms.txt` leaves them
out. Both change the moment the limit is crossed, either way; a restart
judges the copy by the read time inside it. A tab left open shows the
same sentence once the page is 30 minutes old by the browser's own clock.

A read either takes the whole feed or fails; nothing is guessed:

- Refused (`result="refused"`), with the event named in the journal: no
  `END:VCALENDAR` or a cut-off body; an event whose start or end cannot be
  read, ends before it starts, or mixes a day with a time; a `TZID` the
  binary does not know; a time without a zone in a calendar without
  `X-WR-TIMEZONE`; a `RECURRENCE-ID` of no repeating event; a repeat it
  does not expand; a calendar with no bookings at all.
- Counted as bookings: every event not `STATUS:CANCELLED`, also one marked
  "free" (`TRANSP:TRANSPARENT`): wrongly taken is the safe side.
- Repeats, expanded exactly by the service's own code: every day, week,
  month or year, every n-th, on weekdays (the second Saturday, the last
  Friday), on days of the month or in given months, a number of times or
  until a day; deleted times (`EXDATE`), extra dates (`RDATE`), and single
  times moved or cancelled (`RECURRENCE-ID`). A time repeats on its own
  zone's clock (`TZID`, or UTC for a UTC start, as RFC 5545 says).
- Repeats that fail the read: `BYSETPOS`, `BYWEEKNO`, `BYYEARDAY`, hourly
  or finer, weekdays of a whole year without `BYMONTH`, `EXRULE`, `RDATE`
  periods, "this and following" changes (`RANGE`), a rule whose first
  time is not one of its own days (31 February), and a feed whose repeats
  would take more than 500 000 steps to walk.
  The journal names it (`calendar: read failed … event UID …: RRULE:…:
part BYSETPOS=-1`); changing that event in the calendar fixes it at the
  next read.

A feed with far fewer bookings than before is accepted: cancellations are
real, and a rule that waited for a person would itself fall out of step.
The counts are exported so an alert can ask.

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
    calendarURL = "https://calendar.google.com/calendar/ical/klatrerosen.no_hgf7kiksq8hrmilogq5nd6f4rs%40group.calendar.google.com/public/basic.ics";
  };

  services.vhost."beta.aspargesgaarden.no".proxyPass = "http://${cfg.listenAddr}";

  # The hourly snapshot opens the database as root. Run before the service,
  # it leaves root-owned -wal/-shm files the service and litestream cannot
  # write.
  systemd.services."sqlite-backup-asparges.db".after = [ "asparges.service" ];
}
```

## Runbook: the beta

The order:

1. `infrastructure`: merge, apply the Tailscale policy and auth key, then
   DNS.
2. `dotfiles`: merge, add the encrypted auth key, push once. The
   configuration of `core.oracldn` does not evaluate without
   `secrets/asparges-tskey.age`, so CI for that host is red on any push
   that lacks it.
3. The seed data goes to the host while CI builds, before the first start.
4. Deploy, verify.

### 1. Tailscale — [PR] in `infrastructure`, [you] apply

In `tailscale/policy.hujson`, beside the `ghdl` and `garnixlogs` entries:

- `tagOwners`: `"tag:asparges": ["kradalby@kradalby.no"]`.
- `groups`: `"group:asparges-admin"`: both of the owner's identities,
  Kristine, Anders and Imma. Not `group:family`: it has a member who does
  not edit the site, and not the owner.
- The admin grant:

  ```hujson
  {
    "src": ["group:asparges-admin"],
    "dst": ["tag:asparges"],
    "ip":  ["tcp:80", "tcp:443"],
    "app": { "kradalby.no/cap/asparges": [{ "admin": true }] },
  },
  ```

- Monitoring: a grant of its own from `tag:monitoring` to `tag:asparges` on
  `tcp:80` only, as for `ghdl` and `garnixlogs`: Prometheus reaches
  `/metrics` and gets `403` on `/admin/`.
- ACL tests: each admin reaches `tag:asparges` on 80 and 443; another user
  and `tag:server` do not; monitoring reaches 80 and not 443 or 22.
- ACL tests cannot see app capabilities. `tests/asparges_admin.py` (the
  flake check `asparges-admin`) checks that exactly one grant carries
  `kradalby.no/cap/asparges`, from `group:asparges-admin` to
  `tag:asparges`. This repository's `vm-tsnet` test checks that the grant
  opens `/admin/` and that reach alone gets `403`.

The auth key is `tailscale/authkey_asparges.tf`: a `tailscale_tailnet_key`
with `tags = ["tag:asparges"]`, single-use, preauthorized, after the ACL, and
the sensitive output `asparges_authkey`. It is valid 90 days.

```bash
cd ~/git/infrastructure
tofu -chdir=tailscale plan     # exactly: tailscale_acl.acl updated, tailscale_tailnet_key.asparges created
tofu -chdir=tailscale apply
git commit -am "tailscale: state after adding asparges"
```

Tailscale runs the ACL tests on upload; a failed test fails the apply and
changes nothing. Check that Kristine, Anders and Imma have each logged in to
the tailnet once; the grant does nothing for an identity that does not exist
yet.

### 2. DNS — [PR] in `infrastructure`, [you] apply

`beta.aspargesgaarden.no` already exists in `dns/aspargesgaarden_no.tf` as a
proxied CNAME to GitHub Pages. Change it, do not add a second one:

```hcl
"beta" = { value = "oracldn.fap.no", type = "CNAME", proxied = false }
```

Leave alone: the apex and `www` (still the live site on GitHub Pages),
`old`, the iCloud MX/SPF/verification records (not managed by tofu; do not
import or replace them), and the second zone `aspargesgården.no`.

```bash
tofu -chdir=dns plan     # exactly one change: cloudflare_dns_record.aspargesgaarden_no["beta"]
tofu -chdir=dns apply
git commit -am "dns: state after beta.aspargesgaarden.no to core.oracldn"
dig +short beta.aspargesgaarden.no   # oracldn.fap.no., then its address
```

Then check that the ACME token in `dotfiles` (`secrets/cloudflare-token.age`,
used by `common/acme.nix` for DNS-01) may edit the `aspargesgaarden.no` zone.
If not, the vhost serves a self-signed placeholder and the HTTPS probe fails
until the zone is added to the token.

### 3. Host configuration — [PR] in `dotfiles`

Follow `hvor` and `ghdl`, which run the same way on the same host:

- `flake.nix`: the input, and the module in `core.oracldn`'s module list.

  ```nix
  asparges = {
    url = "github:kradalby/aspargesgaarden-elm/asparges";
    inputs.nixpkgs.follows = "nixpkgs-unstable";
    inputs.flake-utils.follows = "flake-utils";
    inputs.headscale.follows = "headscale";   # only the VM test's control server
  };
  # the overlays, beside the other service inputs':
  asparges.overlays.default
  # core.oracldn modules:
  asparges.nixosModules.default
  ```

  The module's `package` defaults to the flake's own build either way. If
  the host's nixpkgs is older than this flake's lock and the build fails on
  the Go version, drop the `nixpkgs` follows.

- `machines/core.oracldn/asparges.nix`: the worked example above, imported
  from `machines/core.oracldn/default.nix`.
- `secrets/secrets.nix`: `"asparges-tskey.age".publicKeys = u ++ [ hosts.core-oracldn ];`
  (the `secrets-sync` check fails until the `.age` file exists, step 4).
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
  `paths`, and `tmp` and `.asparges.db-litestream` (litestream's shadow WAL)
  of it in `extraConfig.exclude`. The live `asparges.db`, `-wal` and `-shm`
  are excluded automatically (every litestream database is); restic takes
  the snapshot from `/var/backup/sqlite` instead, and `originals/`,
  `variants/` and `tsnet/` from the data directory. Built sizes are backed
  up on purpose, so a restore needs no rebuild. The restic source guard
  fails every job on the host while `/var/lib/asparges` is missing.
- `machines/core.oracldn/monitoring.nix`:
  - `(scrapeJob "asparges-beta" [ "asparges-beta:80" ])` beside the ghdl job.
  - `"asparges-beta" = "core-oracldn";` in the VIP backing map.
  - `"https://beta.aspargesgaarden.no"` in the HTTPS probes.
  - Alerts, each with a promtool test in `checks/prometheus-rules/tests.yaml`:
    `increase(asparges_images_build_errors_total[1h]) > 0`; the queue stuck,
    `asparges_images_queue_length > 0 unless on() increase(asparges_images_variants_built_total[1h]) > 0`
    for 15 minutes (a full rebuild keeps the queue up for longer than an
    hour while it builds); and the `absent(asparges_images_queue_length)`
    canary while the target is up.
- `machines/core.oracldn/grafana.nix`: copy
  `inputs.asparges.packages.${system}.grafanaDashboards`'s `asparges.json`
  into `dashboardDir`, as the tsnixcache one is.

### 4. Auth key — [you], before pushing `dotfiles`

On `main` of `dotfiles` with the step-3 PR merged, and the step-1 apply
done:

```bash
(cd ~/git/infrastructure && direnv exec . tofu -chdir=tailscale output -raw asparges_authkey) \
  | ragenix --editor=fake-editor --rules=secrets/secrets.nix --edit=secrets/asparges-tskey.age
git add secrets/asparges-tskey.age
git commit -m "secrets: add asparges-tskey"
git push
```

The key is single-use and read only while `/var/lib/asparges/tsnet/` is
empty; once the node has joined, the key file no longer matters. A failed
first start that left `tsnet/` empty needs no new key. A lost `tsnet/` does:
`tofu -chdir=tailscale apply -replace=tailscale_tailnet_key.asparges`, then
this step again.

### 5. Seed data — [you], before the first deploy

The seed directory (the database with the old site's events and gallery,
edited, plus `originals/` and `variants/`) goes to the host before the
service ever starts:

- The `sqlite-backup` unit for `asparges.db` fails if the file is missing,
  and every restic job on `core.oracldn` requires every snapshot unit: no
  database, no backups for the whole host.
- Copied after a first start, it would meet the service's own empty
  database and `tsnet/`, and the single-use key would be spent.

The seed directory is the one last refreshed with the binary being
deployed.
The copy leaves `-wal` and `-shm` behind, so fold the WAL into the database
first, then check it:

```bash
S=<the seed directory>
nix shell nixpkgs#sqlite -c sqlite3 $S/asparges.db 'PRAGMA wal_checkpoint(TRUNCATE)'
nix shell nixpkgs#sqlite -c sqlite3 $S/asparges.db 'PRAGMA integrity_check'   # ok
ssh root@core-oracldn 'id asparges; ls -d /var/lib/asparges'   # both missing; otherwise stop
```

```bash
rsync -a --chown=root:root --chmod=D2770,F0660 \
  --exclude '*-wal' --exclude '*-shm' --exclude 'tmp/' --exclude 'tsnet/' --exclude README.md \
  $S/ root@core-oracldn:/var/lib/asparges/
ssh root@core-oracldn 'find /var/lib/asparges -type f | wc -l'
find $S -type f ! -name '*-wal' ! -name '*-shm' ! -name README.md ! -path "$S/tmp/*" ! -path "$S/tsnet/*" | wc -l
```

The two counts match. The `asparges` user does not exist yet: the copy is
root's, and on the first start systemd's `StateDirectory` gives the whole
tree to `asparges`. Never `rsync --delete` into a data directory that has a
`tsnet/` folder.

### 6. Deploy — [you]

Both CI systems must be green for `HEAD` of `dotfiles`: garnix for x86, the
`cache-hosts` workflow for `core.oracldn` (aarch64; garnix does not build
it). Then, as the `dotfiles` deploy skill says:

```bash
ssh root@core-oracldn nixos-version --json | jq -r .configurationRevision
git log --oneline <that rev>..HEAD     # what lands besides asparges
colmena apply dry-activate --on core.oracldn --nix-option narinfo-cache-negative-ttl 0
```

A `building '…'` line in the dry run is a cache miss; stop and let CI build
it rather than build under emulation. The dry run starts `asparges.service`,
restarts `litestream` and reloads `nginx`. If `tailscaled` is not in its
restart list:

```bash
colmena apply --on core.oracldn --nix-option narinfo-cache-negative-ttl 0
```

If it is, a live switch would cut colmena's own connection: use
`colmena apply boot`, read the closure diff, reboot, as the deploy skill
describes.

### 7. Verify — the first real ARM deploy

The binary is built and tested on aarch64 in CI (`build`, `gotest` with the
real vips, and the unit's sandbox on a real kernel), but has never run on
this host. Check, in order:

```bash
ssh root@core-oracldn
systemctl status asparges litestream
journalctl -u asparges -b --no-pager | tail -50   # no "images: photo not prepared"
stat -c '%a %U:%G %n' /var/lib/asparges /var/lib/asparges/asparges.db*
journalctl -u litestream -b --no-pager | grep -i asparges   # no permission errors
curl -s http://asparges-beta/metrics | grep '^asparges_images_queue_length'   # 0 once the start check is done
```

The directory is `2770 asparges:asparges`; the database, `-wal` and `-shm`
are `660 asparges:asparges`. At start the worker checks every photo; with
the seed's `variants/` in place it only confirms them.

- [ ] `tailscale status` lists `asparges-beta` with `tag:asparges`.
- [ ] `systemctl status acme-beta.aspargesgaarden.no` succeeded (otherwise
      see the ACME token in step 2).
- [ ] TLS: `curl -sI https://beta.aspargesgaarden.no/` returns `200` with a
      valid certificate and `X-Robots-Tag: noindex`;
      `curl -s https://beta.aspargesgaarden.no/robots.txt` disallows all.
- [ ] Old addresses: `curl -sI https://beta.aspargesgaarden.no/events` is a
      `301` to `/arrangementer`.
- [ ] Admin: `https://asparges-beta.dalby.ts.net/admin/` opens for each of
      the admins ("Logget inn som …"); another tailnet user gets `403`.
- [ ] Monitoring: from the host, `http://asparges-beta/metrics` is `200`
      and `http://asparges-beta/admin/` is `403`; in Prometheus
      `up{job="asparges-beta"}` is 1 and the HTTPS probe is green.
- [ ] HTTPS on the tailnet node: `https://asparges-beta.dalby.ts.net/`
      gets a certificate (never tested outside the VM test, which uses
      plain HTTP).
- [ ] vips under the unit's hardening on ARM (`MemoryDenyWriteExecute`,
      `SystemCallFilter`): upload a large phone photo in the admin, crop it,
      and see "Klargjør bildet …" finish;
      `asparges_images_build_errors_total` stays `0`. Note the build time on
      2 OCPU and the unit's memory peak
      (`systemctl show asparges -p MemoryPeak`) against `MemoryMax=2G`.
- [ ] Backups: `systemctl start sqlite-backup-asparges.db` succeeds;
      `systemctl start litestream-restore-test` passes with `asparges.db` in
      it; the next `restic-backups-tjoda` run succeeds and includes
      `/var/lib/asparges`.
- [ ] Kristine makes a test event, previews it, publishes it and deletes
      it. What confuses her becomes a fix.

## Releases

A new release reaches the host through `dotfiles`: `nix flake update
asparges`, commit, push, wait for CI, deploy as in step 6.

- A release that bumps the image pipeline version (`images.PipelineVersion`)
  rebuilds every size of every photo at start: tens of minutes on
  `core.oracldn`, and until a photo's sizes exist its pictures are missing
  on the pages. Deploy it when nobody needs the site.
- A release that changes `db/schema.sql` migrates the database at start, and
  the previous binary then refuses to open it ("no update found for
  digest"). Take a snapshot before deploying it:
  `systemctl start sqlite-backup-asparges.db`. Rolling back across it means
  going back to that snapshot, losing what was saved since: stop the
  service, restore the snapshot as below, then deploy the previous release,
  which starts it.

## Backups and restore

| What                | How                                                                    | Restores             |
| ------------------- | ---------------------------------------------------------------------- | -------------------- |
| `asparges.db`       | litestream, continuous, to the garage bucket for `core.oracldn`        | `litestream restore` |
| `asparges.db`       | `sqlite-backup`, hourly safe copy to `/var/backup/sqlite`, then restic | `xz -dc` the archive |
| `/var/lib/asparges` | restic, hourly, to tjoda, ldn and Jotta (live db files excluded)       | `restic restore`     |

`/var/backup/sqlite` keeps one day of hourly archives; older ones are in
restic. The backup holds the built sizes, so a restore that brings back
`variants/` needs no rebuild. Without them, the service rebuilds every size
at start: tens of minutes on `core.oracldn`, with pictures missing on the
pages until their photo is done; `asparges_images_queue_length` is 0 when
it is. A database and `originals/` alone are enough; `vm-basic` tests that.

Every command below runs in a root shell on the host. They follow the
weekly `litestream-restore-test` unit and the tools' own help, but have not
been run against this service's real backups; the steps marked _untested_
least of all.

1. Stop both services and move the old database aside, with its `-wal`,
   `-shm` and litestream's state for it. A `-wal` left beside a restored
   database is replayed into it.

   ```bash
   systemctl stop asparges litestream
   mkdir -p /root/asparges-old
   mv /var/lib/asparges/asparges.db* /root/asparges-old/
   mv /var/lib/asparges/.asparges.db-litestream /root/asparges-old/   # untested: litestream starts afresh
   ```

   Stopping litestream pauses replication of every database on the host;
   keep it short.

2. The database, one of:
   - From litestream, the newest. The credentials are only in the unit's
     environment file:

     ```bash
     set -a; . /run/agenix/litestream; set +a
     litestream restore -config /etc/litestream.yml -integrity-check full /var/lib/asparges/asparges.db
     ```

   - From a `sqlite-backup` archive, a known-good hour:

     ```bash
     ls -t /var/backup/sqlite/asparges.db_backup_*.db.xz | head
     xz -dc /var/backup/sqlite/asparges.db_backup_<time>.db.xz > /var/lib/asparges/asparges.db
     ```

3. Photos and the node identity, only if lost, from restic (`restic-tjoda`,
   `restic-ldn` or `restic-jotta`):

   ```bash
   restic-tjoda snapshots --path /var/lib/asparges
   restic-tjoda restore <id> --target /root/asparges-restore \
     --include /var/lib/asparges/originals --include /var/lib/asparges/variants \
     --include /var/lib/asparges/tsnet
   rsync -a /root/asparges-restore/var/lib/asparges/ /var/lib/asparges/
   ```

   Without `tsnet/` the node needs a new auth key (step 4 of the runbook).

4. Ownership and mode, then start. Files a root shell made are root's and
   `0644`; litestream, in the group, needs `0660` to replicate. The service
   sets the database's mode at start too; the `chmod` keeps that from being
   the only guard.

   ```bash
   chown -R asparges:asparges /var/lib/asparges
   chmod 0660 /var/lib/asparges/asparges.db
   systemctl start asparges litestream
   journalctl -u litestream -n 50 --no-pager | grep -i asparges   # untested: no errors, replicating again
   ```

Keep `/root/asparges-old` until the site is right.

## Monitoring

Metrics are on the tsnet node at `http://<hostname>/metrics`, beside the
Go runtime's:

| Metric                                              |                                                                 |
| --------------------------------------------------- | --------------------------------------------------------------- |
| `asparges_images_build_errors_total{kind}`          | Builds of a photo's sizes of one kind that left a file unbuilt. |
| `asparges_images_queue_length`                      | Photos waiting for their sizes; returns to 0.                   |
| `asparges_images_variants_built_total{kind,format}` | Size files written.                                             |
| `asparges_images_build_seconds{kind}`               | Time to build one photo's sizes of one kind.                    |
| `asparges_bot_requests_total{agent,purpose}`        | Search and AI crawlers.                                         |
| `asparges_referrals_total{source}`                  | Visits from search engines and AI answers.                      |
| `asparges_indexnow_submissions_total{result}`       | IndexNow pings (off with `noindex`).                            |
| `asparges_http_requests_total{code}`                | Public requests answered, by status code.                       |
| `asparges_http_request_duration_seconds`            | Time to answer a public request.                                |
| `asparges_calendar_reads_total{result}`             | Reads of the booking calendar: `ok`, `fetch_failed`, `refused`. |
| `asparges_calendar_last_read_timestamp_seconds`     | Last whole read of the calendar in use; 0 is never.             |
| `asparges_calendar_shown`                           | 1 while `/kontakt` shows the calendar, 0 while it is hidden.    |
| `asparges_calendar_events`                          | Bookings (events not cancelled) in the calendar in use.         |
| `asparges_calendar_taken_dates`                     | Taken days in the calendar in use.                              |

The `asparges_calendar_*` gauges exist only where `calendarURL` is set.
Alerts for the calendar:

```promql
# warning: no good read for 15 minutes (three in a row failed)
time() - asparges_calendar_last_read_timestamp_seconds > 900
# critical: the calendar is hidden
asparges_calendar_shown == 0
# warning: refused content, not just Google down (the journal says which event)
increase(asparges_calendar_reads_total{result="refused"}[15m]) > 0
# warning: half the taken days gone since yesterday: cancellations, or a fault
asparges_calendar_taken_dates < 0.5 * max_over_time(asparges_calendar_taken_dates[1d])
# critical canary, on the host that sets calendarURL
absent(asparges_calendar_shown)
```

A photo that fails before any build (its original missing or unreadable)
counts in no metric; it shows as `images: photo not prepared` in the journal
and as failed in the admin.

The host already alerts on a down target, the HTTPS probe, nginx, cert
expiry, litestream and `sqlite-backup`.

The Grafana dashboard is generated by `cmd/dashboard` (`nix run .#dashboard`
prints it). Provision the flake package `grafanaDashboards`, a directory
holding `asparges.json`, into Grafana's dashboard directory. Its `Node`
variable picks the scrape job, `Site` the HTTPS probe.

## Cutover of the main domain (later)

On the owner's word, once the beta has been used for real:

1. **[you]** This repository: merge `asparges` into `main` (a fast-forward)
   and push. That also replaces the workflow that deploys the old site to
   GitHub Pages; the old site keeps being served from `gh-pages` until DNS
   moves.
2. **[PR]** `dotfiles`: point the `asparges` input at `main`
   (`github:kradalby/aspargesgaarden-elm`) and `nix flake update asparges`,
   so the `asparges` branch can go. Set
   `baseURL = "https://aspargesgaarden.no"`, `hostname = "asparges"`,
   `noindex = false`, and `umamiSiteId` to the site's id on
   umami.kradalby.no (look it up there, or add the site). Move the vhost to
   `aspargesgaarden.no`, same `listenAddr`. The binary answers every host
   name alike and redirects no hosts, so `www` and `beta` need nginx vhosts
   of their own that redirect to the main domain (`globalRedirect`; the
   `services.vhost` module has no option for it). Rename the scrape job, VIP
   backing, probe and the alerts' `job`. The data directory stays where it
   is. Deploy; the certificates come by DNS-01, before DNS moves.
3. **[you]** Tailscale: the node keeps its identity in `tsnet/`, so a new
   `hostname` needs no new key. Check the name in `tailscale status` and
   rename the machine in the admin console if it kept the old one.
4. **[PR] / [you]** `infrastructure`, DNS: point the apex and `www` of
   `aspargesgaarden.no` at `oracldn.fap.no`, not proxied. The plan
   (`tofu -chdir=dns plan`) shows exactly those two records changing; apply.
5. **[you]** Check on the live domain: every old address `301`s to its
   Norwegian one; no `X-Robots-Tag`; `robots.txt` allows and names the
   sitemap; canonical links and the sitemap use the main domain.
6. **[PR] / [you]** `infrastructure`, GitHub: remove only the
   `aspargesgaarden` entry from `local.pages` in `github/pages.tf`.
   `tofu -chdir=github plan` must show exactly one destroy,
   `github_repository_pages.this["aspargesgaarden"]`. Keep
   `github/aspargesgaarden.tf`: it holds the `github_repository` resource,
   and removing it destroys the repository. `aspargesgaarden-swift` and
   `old.aspargesgaarden.no` stay as they are.
7. **[you]** Submit `https://aspargesgaarden.no/sitemap.xml` in Google
   Search Console and Bing Webmaster Tools; run one event page through the
   Facebook Sharing Debugger.

The redirects from the old addresses are in the binary; nothing in nginx is
needed for them.

Open: the second zone, `aspargesgården.no` (`xn--aspargesgrden-xfb.no`),
apex and `www`, is a proxied CNAME to GitHub Pages, which serves only
`aspargesgaarden.no` for this repository. Either point it at
`oracldn.fap.no` with an nginx vhost that redirects to the main domain (the
ACME token must then cover that zone too), or delete its records.

## Operations

```bash
journalctl -u asparges -f
```

The seed is made on a laptop; the host never needs `asparges import`. If it
is ever run there, run it with the service stopped and as the service user:
`sudo -u asparges asparges import --data-dir /var/lib/asparges <checkout>`.
It leaves events already there and a non-empty gallery alone.

Never start `serve --e2e-fixtures` against real data: its `/__reset` hook
deletes everything (the binary refuses it without `--dev`).

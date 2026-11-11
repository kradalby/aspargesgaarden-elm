# Agents

`asparges` is one Go binary serving aspargesgaarden.no; see
[README.md](README.md) for layout, build, the local loop and tests. House
conventions (Go, Nix, Elm, git, services, secrets) are in
`/home/kradalby/git/dotfiles/docs/conventions`.

## Rules the code holds you to

- **HTML is elem-go, nothing else.** Pages compose components from `ui/`
  (one file per component, typed props); no template language, no HTML
  strings. A new piece of markup is a component, with a golden test.
- **One image API.** Every photo on a page is `ui.Picture`, fed an
  `images.Ref` that only `images/` makes. `TestPagesBuildNoRawImages`
  fails on a raw `<img>`. Each photo kind has one widest-box constant in
  `images/`; `TestLayoutsWithinSharpWidth` fails when a layout draws a kind
  wider.
- **Nothing is fetched at runtime.** Pages, CSS and JS load only from the
  binary, plus the origins in `ui/external.go` (Umami, the Google calendar
  frame), which also build the CSP header. Tests over the rendered pages,
  `site.css`, `gallery.js` and the admin's files fail on any other address.
  New JS libraries are pinned in the app's `yarn.lock` and bundled at build
  time, never linked from a CDN.
- **Every serve flag has a module option.** `TestEveryServeFlagIsInTheModule`
  fails when a flag of `asparges serve` is not passed by `nix/module.nix`;
  only `--dev` and `--e2e-fixtures` stay out. Document new options in
  `docs/deploy.md`.
- **Goldens are reviewed output.** `testdata/*.html` holds rendered markup,
  one tag per line, the JSON in it indented. Change on purpose with
  `UPDATE_GOLDEN=1 go test ./...`, which also rewrites
  `elm/admin/tests/ApiExamples.elm`, and read the diff before committing.
- **The dashboard charts what is exported.** `cmd/dashboard` generates it
  with the Grafana Foundation SDK; `TestQueriesNameExportedMetrics` fails
  when a panel queries a metric the service does not export. Renaming a
  metric means changing its panel, and the alerts in dotfiles, with it.
- **Old addresses keep working.** `TestAddresses` in `server/` lists every
  redirect; never remove a row.
- **A schema change migrates whole.** Change `db/schema.sql` with an update
  rule in `db/db.go` and keep the schema it replaces as
  `db/testdata/schema-<digest>.sql`; squibble checks columns only, and
  `TestMigratedEqualsFresh` fails when a check, key or index would not reach
  an existing database. An older binary refuses a newer schema, so a
  rollback across a schema change needs the database from before it.
- **Generated files stay in step.** `sqlc generate` after `db/` changes (the
  `generate` check catches drift), `go run ./cmd/vendorhash update` after
  `go.mod`, `elm2nix` after an `elm.json`, the `yarn.<app>` hash in
  `flakehashes.json` after a `yarn.lock`.
- **The admin API is a contract.** `docs/admin-api.md` describes it; change
  both together. `TestEveryCodeAndFieldIsDocumented` fails when the server
  can send a code or field the document does not name. Its first JSON
  example under the Photo, PhotoStatus, Photographer, Event and EventInput
  headings is part of the contract: `TestAdminJSONMatchesTheDocs` fails when
  the server's JSON has other keys, and the app's tests decode those
  examples from `elm/admin/tests/ApiExamples.elm`, which that test writes.

## Elm apps

Each app is `elm/<app>/` with its own `elm.json`, `src/<App>.elm`,
`elm-srcs.nix` and `registry.dat`, and optional `boot.js` (and
`package.json`/`yarn.lock` if it bundles npm packages). Adding one is adding
its name to `elmApps` in `flake.nix`: the build, the `elm-test-<app>` check
and the Tailwind class scan follow. Pages load it from `/static/<app>.js`;
no inline script (CSP `script-src 'self'`).

## Checks before you hand work back

- `go test ./...` (after `go generate ./static`), golangci-lint at zero
  findings, `nix fmt`. Never hand-edit Go imports: `goimports -w`.
- `nix flake check` for anything touching Nix, the module or the build.
- The browser suites when you touched what they cover (README, "Browser
  suites"): `nix develop -c e2e/admin/run.sh`, `nix develop -c e2e/gallery/run.sh`.
- Commits: `area: imperative summary`, lowercase, no trailing period; hooks
  on, never `--no-verify`. No push unless asked.
- Comments say why, never data that ages. Documents state facts and
  commands, not history.

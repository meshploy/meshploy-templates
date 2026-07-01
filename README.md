# meshploy-templates

The community catalog of one-click application templates for
[Meshploy](https://github.com/meshploy/meshploy). Each template is a standard
`docker-compose.yml` (+ `x-meshploy` blocks) plus a `meta.yaml` describing
catalog metadata and deploy-time variables. A Meshploy install fetches this
catalog and deploys a template as a normal **stack** (templates are a stack
*source*, not a separate runtime).

> Design & format reference: `internal-docs/plans/one-click-templates.md`.

## Layout

```
templates/<id>/
├── meta.yaml           # catalog metadata + variable declarations
├── docker-compose.yml  # standard compose + x-meshploy blocks
└── logo.svg            # icon

index.json              # GENERATED — flat catalog a Meshploy install fetches (do not hand-edit)
tools/build-index/      # Go generator + validator (also the CI gate)
```

## The catalog index (`index.json`)

`index.json` is a generated, flat list of every template's manifest — the file a
Meshploy install fetches to render the gallery. **Never edit it by hand.** It is
regenerated from the `templates/` tree by CI on every push to `main`, and the
same tool gates PRs. A Meshploy install prefers this file (one request); if it
is absent it falls back to discovering templates via the GitHub API.

```bash
go run ./tools/build-index          # validate + (re)write index.json
go run ./tools/build-index --check  # validate only; fail if index.json is stale (CI on PRs)
```

## Adding a template

1. Create `templates/<id>/` — the directory name is the immutable `id`.
2. Write `meta.yaml` (see `templates/pgadmin/meta.yaml`) and a
   `docker-compose.yml` with `${VAR}` placeholders that map to the declared
   variables.
3. Pin image tags (no `:latest`).
4. Run `go run ./tools/build-index` and commit the updated `index.json` (or let
   CI regenerate it on merge).
5. Open a PR — CI validates the manifest, the variable/placeholder mapping, and
   pinned image tags.

## Variables

Prompted variables (`prompt:`) are asked at deploy time. Generated variables
(`generate:`) are auto-filled. A variable is exactly one of `prompt:` or
`generate:`.

| Generator   | Produces                                                        |
|-------------|-----------------------------------------------------------------|
| `password`  | 24-char alphanumeric                                            |
| `secret64`  | 64-char base64url                                               |
| `hex32`     | 32-char hex                                                     |
| `uuid`      | a v4 UUID                                                       |
| `subdomain` | `<id>-<rand>.<org base domain>`; registers a route via `expose` |

A `subdomain` variable carries an `expose: { service, port }` block naming the
web-facing service. It drives **routing**, so it does not need to appear as a
`${VAR}` in the compose (unlike env variables, which must).

## Catalog

| id | name | category |
|----|------|----------|
| `pgadmin` | pgAdmin 4 | database |

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
```

## Adding a template

1. Create `templates/<id>/` — the directory name is the immutable `id`.
2. Write `meta.yaml` (see `templates/pgadmin/meta.yaml`) and a
   `docker-compose.yml` with `${VAR}` placeholders that map to the declared
   variables.
3. Pin image tags (no `:latest`).
4. Open a PR — CI validates the manifest and the variable/placeholder mapping.

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

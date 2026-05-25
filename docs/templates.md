# Templates

One-click apps are YAML files describing an image, env, ports, volumes, and
a form for the user. Bundled templates live in
`internal/templates/bundled/*.yaml`; remote templates are merged from URLs
configured in Settings (remote wins on ID collision).

## Schema

```yaml
id: postgres                  # unique slug; users install by ID
name: PostgreSQL              # display name
version: "16"                 # display string only
description: Object-relational database.
icon: postgres                # optional, used by the UI

fields:                       # form rendered in the install dialog
  - key: POSTGRES_PASSWORD
    label: Password
    type: password            # password | string | number | bool
    required: true
  - key: POSTGRES_DB
    label: Default database
    default: app

deploy:                       # rendered with Go text/template against {fields, app_name}
  image: docker.io/library/postgres:16
  env:
    POSTGRES_PASSWORD: "{{.POSTGRES_PASSWORD}}"
    POSTGRES_DB: "{{.POSTGRES_DB}}"
  volumes:
    - container: /var/lib/postgresql/data
      host: "~/BasePodData/{{.app_name}}/pgdata"
  ports:
    - 5432
  internal_only: true         # not exposed via Caddy — reachable only inside basepod network
  healthcheck:
    cmd: ["pg_isready", "-U", "postgres"]
```

## Rendering pipeline

1. User picks a template + app name and fills the form.
2. Server validates required fields, applies defaults.
3. The `deploy` block is rendered through `text/template` with the user
   fields plus an automatic `app_name` variable.
4. The rendered struct is converted into an `apps.CreateInput` and the app
   is created with `deploy_strategy=stop_start` and `internal_only` from the
   template.
5. The image is pulled and a deploy is queued in the background.

## Best practices

- Default `internal_only: true` for databases and stateful services — they
  should be reached from other apps via Podman DNS (`my-db:5432`), not the
  public internet.
- Set a `healthcheck.cmd` whenever possible so blue/green can validate.
- Use `host:` paths under `~/BasePodData/{{.app_name}}/` so the user can back
  the data up with Time Machine.
- Pin image tags to a specific version, not `latest`, for reproducibility.

## Adding a new bundled template

1. Create `internal/templates/bundled/<id>.yaml`
2. Rebuild the server (`go:embed` picks up the new file)
3. The template appears in `GET /api/v1/templates` and the Templates page.

## Remote templates

Set the `template_sources` setting to a JSON array of HTTPS URLs that return
a YAML index. (Implementation detail: the registry fetches every URL on a
1-hour cache and merges by ID over the bundled set.)

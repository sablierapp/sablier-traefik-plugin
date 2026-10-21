# Pangolin Example

This example runs [Sablier](https://github.com/sablierapp/sablier) behind
[Pangolin](https://github.com/fosrl/pangolin), waking a container on demand from
a real Pangolin resource.

Pangolin is not a reverse proxy of its own — it drives a Traefik v3 instance
whose dynamic configuration it generates from its own database. So there is no
Pangolin-specific plugin: this is the regular
[Traefik Sablier plugin](../../README.md), loaded next to Pangolin's own
`badger` plugin.

What is Pangolin-specific is **how the middleware gets attached to a resource**,
which is what this example is really about.

## What is in the stack

| Service | Role |
| --- | --- |
| `pangolin` | Serves Traefik's dynamic configuration from its database |
| `traefik` | The actual proxy, with the `badger` and `sablier` plugins loaded |
| `middleware-manager` | Attaches the Sablier middleware to one Pangolin resource |
| `sablier` | Starts and stops `mimic` on demand, via the Docker socket |
| `mimic` | The on-demand workload, labelled `sablier.enable=true` |

Traefik reads from two dynamic providers at once:

- the **HTTP provider** — pointed at `middleware-manager:3456/api/v1/traefik-config`,
  which re-serves Pangolin's generated routers with per-resource middleware
  attached. Point it back at `pangolin:3001/api/v1/traefik-config` to bypass
  Middleware Manager entirely;
- the **file provider** (`config/traefik/dynamic_config.yml`) — where the
  Sablier middleware is declared.

No `gerbil`/`newt` tunnel is involved: `mimic` runs on the same Docker host as
Traefik, so Pangolin addresses it through a **Local** site.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Ports `80`, `8080` and `3456` free on your machine

### Running the example

```bash
docker compose up -d
./setup.sh
```

`setup.sh` creates the Pangolin server admin, an org, a Local site and an HTTP
resource for `mimic` through Pangolin's API — everything you would otherwise
click through in the dashboard (see [By hand](#by-hand-instead-of-setupsh)).

Pangolin's Traefik polls every 5 seconds, so give it a moment, then:

```bash
curl -H 'Host: mimic.localhost' http://127.0.0.1/
```

The first request returns the Sablier waiting page while the container starts:

```html
<title>Sablier</title>
... Mimic ... Starting ...
```

Once `mimic` reports healthy, the same request reaches the app:

```console
$ curl -H 'Host: mimic.localhost' http://127.0.0.1/
Mimic says hello!
```

After a minute without traffic (`sessionDuration: 1m`) Sablier stops the
container again, and the next request starts the cycle over.

To use a browser instead of `curl`, add the hosts to `/etc/hosts`:

```
127.0.0.1 mimic.localhost pangolin.localhost
```

Then open <http://mimic.localhost>. The Pangolin dashboard is at
<http://pangolin.localhost> (`admin@example.com` / `Password123!`), and the
Traefik dashboard at <http://127.0.0.1:8080>.

## How the middleware is attached

Pangolin builds every HTTP router's middleware chain as
`[badger, ...traefik.additional_middlewares]`. It has **no per-resource
middleware field**, so attaching Sablier to one resource takes one of three
approaches.

### Option A — Middleware Manager (used here)

[Middleware Manager](https://github.com/hhftechnology/middleware-manager) is a
community service, listed in
[Pangolin's own docs](https://docs.pangolin.net/self-host/community-guides/middlewaremanager),
that reads resources from Pangolin's internal API and re-serves Traefik's
dynamic configuration with middlewares attached per resource. Traefik's HTTP
provider points at it instead of at Pangolin.

The Sablier middleware stays declared in the file provider
(`config/traefik/dynamic_config.yml`) and is attached as an *external*
middleware — which is what `setup.sh` does:

```bash
curl -X POST http://127.0.0.1:3456/api/resources/<id>/external-middlewares \
  -H 'Content-Type: application/json' \
  -d '{"middleware_name":"sablier-mimic@file","priority":100,"provider":"file"}'
```

You can do the same from its UI at <http://127.0.0.1:3456>. The result:

```console
$ curl -s http://127.0.0.1:8080/api/http/routers | jq '.[] | select(.provider=="http")'
{
  "name": "1-mimic-router@http",
  "rule": "Host(`mimic.localhost`)",
  "middlewares": ["sablier-mimic@file", "badger@http"],
  "service": "1-mimic-service"
}
```

> [!IMPORTANT]
> Note the order: Middleware Manager always places its own additions **before**
> the router's existing middlewares, so Sablier runs *before* `badger`. The
> `priority` field only orders Middleware Manager's own assignments among
> themselves — it cannot move one after `badger`.
>
> That means anyone who can reach the hostname can wake the container without
> authenticating. For a public demo like this one that is harmless, but if you
> need authentication to happen first, use Option B or Option C below.

### Option B — `additional_middlewares`

Pangolin's own knob, no extra service. Reference the file-provider middleware
from `config/config.yml`:

```yaml
traefik:
    additional_middlewares:
        - sablier-mimic@file
```

The `@file` suffix is required: the middleware lives in the file provider while
Pangolin's routers live in the `http` provider. This produces
`["badger@http", "sablier-mimic@file"]` — authentication first.

To use it in this example, put that block back in `config/config.yml` and point
Traefik's HTTP provider back at Pangolin.

> [!WARNING]
> `additional_middlewares` is **global** — it is appended to *every* HTTP
> resource in the install, and therefore routes all of them through this one
> Sablier group. Fine for a single resource, but it does not scale to a second
> app. Pangolin's dashboard routers are declared in `dynamic_config.yml` and are
> not affected.

### Option C — shadow the router by hand

Per-resource *and* authentication-first, at the cost of hand-maintained config.
Declare a higher-priority router in the file provider that reuses Pangolin's
generated service:

```yaml
http:
  routers:
    mimic-sablier:
      rule: "Host(`mimic.localhost`)"
      priority: 1000              # Pangolin's routers default to 100
      entryPoints:
        - web
      middlewares:
        - badger@http             # authentication first
        - sablier-mimic
      service: "1-mimic-service@http"
```

The generated names follow `<resourceId>-<resource name>-router|service`, but
rather than guessing them, copy the real router out of Traefik's API (the
command above) and edit it — that also gets the `tls` block right on a real
HTTPS install. Re-check it after changing the resource's domain, path rules or
TLS settings in Pangolin.

## By hand, instead of `setup.sh`

1. Open <http://pangolin.localhost> and complete the initial setup. The setup
   token is printed in the logs: `docker compose logs pangolin | grep -A1 'SETUP TOKEN'`.
2. Create an organisation.
3. Create a site of type **Local** — no tunnel is needed, because the workload
   runs on the same Docker host as Traefik.
4. Create an HTTP resource with subdomain `mimic` on the `localhost` domain, and
   turn **SSL off** (this example has no certificate resolver).
5. Add a target: site `local-site`, `http`, host `mimic`, port `80`.

`setup.sh` turns authentication off so the demo needs no login. If you turn it
back on, mind the middleware ordering described above: under Middleware Manager
Sablier runs *before* `badger`, so the container can be woken by an
unauthenticated request.

## Notes

- **Health checks.** `mimic` ships one, which is how Sablier knows when it is
  ready to serve. On the Pangolin side, leave *target* health checks off for
  Sablier-managed targets: Pangolin drops targets it considers unhealthy from
  the generated config, and a stopped container is exactly that.
- **badger version.** This example pins `v1.5.0`; `v1.6.x` and `v1.7.0`
  currently fail Traefik's plugin integrity check. Use whichever version your
  own Pangolin install shipped with.
- **Middleware Manager must keep running.** Traefik's dynamic configuration is
  served by it, so if the container stops, Traefik loses the whole
  Pangolin-generated config, not just the Sablier attachment.
- **Tunnelled setups.** When your containers sit behind a Newt tunnel, Sablier
  runs on the site (it needs the Docker socket) while Traefik runs on the VPS,
  so `sablierUrl` has to be reachable across the tunnel. See the
  [Pangolin guide](https://sablierapp.dev/tutorials/reverse-proxies/pangolin/)
  for the ways to arrange that.

## Stopping the example

```bash
docker compose down -v
rm -rf config/db config/logs config/key   # Pangolin's runtime state
rm -rf data config/middleware-manager     # Middleware Manager's state
```

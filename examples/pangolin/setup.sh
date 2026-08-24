#!/usr/bin/env bash
# Bootstraps the Pangolin install so the example is usable without clicking
# through the UI: creates the server admin, an org, a Local site, and an HTTP
# resource pointing at the `mimic` container, then attaches the Sablier
# middleware to that one resource through Middleware Manager.
#
# Everything this does is also doable from the dashboard — see README.md.
# Pinned to the Pangolin version in compose.yml; the API may change in others.
set -euo pipefail

BASE="http://127.0.0.1"
HOST="pangolin.localhost"
EMAIL="admin@example.com"
PASSWORD="Password123!"

api() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-s -X "$method" -H "Host: ${HOST}" -H "Content-Type: application/json"
              -H "X-CSRF-Token: x-csrf-protection")
  [ -n "${SESSION:-}" ] && args+=(-H "Cookie: p_session_token=${SESSION}")
  [ -n "$body" ] && args+=(-d "$body")
  curl "${args[@]}" "${BASE}${path}"
}

echo "==> Waiting for Pangolin"
until curl -sf -H "Host: ${HOST}" "${BASE}/api/v1/" >/dev/null 2>&1; do sleep 2; done

echo "==> Creating server admin (${EMAIL})"
TOKEN=$(docker compose logs pangolin 2>&1 | grep -A1 'SETUP TOKEN GENERATED' \
        | grep 'Token:' | tail -1 | awk '{print $NF}' | tr -d '\r')
if [ -z "$TOKEN" ]; then
  echo "No setup token found in the pangolin logs — already initialised?" >&2
else
  api PUT /api/v1/auth/set-server-admin \
    "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\",\"setupToken\":\"${TOKEN}\"}" >/dev/null
fi

echo "==> Logging in"
SESSION=$(curl -s -D - -o /dev/null -X POST -H "Host: ${HOST}" \
  -H "Content-Type: application/json" -H "X-CSRF-Token: x-csrf-protection" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" \
  "${BASE}/api/v1/auth/login" \
  | grep -i 'set-cookie' | sed 's/.*p_session_token=\([^;]*\).*/\1/' | tr -d '\r')
[ -n "$SESSION" ] || { echo "Login failed" >&2; exit 1; }

echo "==> Creating org, Local site and resource"
api PUT /api/v1/org \
  '{"orgId":"demo","name":"Demo","subnet":"100.90.128.0/24","utilitySubnet":"100.91.128.0/24"}' >/dev/null
api PUT /api/v1/org/demo/site '{"name":"local-site","type":"local"}' >/dev/null
api PUT /api/v1/org/demo/resource \
  '{"name":"mimic","subdomain":"mimic","domainId":"domain1","mode":"http"}' >/dev/null
# ssl:false because this example has no certificate resolver.
# sso:false makes the resource public so the demo needs no login — a real
# deployment leaves Pangolin's authentication on, and badger simply runs
# before the Sablier middleware.
api POST /api/v1/resource/1 '{"ssl":false,"sso":false}' >/dev/null
api PUT /api/v1/resource/1/target \
  '{"siteId":1,"ip":"mimic","port":80,"method":"http","enabled":true}' >/dev/null

echo "==> Attaching the Sablier middleware to the resource"
# Middleware Manager discovers resources from Pangolin's internal API. Wait for
# it to pick ours up, then attach the middleware declared in
# config/traefik/dynamic_config.yml as an "external" middleware (it lives in
# Traefik's file provider, hence the @file suffix).
# Middleware Manager serves its management API under /api; /api/v1 is only the
# Traefik config endpoint.
MM="http://127.0.0.1:3456/api"
for _ in $(seq 1 30); do
  RESOURCE_ID=$(curl -s "${MM}/resources" \
    | python3 -c "import sys,json
try: d=json.load(sys.stdin)
except Exception: raise SystemExit
rs=d.get('resources',d) if isinstance(d,dict) else d
for r in (rs or []):
    if 'mimic' in str(r.get('host','')): print(r['id']); break" 2>/dev/null)
  [ -n "${RESOURCE_ID:-}" ] && break
  sleep 2
done

if [ -z "${RESOURCE_ID:-}" ]; then
  echo "Middleware Manager has not discovered the resource yet." >&2
  echo "Attach 'sablier-mimic@file' by hand at http://127.0.0.1:3456" >&2
else
  curl -s -X POST -H "Content-Type: application/json" \
    -d '{"middleware_name":"sablier-mimic@file","priority":100,"provider":"file"}' \
    "${MM}/resources/${RESOURCE_ID}/external-middlewares" >/dev/null
  echo "    attached sablier-mimic@file to resource ${RESOURCE_ID}"
fi

echo
echo "Done. Traefik polls every 5s, then:"
echo "  curl -H 'Host: mimic.localhost' http://127.0.0.1/"
echo "  or add '127.0.0.1 mimic.localhost pangolin.localhost' to /etc/hosts"
echo "  and open http://mimic.localhost in a browser."
echo
echo "Pangolin dashboard:   http://pangolin.localhost  (${EMAIL} / ${PASSWORD})"
echo "Middleware Manager:   http://127.0.0.1:3456"

#!/usr/bin/env bash
set -euo pipefail

profile="${1:-smoke}"
compose_file="${COMPOSE_FILE:-loadtest/docker-compose.yml}"
host_url="${BASE_URL_HOST:-http://localhost:8080}"
reports_dir="${REPORTS_DIR:-loadtest/reports}"
seed_file_host="${SEED_FILE_HOST:-loadtest/seed/counterparties.json}"
seed_file_k6="${SEED_FILE:-/seed/counterparties.json}"
scenarios="${SCENARIOS:-post_document list_query catalog_crud}"

case "$profile" in
  smoke)
    : "${SEED_COUNTERPARTIES:=60}"
    : "${SEED_DOCUMENTS:=80}"
    : "${POST_RAMP_1:=5s}" "${POST_HOLD_1:=10s}" "${POST_RAMP_2:=5s}" "${POST_HOLD_2:=10s}" "${POST_RAMP_DOWN:=5s}"
    : "${POST_TARGET_1:=4}" "${POST_TARGET_2:=8}" "${POST_P95_MS:=1500}"
    : "${POST_SLEEP:=0.05}"
    : "${LIST_RAMP_1:=5s}" "${LIST_RAMP_2:=10s}" "${LIST_HOLD_2:=10s}" "${LIST_RAMP_DOWN:=5s}"
    : "${LIST_START_RPS:=5}" "${LIST_TARGET_1:=15}" "${LIST_TARGET_2:=30}" "${LIST_PREALLOCATED_VUS:=20}" "${LIST_MAX_VUS:=80}"
    : "${CATALOG_VUS:=5}" "${CATALOG_DURATION:=20s}" "${CATALOG_P95_MS:=800}"
    ;;
  validation)
    : "${SEED_COUNTERPARTIES:=200}"
    : "${SEED_DOCUMENTS:=500}"
    ;;
  *)
    echo "usage: $0 [smoke|validation]" >&2
    exit 2
    ;;
esac

export POST_RAMP_1 POST_HOLD_1 POST_RAMP_2 POST_HOLD_2 POST_RAMP_DOWN
export POST_TARGET_1 POST_TARGET_2 POST_P95_MS POST_ERROR_RATE POST_SLEEP
export LIST_RAMP_1 LIST_RAMP_2 LIST_HOLD_2 LIST_RAMP_DOWN
export LIST_START_RPS LIST_TARGET_1 LIST_TARGET_2 LIST_PREALLOCATED_VUS LIST_MAX_VUS
export LIST_LIMIT LIST_OFFSET_PAGES LIST_P95_MS LIST_P99_MS LIST_ERROR_RATE
export CATALOG_VUS CATALOG_DURATION CATALOG_LIST_LIMIT CATALOG_P95_MS CATALOG_ERROR_RATE

mkdir -p "$reports_dir" "$(dirname "$seed_file_host")"

if [[ "${RESET_STACK:-0}" == "1" ]]; then
  echo "==> resetting PostgreSQL loadtest stack"
  docker compose -f "$compose_file" down -v
fi

echo "==> starting PostgreSQL loadtest stack ($profile)"
services=(postgres onebase prometheus)
if [[ "${START_GRAFANA:-0}" == "1" ]]; then
  services+=(grafana)
fi
docker compose -f "$compose_file" up -d --build "${services[@]}"

echo "==> waiting for onebase health"
ready=0
for _ in $(seq 1 60); do
  if curl -fsS "$host_url/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
if [[ "$ready" != "1" ]]; then
  echo "onebase did not become healthy at $host_url/health" >&2
  docker compose -f "$compose_file" ps
  docker compose -f "$compose_file" logs --tail=120 onebase
  exit 1
fi

echo "==> seeding $SEED_COUNTERPARTIES counterparties and $SEED_DOCUMENTS documents"
go run ./loadtest/seed \
  -url "$host_url" \
  -counterparties "$SEED_COUNTERPARTIES" \
  -documents "$SEED_DOCUMENTS" \
  -out "$seed_file_host"

run_k6() {
  local name="$1"
  local script="$2"
  local report="/reports/${name}-${profile}.html"
  echo "==> k6 $name"
  docker compose -f "$compose_file" run --rm --service-ports \
    -e BASE_URL=http://onebase:8080 \
    -e SEED_FILE="$seed_file_k6" \
    -e OB_SESSION_COOKIE \
    -e K6_WEB_DASHBOARD="${K6_WEB_DASHBOARD:-false}" \
    -e K6_WEB_DASHBOARD_HOST=0.0.0.0 \
    -e K6_WEB_DASHBOARD_EXPORT="$report" \
    -e POST_RAMP_1 -e POST_HOLD_1 -e POST_RAMP_2 -e POST_HOLD_2 -e POST_RAMP_DOWN \
    -e POST_TARGET_1 -e POST_TARGET_2 -e POST_P95_MS -e POST_ERROR_RATE -e POST_SLEEP \
    -e LIST_RAMP_1 -e LIST_RAMP_2 -e LIST_HOLD_2 -e LIST_RAMP_DOWN \
    -e LIST_START_RPS -e LIST_TARGET_1 -e LIST_TARGET_2 -e LIST_PREALLOCATED_VUS -e LIST_MAX_VUS \
    -e LIST_LIMIT -e LIST_OFFSET_PAGES -e LIST_P95_MS -e LIST_P99_MS -e LIST_ERROR_RATE \
    -e CATALOG_VUS -e CATALOG_DURATION -e CATALOG_LIST_LIMIT -e CATALOG_P95_MS -e CATALOG_ERROR_RATE \
    k6 run -o experimental-prometheus-rw "/scripts/scenarios/$script"
  echo "    report: ${reports_dir}/${name}-${profile}.html"
}

for scenario in $scenarios; do
  case "$scenario" in
    post_document) run_k6 post_document post_document.js ;;
    list_query) run_k6 list_query list_query.js ;;
    catalog_crud) run_k6 catalog_crud catalog_crud.js ;;
    *) echo "unknown scenario in SCENARIOS: $scenario" >&2; exit 2 ;;
  esac
done

echo "==> Prometheus summary"
python3 - <<'PY'
import json
import urllib.parse
import urllib.request

queries = [
    "max_over_time(onebase_db_pool_acquired_conns[30m])",
    "max_over_time(onebase_db_pool_max_conns[30m])",
    "increase(onebase_db_pool_empty_acquire_total[30m])",
    "increase(onebase_limited_operation_total[30m])",
    "increase(onebase_slow_operation_total[30m])",
]

for q in queries:
    url = "http://localhost:9090/api/v1/query?query=" + urllib.parse.quote(q)
    try:
        with urllib.request.urlopen(url, timeout=5) as resp:
            payload = json.load(resp)
    except Exception as exc:
        print(f"{q}: {exc}")
        continue
    result = payload.get("data", {}).get("result", [])
    if not result:
        print(f"{q}: no data")
        continue
    values = [r.get("value", [None, ""])[1] for r in result]
    print(f"{q}: {', '.join(values[:5])}")
PY

if [[ "${CLEANUP:-0}" == "1" ]]; then
  echo "==> cleanup"
  docker compose -f "$compose_file" down -v
else
  echo "==> stack left running; cleanup with: docker compose -f $compose_file down -v"
fi

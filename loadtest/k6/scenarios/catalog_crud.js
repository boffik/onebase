// Сценарий CRUD справочника: список + создание контрагента. Нагружает лёгкий
// путь (без проведения) — полезно сравнивать с post_document.js, чтобы отделить
// стоимость HTTP/аутентификации/сериализации от стоимости проведения.
//
// Запуск:
//   k6 run -e BASE_URL=http://localhost:8080 loadtest/k6/scenarios/catalog_crud.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { u, CATALOG_COUNTERPARTY, GET_HEADERS, createCounterparty } from '../lib/common.js';
import { envDuration, envFloat, envInt } from '../lib/env.js';

const CATALOG_LIST_LIMIT = envInt('CATALOG_LIST_LIMIT', 100);

export const options = {
  scenarios: {
    crud: {
      executor: 'constant-vus',
      vus: envInt('CATALOG_VUS', 30),
      duration: envDuration('CATALOG_DURATION', '2m'),
    },
  },
  thresholds: {
    http_req_duration: [`p(95)<${envInt('CATALOG_P95_MS', 300)}`],
    http_req_failed: [`rate<${envFloat('CATALOG_ERROR_RATE', 0.01)}`],
  },
};

export default function () {
  // 70% чтений, 30% записей — типичный профиль справочника.
  if (Math.random() < 0.7) {
    const res = http.get(u(`/catalogs/${CATALOG_COUNTERPARTY}?limit=${CATALOG_LIST_LIMIT}&offset=0&sort=${encodeURIComponent('Наименование')}&dir=asc`), GET_HEADERS);
    check(res, {
      'список 200': (r) => r.status === 200,
      'есть X-Limit': (r) => r.headers['X-Limit'] !== undefined,
    });
  } else {
    createCounterparty(`${__VU}-${__ITER}`);
  }
  sleep(0.1);
}

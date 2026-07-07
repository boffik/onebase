// Сценарий list-запросов: чтение списков справочника и документов с пагинацией.
// Моделирует «листание» пользователем — read-heavy нагрузка на индексы и
// сериализацию. Перед запуском засейте данные (seed), иначе списки пустые.
//
// Запуск:
//   k6 run -e BASE_URL=http://localhost:8080 loadtest/k6/scenarios/list_query.js

import http from 'k6/http';
import { check } from 'k6';
import { u, CATALOG_COUNTERPARTY, DOCUMENT_POSTING, GET_HEADERS } from '../lib/common.js';
import { envDuration, envFloat, envInt } from '../lib/env.js';

const LIST_LIMIT = envInt('LIST_LIMIT', 100);
const LIST_OFFSET_PAGES = envInt('LIST_OFFSET_PAGES', 10);

export const options = {
  scenarios: {
    listing: {
      executor: 'ramping-arrival-rate',
      startRate: envInt('LIST_START_RPS', 20),
      timeUnit: '1s',
      preAllocatedVUs: envInt('LIST_PREALLOCATED_VUS', 50),
      maxVUs: envInt('LIST_MAX_VUS', 200),
      stages: [
        { duration: envDuration('LIST_RAMP_1', '30s'), target: envInt('LIST_TARGET_1', 50) },
        { duration: envDuration('LIST_RAMP_2', '1m'), target: envInt('LIST_TARGET_2', 200) },
        { duration: envDuration('LIST_HOLD_2', '30s'), target: envInt('LIST_TARGET_2', 200) },
        { duration: envDuration('LIST_RAMP_DOWN', '20s'), target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: [`p(95)<${envInt('LIST_P95_MS', 500)}`, `p(99)<${envInt('LIST_P99_MS', 1500)}`],
    http_req_failed: [`rate<${envFloat('LIST_ERROR_RATE', 0.01)}`],
  },
};

export default function () {
  const dir = Math.random() < 0.5 ? 'asc' : 'desc';
  const offset = Math.floor(Math.random() * LIST_OFFSET_PAGES) * LIST_LIMIT;
  const page = `limit=${LIST_LIMIT}&offset=${offset}`;
  let res;
  if (Math.random() < 0.5) {
    res = http.get(u(`/catalogs/${CATALOG_COUNTERPARTY}?${page}&sort=${encodeURIComponent('Наименование')}&dir=${dir}`), GET_HEADERS);
  } else {
    res = http.get(u(`/documents/${DOCUMENT_POSTING}?${page}&sort=${encodeURIComponent('Дата')}&dir=${dir}`), GET_HEADERS);
  }
  check(res, {
    'список 200': (r) => r.status === 200,
    'есть X-Limit': (r) => r.headers['X-Limit'] !== undefined,
    'есть X-Offset': (r) => r.headers['X-Offset'] !== undefined,
    'есть X-Total-Count': (r) => r.headers['X-Total-Count'] !== undefined,
  });
}

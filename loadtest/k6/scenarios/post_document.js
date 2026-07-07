// Главный сценарий: создание и проведение документа поступления — самая тяжёлая
// и репрезентативная операция onebase (OnPost на DSL + движения регистра в
// транзакции). Ссылается на заранее засеянных контрагентов (seed-файл); если
// файла нет — создаёт одного контрагента в setup().
//
// Запуск:
//   k6 run -e BASE_URL=http://localhost:8080 \
//          -e SEED_FILE=../../seed/counterparties.json \
//          loadtest/k6/scenarios/post_document.js

import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { createCounterparty, postReceipt } from '../lib/common.js';
import { envDuration, envFloat, envInt } from '../lib/env.js';

// Загружаем id контрагентов из seed-файла. open() — только в init-контексте,
// поэтому оборачиваем в try/catch: без файла упадём на fallback в setup().
const SEED_FILE = __ENV.SEED_FILE || '../../seed/counterparties.json';
const POST_SLEEP = envFloat('POST_SLEEP', 0);
const seeded = new SharedArray('counterparties', function () {
  try {
    return JSON.parse(open(SEED_FILE));
  } catch (_) {
    return [];
  }
});

export const options = {
  scenarios: {
    posting: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: envDuration('POST_RAMP_1', '30s'), target: envInt('POST_TARGET_1', 20) },
        { duration: envDuration('POST_HOLD_1', '1m'), target: envInt('POST_TARGET_1', 20) },
        { duration: envDuration('POST_RAMP_2', '30s'), target: envInt('POST_TARGET_2', 50) },
        { duration: envDuration('POST_HOLD_2', '1m'), target: envInt('POST_TARGET_2', 50) },
        { duration: envDuration('POST_RAMP_DOWN', '20s'), target: 0 },
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    // SLA: 95% проведений быстрее 800 мс, ошибок < 1%.
    http_req_duration: [`p(95)<${envInt('POST_P95_MS', 800)}`],
    http_req_failed: [`rate<${envFloat('POST_ERROR_RATE', 0.01)}`],
  },
};

export function setup() {
  if (seeded.length > 0) return { ids: null };
  // Fallback без сидинга: создаём одного контрагента на весь прогон.
  const id = createCounterparty('fallback');
  return { ids: id ? [id] : [] };
}

export default function (data) {
  const pool = seeded.length > 0 ? seeded : (data.ids || []);
  if (pool.length === 0) return;
  const cp = pool[Math.floor(Math.random() * pool.length)];
  const res = postReceipt(cp, __ITER);
  check(res, {
    'проведение 200': (r) => r.status === 200,
    'есть id в ответе': (r) => {
      try { return !!r.json('id'); } catch (_) { return false; }
    },
  });
  if (POST_SLEEP > 0) {
    sleep(POST_SLEEP);
  }
}

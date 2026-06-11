// Главный сценарий: создание и проведение документа поступления — самая тяжёлая
// и репрезентативная операция onebase (OnPost на DSL + движения регистра в
// транзакции). Ссылается на заранее засеянных контрагентов (seed-файл); если
// файла нет — создаёт одного контрагента в setup().
//
// Запуск:
//   k6 run -e BASE_URL=http://localhost:8080 \
//          -e SEED_FILE=../../seed/counterparties.json \
//          loadtest/k6/scenarios/post_document.js

import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { createCounterparty, postReceipt } from '../lib/common.js';

// Загружаем id контрагентов из seed-файла. open() — только в init-контексте,
// поэтому оборачиваем в try/catch: без файла упадём на fallback в setup().
const SEED_FILE = __ENV.SEED_FILE || '../../seed/counterparties.json';
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
        { duration: '30s', target: 20 },  // разгон
        { duration: '1m', target: 20 },   // плато
        { duration: '30s', target: 50 },  // ступень выше — ищем предел
        { duration: '1m', target: 50 },
        { duration: '20s', target: 0 },   // остывание
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    // SLA: 95% проведений быстрее 800 мс, ошибок < 1%.
    http_req_duration: ['p(95)<800'],
    http_req_failed: ['rate<0.01'],
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
}

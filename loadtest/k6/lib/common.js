// Общие хелперы k6-сценариев onebase.
//
// Аутентификация: проще всего гонять нагрузку по базе БЕЗ пользователей —
// тогда onebase пускает анонимно и токен не нужен. Если в базе есть юзеры,
// передайте сессионный токен через переменную окружения OB_TK — он будет
// добавляться к каждому запросу как ?_tk=… (см. internal/auth/middleware.go).

import http from 'k6/http';
import { check } from 'k6';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.OB_TK || '';

// u строит абсолютный URL и при необходимости дописывает токен сессии.
export function u(path) {
  const full = `${BASE_URL}${path}`;
  if (!TOKEN) return full;
  return full + (path.includes('?') ? '&' : '?') + '_tk=' + encodeURIComponent(TOKEN);
}

// Имена сущностей эталонной конфигурации examples/simple-erp. Под другой конфиг
// поменяйте здесь — в URL пойдёт encodeURIComponent (кириллица допустима).
export const CATALOG_COUNTERPARTY = encodeURIComponent('Контрагент');
export const DOCUMENT_POSTING = encodeURIComponent('Поступление');

export const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

// createCounterparty создаёт контрагента, возвращает id или null.
export function createCounterparty(suffix) {
  const body = JSON.stringify({
    'Наименование': `ООО Контрагент ${suffix}`,
    'ИНН': `77${String(suffix).padStart(8, '0')}`,
  });
  const res = http.post(u(`/catalogs/${CATALOG_COUNTERPARTY}`), body, JSON_HEADERS);
  check(res, { 'контрагент создан (200)': (r) => r.status === 200 });
  try {
    return res.json('id');
  } catch (_) {
    return null;
  }
}

// postReceipt создаёт и проводит документ поступления на указанного контрагента
// одним вызовом (__action=post). Это самый тяжёлый, репрезентативный путь:
// OnPost (DSL) + движения регистра в транзакции.
export function postReceipt(counterpartyID, itemIdx) {
  const qty = 1 + (itemIdx % 20);
  const price = 10 + (itemIdx % 990);
  const body = JSON.stringify({
    'Дата': new Date().toISOString().slice(0, 10),
    'Поставщик': counterpartyID,
    '__tableparts': {
      'Товары': [{
        'Номенклатура': `Товар ${itemIdx % 50}`,
        'Количество': qty,
        'Цена': price,
        'Сумма': qty * price,
      }],
    },
    '__action': 'post',
  });
  return http.post(u(`/documents/${DOCUMENT_POSTING}`), body, JSON_HEADERS);
}

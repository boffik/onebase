// Service worker onebase (этап 45). Консервативная стратегия, чтобы ничего не
// сломать: кэшируется ТОЛЬКО статика (vendor-ассеты, иконки, manifest, оболочка
// офлайн-страницы). HTML-страницы /ui/* НЕ кэшируются — они под авторизацией и с
// живыми данными, иначе был бы риск показать устаревшую/чужую страницу.
//
// Стратегии (см. fetch ниже):
//  - /icons/*, /vendor/*, manifest — cache-first (отдаём из кэша, при промахе
//    тянем из сети и кэшируем только ok-ответы);
//  - навигация                     — network-first с фолбэком на /offline.html.
//
// Сброс кэша: имя CACHE содержит ревизию сборки — onebase подставляет её в
// __OB_CACHE__ при отдаче /sw.js (см. internal/ui/pwa.go). Значит каждый релиз
// автоматически меняет имя кэша → старые кэши чистятся в activate, а vendor-
// ассеты (их URL НЕ версионируются) перетягиваются заново. Ручной правки не
// требуется. Сам /sw.js отдаётся с Cache-Control: no-cache, поэтому браузер
// замечает смену байт и переустанавливает воркер.

const CACHE = '__OB_CACHE__';

// Минимальная оболочка для офлайна. vendor/echarts грузится лениво дашбордом —
// не предкэшируем его жёстко, чтобы install не зависел от его пути.
const PRECACHE = [
  '/offline.html',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
  '/manifest.webmanifest',
];

self.addEventListener('install', (event) => {
  // Кэшируем поэлементно (не addAll): сбой одного ассета не должен лишать нас
  // остальных — в частности /offline.html должен закэшироваться независимо от
  // иконок/манифеста, иначе офлайн-фолбэк молча перестал бы работать.
  event.waitUntil(
    caches.open(CACHE)
      .then((cache) => Promise.allSettled(
        PRECACHE.map((u) => fetch(u).then((res) => {
          if (res && res.ok) return cache.put(u, res);
        }))
      ))
      .then(() => self.skipWaiting())
      .catch(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

function isStatic(url) {
  return url.pathname.startsWith('/vendor/')
    || url.pathname.startsWith('/icons/')
    || url.pathname === '/manifest.webmanifest';
}

// cacheFirst — отдаём из кэша, при промахе идём в сеть и кэшируем (только ok-
// ответы, чтобы не закэшировать 401/redirect/ошибку).
function cacheFirst(req) {
  return caches.match(req).then((hit) => hit || fetch(req).then((res) => {
    if (res && res.ok) {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
    }
    return res;
  }));
}

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return; // POST и пр. — только сеть, не перехватываем

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // сторонние домены не трогаем

  if (isStatic(url)) {
    event.respondWith(cacheFirst(req));
    return;
  }

  // Навигация (HTML-страницы) — network-first, фолбэк на офлайн-заглушку.
  // Ничего не кэшируем (живые данные + авторизация). Response.error() — чтобы
  // respondWith никогда не получил undefined (если /offline.html не закэширован).
  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req).catch(() => caches.match('/offline.html').then((r) => r || Response.error()))
    );
    return;
  }

  // Остальное (XHR/fetch к /ui/messages и т.п.) — просто сеть.
});

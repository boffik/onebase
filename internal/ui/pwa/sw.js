// Service worker onebase (этап 45). Консервативная стратегия, чтобы ничего не
// сломать: кэшируется ТОЛЬКО статика (vendor-ассеты, иконки, manifest, оболочка
// офлайн-страницы). HTML-страницы /ui/* НЕ кэшируются — они под авторизацией и с
// живыми данными, иначе был бы риск показать устаревшую/чужую страницу.
//
// Чтобы выкатить обновление SW — поднимите версию CACHE: старые кэши удаляются
// в activate. Сам файл /sw.js отдаётся сервером с Cache-Control: no-cache.

const CACHE = 'onebase-v1';

// Минимальная оболочка для офлайна. vendor/echarts грузится лениво дашбордом —
// не предкэшируем его жёстко, чтобы install не падал, если путь изменится.
const PRECACHE = [
  '/offline.html',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
  '/manifest.webmanifest',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE)
      .then((c) => c.addAll(PRECACHE))
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

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return; // POST и пр. — только сеть, не перехватываем

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // сторонние домены не трогаем

  // Статика — cache-first (версионируется в URL у vendor; иконки immutable).
  if (isStatic(url)) {
    event.respondWith(
      caches.match(req).then((hit) => hit || fetch(req).then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
        return res;
      }))
    );
    return;
  }

  // Навигация (HTML-страницы) — network-first, фолбэк на офлайн-заглушку.
  // Ничего не кэшируем (живые данные + авторизация).
  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req).catch(() => caches.match('/offline.html'))
    );
    return;
  }

  // Остальное (XHR/fetch к /ui/messages и т.п.) — просто сеть.
});

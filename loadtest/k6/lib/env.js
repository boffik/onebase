export function envInt(name, fallback) {
  const raw = __ENV[name];
  if (!raw) return fallback;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

export function envFloat(name, fallback) {
  const raw = __ENV[name];
  if (!raw) return fallback;
  const n = Number.parseFloat(raw);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

export function envDuration(name, fallback) {
  return __ENV[name] || fallback;
}

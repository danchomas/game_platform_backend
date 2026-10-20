const numberFormatter = new Intl.NumberFormat('ru-RU');

export function formatNumber(value = 0) {
  return numberFormatter.format(value || 0);
}

export function percent(ratio = 0) {
  return `${Math.round((ratio || 0) * 100)}%`;
}

export function duration(seconds = 0) {
  const totalSeconds = Math.max(0, Math.round(seconds || 0));
  const minutes = Math.floor(totalSeconds / 60);
  const remainingSeconds = String(totalSeconds % 60).padStart(2, '0');
  return `${minutes}:${remainingSeconds}`;
}

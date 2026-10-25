export function normalizeApiBase(value) {
  const base = value.trim().replace(/\/$/, '');
  if (!base) {
    return '';
  }
  let url;
  try {
    url = new URL(base);
  } catch {
    throw new Error('Укажите корректный HTTP(S)-адрес сервера.');
  }
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new Error('Укажите HTTP(S)-адрес без логина, параметров и фрагментов.');
  }
  return url.href.replace(/\/$/, '');
}

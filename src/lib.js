export function readStore(key, fallback) {
  try { const value = localStorage.getItem(`bee.${key}`); return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}
export function writeStore(key, value) { try { localStorage.setItem(`bee.${key}`, JSON.stringify(value)); } catch { /* Private mode may disable storage. */ } }
export const formatNumber = n => new Intl.NumberFormat('ru-RU').format(n || 0);
export const percent = n => `${Math.round((n || 0) * 100)}%`;
export const duration = n => `${Math.floor((n || 0) / 60)}:${String(Math.round((n || 0) % 60)).padStart(2, '0')}`;
export function download(content, filename, type = 'text/html') {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const a = document.createElement('a'); a.href = url; a.download = filename; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function toCSV(rows) {
  return '\ufeff' + rows.map(row => row.map(value => `"${String(value ?? '').replaceAll('"', '""')}"`).join(';')).join('\r\n');
}
export function createAPI(base, token) {
  return async (path, options = {}) => {
    const timeout = options.timeout || 240000;
    const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), timeout);
    try {
      const response = await fetch(`${base.replace(/\/$/, '')}${path}`, {
        ...options, signal: options.signal || controller.signal,
        headers: { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers }
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `Сервер ответил с ошибкой ${response.status}`);
      }
      if (options.text) return await response.text();
      if (response.status === 204) return null;
      const type = response.headers.get('content-type') || '';
      if (!type.includes('application/json')) throw new Error('API недоступен. Проверьте адрес сервера в настройках подключения.');
      return await response.json();
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('Сервер не успел ответить. Обновите список игр перед повторной генерацией.');
      if (error instanceof TypeError) throw new Error('Нет соединения с сервером. Проверьте, что бэкенд запущен и адрес указан верно.');
      throw error;
    } finally { clearTimeout(timer); }
  };
}
// Generated game code runs in an opaque-origin iframe. Preview cannot call APIs or navigate the host.
export function previewHTML(html) {
  const policy = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' blob:; style-src 'unsafe-inline'; img-src data: blob:; media-src data: blob:; font-src data:; connect-src 'none'; form-action 'none'; base-uri 'none'">`;
  const stub = `<script>window.GameSDK={start(){parent.postMessage({source:'studio-preview',type:'start'},'*')},end(result,score){parent.postMessage({source:'studio-preview',type:'end',data:{result,score}},'*')},event(){}};<\/script>`;
  if (/<head[^>]*>/i.test(html)) return html.replace(/<head[^>]*>/i, match => `${match}${policy}${stub}`);
  return `<!doctype html><head>${policy}${stub}</head>` + html.replace(/<!doctype[^>]*>/i, '');
}

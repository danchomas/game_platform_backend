import test from 'node:test';
import assert from 'node:assert/strict';
import { HttpClient } from '../src/shared/api/HttpClient.js';
import { previewHTML } from '../src/features/studio/previewHTML.js';
import { toCSV } from '../src/shared/lib/download.js';
import { DEMO_GAMES } from '../src/demo/games.js';
import { getDemoAnalytics } from '../src/demo/analytics.js';
import { demoHTML } from './fixtures/demoHTML.js';

test('HTTP client sends bearer token and preserves the prompt', async () => {
  let capturedRequest;
  const client = new HttpClient({
    baseUrl: 'https://example.test/',
    accessToken: 'secret',
    fetchImplementation: async (url, options) => {
      capturedRequest = { url, options };
      return new Response(JSON.stringify({ game: { id: 'abc' }, version: 1 }), {
        headers: { 'Content-Type': 'application/json' },
      });
    },
  });
  const result = await client.request('/api/games', {
    method: 'POST',
    body: JSON.stringify({ prompt: 'Змейка для бабушки' }),
  });
  assert.equal(capturedRequest.url, 'https://example.test/api/games');
  assert.equal(capturedRequest.options.headers.Authorization, 'Bearer secret');
  assert.deepEqual(JSON.parse(capturedRequest.options.body), { prompt: 'Змейка для бабушки' });
  assert.equal(result.game.id, 'abc');
  assert.equal('timeout' in capturedRequest.options, false);
  assert.equal('text' in capturedRequest.options, false);
});

test('HTTP client surfaces server errors and rejects an HTML fallback', async () => {
  const errorClient = new HttpClient({
    fetchImplementation: async () =>
      new Response(JSON.stringify({ error: 'неверный код доступа' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
  });
  await assert.rejects(errorClient.request('/api/auth/login'), /неверный код доступа/);
  const fallbackClient = new HttpClient({
    fetchImplementation: async () =>
      new Response('<html>index</html>', { headers: { 'Content-Type': 'text/html' } }),
  });
  await assert.rejects(fallbackClient.request('/api/games/mine'), /API недоступен/);
  const exportClient = new HttpClient({
    fetchImplementation: async () => new Response('<html>game</html>'),
  });
  assert.equal(
    await exportClient.request('/api/games/x/export', { text: true }),
    '<html>game</html>',
  );
  const deleteClient = new HttpClient({
    fetchImplementation: async () => new Response(null, { status: 204 }),
  });
  assert.equal(await deleteClient.request('/api/games/x', { method: 'DELETE' }), null);
});

test('preview blocks network and forms even when generated HTML omits head', () => {
  for (const html of [
    '<!doctype html><html><head><title>x</title></head><body>x</body></html>',
    '<body><script>run();</script></body>',
  ]) {
    const result = previewHTML(html);
    assert.match(result, /connect-src 'none'/);
    assert.match(result, /form-action 'none'/);
    assert.ok(
      result.indexOf('Content-Security-Policy') < result.indexOf('<script>run()') ||
        !result.includes('<script>run()'),
    );
    assert.match(result, /studio-preview/);
  }
});
test('all four demo exports are self-contained with valid scripts and SDK hooks', () => {
  for (const game of DEMO_GAMES) {
    const html = demoHTML(game);
    assert.match(html, /<!DOCTYPE html>/);
    assert.doesNotMatch(html, /(?:src|href)=["']https?:/);
    assert.match(html, /GameSDK.start/);
    assert.match(html, /GameSDK.end/);
    const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
    assert.doesNotThrow(() => new Function(script));
  }
});
test('game titles cannot terminate exported script or title elements', () => {
  const html = demoHTML({ ...DEMO_GAMES[0], title: '</script><script>alert(1)</script>' });
  assert.equal((html.match(/<script>/g) || []).length, 1);
  assert.match(html, /&lt;\/script&gt;/);
  assert.doesNotThrow(() => new Function(html.match(/<script>([\s\S]*?)<\/script>/)[1]));
});
test('demo analytics respects dates, range and per-game filtering', () => {
  const all = getDemoAnalytics(7),
    snake = getDemoAnalytics(30, 'demo-snake'),
    empty = getDemoAnalytics(7, 'demo-click');
  assert.equal(all.daily.length, 7);
  assert.equal(snake.daily.length, 30);
  assert.ok(all.summary.sessions > 0);
  assert.equal(empty.summary.sessions, 0);
  assert.ok(all.summary.completion_rate <= 1);
  assert.equal(all.daily.at(-1).day, new Date().toLocaleDateString('sv-SE'));
});
test('CSV preserves Cyrillic, semicolons, quotes and line breaks', () => {
  assert.equal(
    toCSV([
      ['Игра', 'a;"b"'],
      ['строка\nдва', null],
    ]),
    '\ufeff"Игра";"a;""b"""\r\n"строка\nдва";""',
  );
});

test('request timeout still works when the caller provides a cancellation signal', async () => {
  const client = new HttpClient({
    fetchImplementation: async (_url, { signal }) =>
      new Promise((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), {
          once: true,
        });
      }),
  });
  const controller = new AbortController();
  await assert.rejects(
    client.request('/api/games', { timeout: 1, signal: controller.signal }),
    /Сервер не успел ответить/,
  );
});

test('explicit cancellation is distinguishable from a server timeout', async () => {
  const client = new HttpClient({
    fetchImplementation: async (_url, { signal }) =>
      new Promise((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), {
          once: true,
        });
      }),
  });
  const controller = new AbortController();
  const request = client.request('/api/games', { signal: controller.signal });
  controller.abort();
  await assert.rejects(request, { name: 'AbortError' });
});

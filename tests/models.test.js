import test from 'node:test';
import assert from 'node:assert/strict';
import { PlatformApi } from '../src/shared/api/PlatformApi.js';
import { normalizeApiBase } from '../src/features/connection/connectionValidation.js';
import { filterGames } from '../src/features/games/galleryFilters.js';
import { DEMO_GAMES } from '../src/demo/games.js';

test('game API centralizes draft export, iteration, restore and publication routes', async () => {
  const calls = [];
  const api = new PlatformApi({
    request: async (path, options) => {
      calls.push({ path, ...options });
    },
  });
  await api.getGameHTML('id/with space');
  await api.iterateGame('id', 'Кнопки больше');
  await api.restoreVersion('id', 3);
  await api.setPublished('id', true);
  await api.setPublished('id', false);
  await api.renameGame('id', 'Новая игра');
  await api.getAnalytics({ days: 7, gameId: 'id', scope: 'mine' });
  assert.equal(calls[0].path, '/api/games/id%2Fwith%20space/export');
  assert.equal(calls[0].text, true);
  assert.equal(calls[1].method, 'POST');
  assert.deepEqual(JSON.parse(calls[1].body), { content: 'Кнопки больше' });
  assert.equal(calls[2].path, '/api/games/id/versions/3/restore');
  assert.equal(calls[3].path, '/api/games/id/publish');
  assert.equal(calls[4].path, '/api/games/id/unpublish');
  assert.equal(calls[5].method, 'PATCH');
  assert.equal(calls[6].path, '/api/analytics/games/id?days=7');
});

test('public and personal libraries retain distinct pagination limits', async () => {
  const paths = [];
  const api = new PlatformApi({ request: async (path) => paths.push(path) });
  await api.listGames({ collection: 'public', offset: 100 });
  await api.listGames({ collection: 'mine', offset: 200 });
  assert.deepEqual(paths, [
    '/api/gallery?limit=100&offset=100',
    '/api/games/mine?limit=200&offset=200',
  ]);
});

test('analytics accepts Go null collections before the first game event', async () => {
  const response = {
    scope: 'mine',
    days: 7,
    summary: { sessions: 0, wins: 0 },
    daily: [{ day: '2026-10-20', sessions: 0 }],
    games: null,
    custom_events: null,
    session_duration: null,
  };
  const api = new PlatformApi({ request: async () => response });
  const analytics = await api.getAnalytics({ days: 7 });
  assert.deepEqual(analytics.games, []);
  assert.deepEqual(analytics.custom_events, []);
  assert.deepEqual(analytics.session_duration, []);
  assert.deepEqual(analytics.daily, response.daily);
  assert.deepEqual(analytics.summary, response.summary);
  assert.equal(analytics.scope, 'mine');
  assert.equal(response.custom_events, null);
});

test('analytics preserves nonempty server events and duration buckets', async () => {
  const response = {
    summary: { sessions: 1, wins: 1 },
    daily: [{ day: '2026-10-20', sessions: 1 }],
    games: [{ id: 'game-1', title: 'Тестовая игра', sessions: 1 }],
    custom_events: [{ type: 'level_complete', count: 1, sessions: 1 }],
    session_duration: [{ label: '10-30s', count: 1 }],
  };
  const api = new PlatformApi({ request: async () => response });
  assert.deepEqual(await api.getAnalytics({ days: 7, gameId: 'game-1' }), response);
});

test('gallery composes search and status filters without mutating source data', () => {
  const games = structuredClone(DEMO_GAMES);
  const order = games.map((game) => game.id);
  assert.equal(
    filterGames(games, { query: '  ЗМЕЙКА  ', status: 'published' })[0].id,
    'demo-snake',
  );
  assert.deepEqual(
    filterGames(games, { status: 'drafts' }).map((game) => game.id),
    ['demo-click'],
  );
  assert.equal(filterGames(games, { query: 'для детей' })[0].id, 'demo-catch');
  assert.equal(filterGames(games, { sort: 'popular' })[0].id, 'demo-snake');
  assert.deepEqual(
    games.map((game) => game.id),
    order,
  );
});

test('connection accepts a local proxy and rejects credential-bearing or invalid URLs', () => {
  assert.equal(normalizeApiBase('  '), '');
  assert.equal(normalizeApiBase(' https://api.example.test/ '), 'https://api.example.test');
  assert.equal(normalizeApiBase('http://127.0.0.1:8080/'), 'http://127.0.0.1:8080');
  for (const value of [
    'javascript:alert(1)',
    'ftp://api.example.test',
    'https://name:secret@example.test',
    'https://example.test?token=secret',
    'https://example.test#fragment',
    'invalid',
  ]) {
    assert.throws(() => normalizeApiBase(value));
  }
});

test('duration formatting carries rounded seconds into the next minute', async () => {
  const { duration } = await import('../src/shared/lib/format.js');
  assert.equal(duration(59.6), '1:00');
  assert.equal(duration(125), '2:05');
  assert.equal(duration(-1), '0:00');
});

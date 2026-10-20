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

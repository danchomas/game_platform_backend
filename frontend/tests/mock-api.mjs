// Local UI QA fixture. No LLM calls, database writes or production data.
import http from 'node:http';
import { demoHTML } from './fixtures/demoHTML.js';
import { DEMO_GAMES } from '../src/demo/games.js';
const user = { id: 'qa-manager', name: 'Тестовый менеджер', role: 'manager' };
let game = null,
  messages = [],
  versions = [];
const plan = {
  title: 'Тестовая змейка',
  genre: 'snake',
  controls: { scheme: 'swipe' },
  ui: { scale: 'xlarge' },
  round_seconds: 180,
  summary: '• Тестовый план API: змейка\n• Управление свайпами',
};

// Walks the fixture game through the server pipeline stages, as the real background build does.
function startBuild(reply) {
  const stages = ['generating', 'validating', 'testing', 'ready'];
  stages.forEach((status, index) =>
    setTimeout(() => {
      game.status = status;
      if (status === 'ready') {
        game.version++;
        game.html = demoHTML({ ...DEMO_GAMES[0], title: game.title });
        messages.push({ role: 'assistant', content: reply, version: game.version });
        versions.unshift({ version: game.version, instruction: messages.at(-2)?.content || '' });
      }
    }, 700 * index),
  );
}
const server = http.createServer(async (req, res) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Headers', 'Authorization, Content-Type');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PATCH, DELETE, OPTIONS');
  if (req.method === 'OPTIONS') {
    res.writeHead(204).end();
    return;
  }
  const url = new URL(req.url, 'http://localhost');
  let body = '';
  for await (const part of req) body += part;
  const input = body ? JSON.parse(body) : {};
  const json = (value, status = 200) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(value));
  };
  if (url.pathname === '/api/auth/register' || url.pathname === '/api/auth/login') {
    json({ user, token: 'qa-only-token', access_key: 'QA-LOCAL-ONLY' });
    return;
  }
  if (url.pathname === '/api/me') {
    json(user);
    return;
  }
  if (url.pathname === '/api/games/mine' || url.pathname === '/api/gallery') {
    json(game ? [{ ...game, html: undefined }] : []);
    return;
  }
  if (url.pathname === '/api/games' && req.method === 'POST') {
    game = {
      ...DEMO_GAMES[0],
      id: 'qa-generated',
      owner_id: user.id,
      title: 'Тестовая змейка',
      prompt: input.prompt,
      published: false,
      version: 0,
      status: 'awaiting_approval',
      spec: plan,
      html: '',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    messages = [
      { role: 'user', content: input.prompt },
      { role: 'assistant', content: plan.summary },
    ];
    versions = [];
    json({ game, plan, reply: plan.summary }, 201);
    return;
  }
  if (url.pathname.endsWith('/plan') && req.method === 'POST') {
    messages.push(
      { role: 'user', content: input.content },
      { role: 'assistant', content: plan.summary },
    );
    json({ game, plan, reply: plan.summary });
    return;
  }
  if (url.pathname.endsWith('/approve') && req.method === 'POST') {
    startBuild('Тестовый ответ API: игра собрана и проверена.');
    json({ game }, 202);
    return;
  }
  if (url.pathname.endsWith('/status')) {
    json({
      game_id: game.id,
      status: game.status,
      version: game.version,
      game: { ...game, html: undefined },
    });
    return;
  }
  if (url.pathname.endsWith('/messages')) {
    if (req.method === 'GET') {
      json(messages);
      return;
    }
    messages.push({ role: 'user', content: input.content });
    startBuild('Тестовый ответ API: правки применены.');
    json({ game }, 202);
    return;
  }
  if (url.pathname.endsWith('/versions')) {
    json(versions);
    return;
  }
  if (url.pathname.endsWith('/restore')) {
    game.version++;
    versions.unshift({ version: game.version, instruction: 'Восстановление' });
    json({ game, reply: 'Версия восстановлена.', version: game.version });
    return;
  }
  if (url.pathname.endsWith('/publish') || url.pathname.endsWith('/unpublish')) {
    game.published = url.pathname.endsWith('/publish');
    json(game);
    return;
  }
  if (url.pathname.endsWith('/export') || url.pathname.startsWith('/g/')) {
    res.writeHead(200, { 'Content-Type': 'text/html' }).end(game.html);
    return;
  }
  if (url.pathname.startsWith('/api/analytics/')) {
    json({
      summary: {
        sessions: 12,
        players: 9,
        returning_players: 3,
        engaged_sessions: 10,
        game_starts: 10,
        game_ends: 7,
        wins: 5,
        losses: 2,
        avg_session_sec: 94,
        avg_score: 4.5,
        engagement_rate: 10 / 12,
        completion_rate: 0.7,
        win_rate: 5 / 7,
      },
      daily: [
        {
          day: new Date().toLocaleDateString('sv-SE'),
          sessions: 12,
          game_starts: 10,
          wins: 5,
          losses: 2,
        },
      ],
      games: game ? [{ ...game, sessions: 12, avg_session_sec: 94, completion_rate: 0.7 }] : [],
      custom_events: [],
      session_duration: [{ label: '1–3 мин', count: 12 }],
    });
    return;
  }
  if (req.method === 'PATCH') {
    game.title = input.title;
    json(game);
    return;
  }
  if (req.method === 'DELETE') {
    game = null;
    res.writeHead(204).end();
    return;
  }
  json({ error: 'QA endpoint not found' }, 404);
});
server.listen(4174, '127.0.0.1', () =>
  process.stdout.write('Local QA fixture: http://127.0.0.1:4174\n'),
);

import { readStore } from '../shared/lib/storage.js';
import { DEMO_GAMES } from './games.js';

export function getDemoAnalytics(days = 7, gameId = 'all') {
  const gameShare =
    gameId === 'all'
      ? 1
      : {
          'demo-snake': 0.44,
          'demo-catch': 0.33,
          'demo-memory': 0.23,
        }[gameId] || 0;
  const dailySessionCounts = Array.from(
    {
      length: days,
    },
    (_, index) =>
      Math.round((770 + index * 27 + [200, -120, 280, 60, 180, 390, 220][index % 7]) * gameShare),
  );
  const totalSessions = dailySessionCounts.reduce((sum, count) => sum + count, 0);
  const events = readStore('demoEvents', []).filter(
    (event) => gameId === 'all' || event.gameId === gameId,
  );
  const starts = events.filter((event) => event.type === 'start').length,
    ends = events.filter((event) => event.type === 'end').length;
  const summary = {
    sessions: totalSessions + starts,
    players: Math.round(totalSessions * 0.74),
    returning_players: Math.round(totalSessions * 0.21),
    engaged_sessions: Math.round(totalSessions * 0.86),
    game_starts: Math.round(totalSessions * 0.86) + starts,
    game_ends: Math.round(totalSessions * 0.62) + ends,
    wins: Math.round(totalSessions * 0.42),
    losses: Math.round(totalSessions * 0.2),
    avg_session_sec: totalSessions ? 154 : 0,
    avg_score: totalSessions ? 8.3 : 0,
    max_score: totalSessions ? 24 : 0,
    engagement_rate: totalSessions ? 0.86 : 0,
    completion_rate: totalSessions ? 0.72 : 0,
    win_rate: totalSessions ? 0.68 : 0,
  };
  return {
    summary,
    daily: dailySessionCounts.map((sessions, index) => {
      const date = new Date();
      date.setDate(date.getDate() - days + 1 + index);
      return {
        day: date.toLocaleDateString('sv-SE'),
        sessions: sessions,
        game_starts: Math.round(sessions * 0.86),
        wins: Math.round(sessions * 0.42),
        losses: Math.round(sessions * 0.2),
      };
    }),
    games: DEMO_GAMES.map((game, index) => ({
      ...game,
      sessions: Math.round(totalSessions * [0.44, 0.33, 0.23, 0][index]),
      players: Math.round(totalSessions * [0.32, 0.26, 0.16, 0][index]),
      avg_session_sec: [182, 136, 121, 0][index],
      completion_rate: [0.78, 0.69, 0.64, 0][index],
      win_rate: [0.71, 0.64, 0.66, 0][index],
    })),
    custom_events: [
      {
        type: 'level_complete',
        count: Math.round(totalSessions * 0.52),
      },
      {
        type: 'reward_claimed',
        count: Math.round(totalSessions * 0.31),
      },
    ],
    session_duration: [
      {
        label: 'до 30 с',
        count: 14,
      },
      {
        label: '30–60 с',
        count: 22,
      },
      {
        label: '1–3 мин',
        count: 43,
      },
      {
        label: '3+ мин',
        count: 21,
      },
    ],
  };
}

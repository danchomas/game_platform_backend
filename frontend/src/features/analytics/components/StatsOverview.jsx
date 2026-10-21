import { Users, MousePointer2, Timer, Flag } from 'lucide-react';
import { formatNumber, percent, duration } from '../../../shared/lib/format';

export function StatsOverview({ summary = {}, demo, loading }) {
  return (
    <div className="stat-grid">
      {[
        {
          title: 'Игровые сессии',
          value: formatNumber(summary.sessions),
          icon: MousePointer2,
          sub: 'Запуски игр за период',
          line: [12, 25, 19, 43, 35, 50, 62, 55, 78],
        },
        {
          title: 'Уникальные игроки',
          value: formatNumber(summary.players),
          icon: Users,
          sub: `${formatNumber(summary.returning_players)} вернулись снова`,
          line: [15, 23, 19, 32, 30, 49, 41, 60, 70],
        },
        {
          title: 'Средняя сессия',
          value: duration(summary.avg_session_sec),
          icon: Timer,
          sub: 'Активное время в игре',
          line: [30, 27, 36, 33, 47, 39, 45, 58, 65],
        },
        {
          title: 'Завершение игры',
          value: percent(summary.completion_rate),
          icon: Flag,
          sub: 'От всех начатых игр',
          line: [21, 26, 20, 39, 46, 41, 55, 53, 64],
        },
      ].map(({ title, value, icon: Icon, sub: description, line: sparklineValues }) => (
        <div className="stat-card" key={title}>
          <div className="stat-title">
            {title}
            <Icon size={16} />
          </div>
          <strong>{loading ? '—' : value}</strong>
          <div className="stat-bottom">
            <span>{description}</span>
            {demo && (
              <svg viewBox="0 0 90 35" width="70" height="30" aria-hidden="true">
                <polyline
                  points={sparklineValues
                    .map((value, index) => `${index * 11},${35 - value * 0.4}`)
                    .join(' ')}
                  fill="none"
                  stroke="#a9b28f"
                  strokeWidth="1.7"
                />
              </svg>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

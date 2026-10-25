import { useState } from 'react';
import { formatNumber } from '../../../shared/lib/format';
import { EmptyState } from '../../../shared/ui/EmptyState';
import { Spinner } from '../../../shared/ui/Spinner';

export function ActivityChart({ daily = [], loading, days }) {
  const [metric, setMetric] = useState('sessions');
  const [hover, setHover] = useState(null);
  const max = Math.max(...daily.map((point) => point[metric] || 0), 1) * 1.2;
  const chart = daily.map((point, index) => ({
    x: 48 + (index / Math.max(daily.length - 1, 1)) * 852,
    y: 190 - ((point[metric] || 0) / max) * 155,
    ...point,
  }));
  const path = chart.map((point, index) => `${index ? 'L' : 'M'}${point.x},${point.y}`).join(' ');
  const first = daily[0]?.day,
    last = daily.at(-1)?.day;
  const formatDay = (day) =>
    day
      ? new Date(day + 'T12:00:00').toLocaleDateString('ru-RU', {
          day: 'numeric',
          month: 'short',
        })
      : '';
  return (
    <section className="panel activity-panel">
      <div className="panel-title">
        <div>
          <h2>Вовлечённость в динамике</h2>
          <p>{first ? `${formatDay(first)} — ${formatDay(last)}` : 'Пока нет игровых событий'}</p>
        </div>
        <span className="legend-dot">{metric === 'sessions' ? 'Сессии' : 'Начала игры'}</span>
      </div>
      <div className="segment small">
        <button
          className={metric === 'sessions' ? 'active' : ''}
          onClick={() => setMetric('sessions')}
        >
          Сессии
        </button>
        <button
          className={metric === 'game_starts' ? 'active' : ''}
          onClick={() => setMetric('game_starts')}
        >
          Начала игры
        </button>
      </div>
      <div className="chart-wrap">
        {loading ? (
          <div className="chart-loading">
            <Spinner />
          </div>
        ) : !daily.length ? (
          <EmptyState
            title="Здесь появится динамика"
            text="Опубликуйте игру и поделитесь ссылкой с игроками."
          />
        ) : (
          <>
            <svg
              viewBox="0 0 930 235"
              role="img"
              aria-label={`График: ${metric === 'sessions' ? 'сессии' : 'начала игры'} за ${days} дней`}
              onMouseLeave={() => setHover(null)}
            >
              <defs>
                <linearGradient id="chart-fill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="0%" stopColor="#efd365" stopOpacity=".34" />
                  <stop offset="100%" stopColor="#efd365" stopOpacity=".01" />
                </linearGradient>
              </defs>
              {[0, 0.25, 0.5, 0.75, 1].map((fraction) => (
                <g key={fraction}>
                  <line
                    x1="48"
                    y1={190 - fraction * 155}
                    x2="900"
                    y2={190 - fraction * 155}
                    stroke="#eeeee9"
                    strokeDasharray="3 5"
                  />
                  <text x="0" y={194 - fraction * 155} fill="#a2a49a" fontSize="13">
                    {formatNumber(Math.round(max * fraction))}
                  </text>
                </g>
              ))}
              <path d={`${path} L${chart.at(-1).x},190 L48,190 Z`} fill="url(#chart-fill)" />
              <path
                d={path}
                stroke="#d5b343"
                strokeWidth="2.5"
                fill="none"
                strokeLinejoin="round"
              />
              {chart.map((point, index) => (
                <g key={index}>
                  {(index === 0 ||
                    index === chart.length - 1 ||
                    index % Math.ceil(chart.length / 6) === 0) && (
                    <text x={point.x} y="222" textAnchor="middle" fill="#999c91" fontSize="13">
                      {formatDay(point.day)}
                    </text>
                  )}
                  <rect
                    x={point.x - 852 / Math.max(chart.length, 1) / 2}
                    y="20"
                    width={852 / Math.max(chart.length, 1) + 2}
                    height="180"
                    fill="transparent"
                    onMouseEnter={() => setHover(point)}
                  />
                </g>
              ))}
              {hover && (
                <g pointerEvents="none">
                  <line
                    x1={hover.x}
                    x2={hover.x}
                    y1="25"
                    y2="190"
                    stroke="#a6a88b"
                    strokeDasharray="3 3"
                  />
                  <circle
                    cx={hover.x}
                    cy={hover.y}
                    r="5"
                    fill="#d5b343"
                    stroke="white"
                    strokeWidth="3"
                  />
                  <rect
                    x={Math.min(Math.max(hover.x - 61, 42), 770)}
                    y="0"
                    width="125"
                    height="28"
                    rx="7"
                    fill="#30332a"
                  />
                  <text
                    x={Math.min(Math.max(hover.x - 61, 42), 770) + 62}
                    y="18"
                    fontSize="13"
                    textAnchor="middle"
                    fill="white"
                  >
                    {formatDay(hover.day)} · {formatNumber(hover[metric])}
                  </text>
                </g>
              )}
            </svg>
          </>
        )}
      </div>
    </section>
  );
}

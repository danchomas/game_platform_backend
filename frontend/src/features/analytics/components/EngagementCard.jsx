import { formatNumber, percent } from '../../../shared/lib/format';

export function EngagementCard({ summary = {} }) {
  return (
    <section className="panel engagement-panel">
      <div className="panel-title">
        <div>
          <h2>В игре с первого касания</h2>
          <p>Доля вовлечённых сессий</p>
        </div>
      </div>
      <div className="donut">
        <svg viewBox="0 0 180 180">
          <circle cx="90" cy="90" r="70" fill="none" stroke="#f0f0e9" strokeWidth="13" />
          <circle
            cx="90"
            cy="90"
            r="70"
            fill="none"
            stroke="#f1d45e"
            strokeWidth="13"
            strokeDasharray={`${Math.min(summary.engagement_rate || 0, 1) * 439.8} 439.8`}
            strokeLinecap="round"
            transform="rotate(-90 90 90)"
          />
        </svg>
        <div>
          <strong>{percent(summary.engagement_rate)}</strong>
          <span>вовлечённость</span>
        </div>
      </div>
      <div className="donut-caption">
        <span>
          <i /> Взаимодействовали
        </span>
        <strong>{formatNumber(summary.engaged_sessions)}</strong>
      </div>
      <div className="donut-caption muted">
        <span>
          <i /> Только открыли
        </span>
        <strong>
          {formatNumber(Math.max(0, (summary.sessions || 0) - (summary.engaged_sessions || 0)))}
        </strong>
      </div>
    </section>
  );
}

import { Timer, Sparkles } from 'lucide-react';
import { percent } from '../../../shared/lib/format';

export function SessionDuration({ data }) {
  const summary = data?.summary || {};
  const totalDuration = (data?.session_duration || []).reduce(
    (sum, bucket) => sum + bucket.count,
    0,
  );
  return (
    <section className="panel session-panel">
      <div className="panel-title">
        <div>
          <h2>Время за игрой</h2>
          <p>Распределение сессий</p>
        </div>
        <Timer size={16} />
      </div>
      <div className="duration-bars">
        {(data?.session_duration || []).map((bucket) => (
          <div key={bucket.label}>
            <span>{bucket.label}</span>
            <div>
              <i
                style={{
                  width: `${totalDuration ? (bucket.count / totalDuration) * 100 : 0}%`,
                }}
              />
            </div>
            <strong>{percent(totalDuration ? bucket.count / totalDuration : 0)}</strong>
          </div>
        ))}
      </div>
      <div className="insight">
        <Sparkles size={17} />
        <p>
          {summary.sessions > 0
            ? `${percent(summary.completion_rate)} начатых игр доходят до финала. Попробуйте короткие раунды, чтобы повысить завершения.`
            : 'Начните собирать статистику: опубликуйте игру и откройте её по ссылке.'}
        </p>
      </div>
    </section>
  );
}

import { Hammer } from 'lucide-react';

const CONTROL_LABELS = {
  tap: 'касания',
  swipe: 'свайпы',
  drag: 'перетаскивание',
  hold: 'удержание',
  buttons: 'экранные кнопки',
  mixed: 'жесты и кнопки',
};

const SCALE_LABELS = {
  normal: 'обычный',
  large: 'крупный',
  xlarge: 'очень крупный',
};

function describePlan(plan) {
  return [
    plan.genre && ['Жанр', plan.genre],
    plan.controls?.scheme && [
      'Управление',
      CONTROL_LABELS[plan.controls.scheme] || plan.controls.scheme,
    ],
    plan.ui?.scale && ['Интерфейс', SCALE_LABELS[plan.ui.scale] || plan.ui.scale],
    plan.round_seconds && ['Раунд', `≈ ${Math.round(plan.round_seconds / 60) || 1} мин`],
  ].filter(Boolean);
}

export function PlanCard({ plan, busy, onApprove }) {
  return (
    <div className="plan-card">
      <div className="plan-card-title">
        План игры{plan.title ? ` «${plan.title}»` : ''}
        <small>Можно поправить в чате или сразу собрать</small>
      </div>
      <dl className="plan-facts">
        {describePlan(plan).map(([term, value]) => (
          <div key={term}>
            <dt>{term}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      <button className="plan-approve" onClick={onApprove} disabled={busy}>
        <Hammer size={14} />
        Собрать игру
      </button>
    </div>
  );
}

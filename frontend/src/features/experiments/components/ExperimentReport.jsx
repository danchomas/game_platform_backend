import { Trophy, Shuffle } from 'lucide-react';
import { formatNumber, percent, duration } from '../../../shared/lib/format';
import { EmptyState } from '../../../shared/ui/EmptyState';

/**
 * Dashboard of a live experiment: who the bandit favours, retention with confidence
 * intervals and the per-variant funnel.
 */
export function ExperimentReport({ report, busy, onChooseWinner }) {
  if (!report) {
    return null;
  }
  const { experiment, variants, leader_id: leaderId, significant } = report;
  const shown = variants.filter((variant) => variant.stats.exposures > 0);
  const leader = variants.find((variant) => variant.game_id === leaderId);
  const canChoose = experiment.status !== 'draft';

  return (
    <section className="experiment-report">
      <LeaderBanner
        leader={leader}
        significant={significant}
        threshold={report.retained_seconds}
        experiment={experiment}
      />
      {shown.length ? (
        <>
          <BanditPanel
            variants={variants}
            leaderId={leaderId}
            winnerId={experiment.winner_id}
            rule={report}
          />
          <RetentionChart
            variants={variants}
            leaderId={leaderId}
            winnerId={experiment.winner_id}
            threshold={report.retained_seconds}
          />
          <FunnelTable
            variants={variants}
            winnerId={experiment.winner_id}
            canChoose={canChoose}
            busy={busy}
            onChooseWinner={onChooseWinner}
          />
        </>
      ) : (
        <div className="panel">
          <EmptyState
            title="Ждём первых игроков"
            text="Откройте ссылку эксперимента или отправьте её игрокам — показы появятся здесь."
          />
        </div>
      )}
    </section>
  );
}

function LeaderBanner({ leader, significant, threshold, experiment }) {
  const finished = experiment.status === 'finished';
  const winner = finished ? experiment.winner_id : '';
  if (!leader && !winner) {
    return null;
  }
  if (winner) {
    const auto = experiment.winner_source === 'auto';
    return (
      <div className="panel leader-banner significant">
        <Trophy size={20} />
        <div>
          <strong>
            {auto ? 'Бандит выбрал победителя автоматически' : 'Победитель выбран вручную'}
          </strong>
          <span>Ссылка эксперимента теперь отдаёт всем игрокам только эту игру.</span>
        </div>
      </div>
    );
  }
  const { stats } = leader;
  const verdict = significant
    ? 'Порог автопобеды набран: эксперимент завершится при следующем заходе.'
    : 'Лидер может смениться: бандит продолжает подбирать варианты для новых игроков.';
  return (
    <div className={`panel leader-banner ${significant ? 'significant' : ''}`}>
      <Trophy size={20} />
      <div>
        <strong>
          Лидирует вариант {leader.label}: лучший с вероятностью {percent(stats.prob_best)}
        </strong>
        <span>
          {percent(stats.retention_rate)} показов доиграли до {threshold} с (95% интервал{' '}
          {percent(stats.retention_low)}–{percent(stats.retention_high)}). {verdict}
        </span>
      </div>
    </div>
  );
}

// Thompson sampling sends new players where P(best) is high, so the two bars should converge.
function BanditPanel({ variants, leaderId, winnerId, rule }) {
  return (
    <div className="panel bandit-panel">
      <div className="panel-title">
        <div>
          <h2>
            <Shuffle size={14} /> Распределение: Thompson sampling
          </h2>
          <p>
            Новых игроков чаще получает вариант, который вероятнее лучший; вернувшийся игрок
            остаётся в своём. Эксперимент завершится сам, когда вариант будет лучшим с вероятностью{' '}
            {percent(rule.auto_win_prob)}, у него наберётся {rule.auto_min_trials} завершённых
            показов, а у остальных — хотя бы по {rule.auto_min_rival_trials}.
          </p>
        </div>
      </div>
      <div className="bandit-rows" role="list">
        <div className="bandit-row bandit-legend" aria-hidden="true">
          <span />
          <span>
            <i className="swatch share" /> Доля показов
          </span>
          <span>
            <i className="swatch prob" /> Вероятность быть лучшим
          </span>
        </div>
        {variants
          .filter((variant) => variant.published && variant.version > 0)
          .map((variant) => {
            const { stats } = variant;
            const highlighted = variant.game_id === (winnerId || leaderId);
            return (
              <div className="bandit-row" role="listitem" key={variant.game_id}>
                <span className="retention-name">
                  {variant.label}
                  <small>{variant.title}</small>
                </span>
                <Meter
                  className="share"
                  value={stats.traffic_share}
                  label={`Вариант ${variant.label}: ${percent(stats.traffic_share)} показов (${formatNumber(stats.exposures)})`}
                />
                <Meter
                  className={`prob ${highlighted ? 'leader' : ''}`}
                  value={stats.prob_best}
                  label={`Вариант ${variant.label}: лучший с вероятностью ${percent(stats.prob_best)}`}
                />
              </div>
            );
          })}
      </div>
    </div>
  );
}

function Meter({ className, value, label }) {
  return (
    <span className="bandit-meter" title={label}>
      <span className="retention-track">
        <b className={className} style={{ width: percent(value) }} />
      </span>
      <span className="retention-value">{percent(value)}</span>
    </span>
  );
}

function RetentionChart({ variants, leaderId, winnerId, threshold }) {
  return (
    <div className="panel retention-chart">
      <div className="panel-title">
        <div>
          <h2>Удержание по вариантам</h2>
          <p>
            Доля показов, где игрок активно играл дольше {threshold} с. Полоса — оценка, усы — 95%
            интервал.
          </p>
        </div>
      </div>
      <div className="retention-rows" role="list">
        {variants.map((variant) => {
          const { stats } = variant;
          const empty = stats.exposures === 0;
          const highlighted = variant.game_id === (winnerId || leaderId);
          const tooltip = empty
            ? `Вариант ${variant.label}: показов не было`
            : `Вариант ${variant.label}: ${percent(stats.retention_rate)} (${formatNumber(
                stats.retained,
              )} из ${formatNumber(stats.exposures)}), интервал ${percent(
                stats.retention_low,
              )}–${percent(stats.retention_high)}`;
          return (
            <div className="retention-row" role="listitem" key={variant.game_id} title={tooltip}>
              <span className="retention-name">
                {variant.label}
                <small>{variant.title}</small>
              </span>
              <span className={`retention-track ${empty ? 'no-data' : ''}`}>
                {!empty && (
                  <>
                    <b
                      className={highlighted ? 'leader' : ''}
                      style={{ width: percent(stats.retention_rate) }}
                    />
                    <i
                      style={{
                        left: percent(stats.retention_low),
                        width: percent(stats.retention_high - stats.retention_low),
                      }}
                    />
                  </>
                )}
              </span>
              <span className="retention-value">{empty ? '—' : percent(stats.retention_rate)}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function FunnelTable({ variants, winnerId, canChoose, busy, onChooseWinner }) {
  return (
    <div className="panel funnel-table">
      <div className="panel-title">
        <div>
          <h2>Воронка вариантов</h2>
          <p>
            Считаются только заходы по ссылке эксперимента. Показ записывает сервер, остальное —
            SDK, одинаково для всех вариантов.
          </p>
        </div>
      </div>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Вариант</th>
              <th>Показы</th>
              <th>Доля</th>
              <th>P(лучший)</th>
              <th>Загрузились</th>
              <th>Вовлеклись</th>
              <th>Удержались</th>
              <th>Ошибки JS</th>
              <th>Ср. сессия</th>
              <th>Завершения*</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {variants.map((variant) => {
              const { stats } = variant;
              const isWinner = variant.game_id === winnerId;
              return (
                <tr key={variant.game_id}>
                  <td>
                    <strong className="funnel-variant">{variant.label}</strong> {variant.title}
                  </td>
                  <td>{formatNumber(stats.exposures)}</td>
                  <td>{percent(stats.traffic_share)}</td>
                  <td>{percent(stats.prob_best)}</td>
                  <td>{percent(stats.load_rate)}</td>
                  <td>{percent(stats.engagement_rate)}</td>
                  <td>{percent(stats.retention_rate)}</td>
                  <td>{percent(stats.error_rate)}</td>
                  <td>{duration(stats.avg_session_sec)}</td>
                  <td>{percent(stats.completion_rate)}</td>
                  <td>
                    {isWinner ? (
                      <span className="status live">
                        <i />
                        Победитель
                      </span>
                    ) : (
                      canChoose &&
                      variant.published && (
                        <button
                          className="button secondary small"
                          disabled={Boolean(busy)}
                          onClick={() => onChooseWinner(variant.game_id)}
                        >
                          Выбрать победителем
                        </button>
                      )
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p className="funnel-note">
        * Завершения зависят от того, как код игры сообщает о конце партии, поэтому между вариантами
        сравниваются осторожно.
      </p>
    </div>
  );
}

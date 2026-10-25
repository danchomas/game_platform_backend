import { Trophy } from 'lucide-react';
import { formatNumber, percent, duration } from '../../../shared/lib/format';
import { EmptyState } from '../../../shared/ui/EmptyState';

/** Dashboard of a live experiment: retention with confidence intervals and the per-variant funnel. */
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
        finished={experiment.status === 'finished'}
      />
      {shown.length ? (
        <>
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

function LeaderBanner({ leader, significant, threshold, finished }) {
  if (!leader) {
    return null;
  }
  const { stats } = leader;
  let verdict = 'Данных пока мало: интервалы вариантов пересекаются, лидер может смениться.';
  if (significant) {
    verdict = 'Разница значимая: можно выбирать победителя.';
  }
  if (finished) {
    verdict = 'Эксперимент завершён, ссылка отдаёт победителя.';
  }
  return (
    <div className={`panel leader-banner ${significant ? 'significant' : ''}`}>
      <Trophy size={20} />
      <div>
        <strong>
          Лидирует вариант {leader.label}: {percent(stats.retention_rate)} показов доиграли до{' '}
          {threshold} с
        </strong>
        <span>
          95% интервал: {percent(stats.retention_low)}–{percent(stats.retention_high)}. {verdict}
        </span>
      </div>
    </div>
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

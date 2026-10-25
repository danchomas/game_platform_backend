import { ArrowUpRight } from 'lucide-react';
import { formatNumber, percent, duration } from '../../../shared/lib/format';
import { EmptyState } from '../../../shared/ui/EmptyState';

export function GamesPerformance({ data, filter, onOpen }) {
  const summary = data?.summary || {};
  const rows = [...(data?.games || [])].sort(
    (firstGame, secondGame) => secondGame.sessions - firstGame.sessions,
  );
  return (
    <section className="panel games-table">
      <div className="panel-title">
        <div>
          <h2>{filter === 'all' ? 'Как играют в ваши игры' : 'Результаты выбранной игры'}</h2>
          <p>Сравнивайте механики и находите удачные идеи</p>
        </div>
        <span className="counter">{filter === 'all' ? rows.length : 1}</span>
      </div>
      {filter !== 'all' ? (
        <div className="detail-metrics">
          <div>
            <span>Победы</span>
            <strong>{formatNumber(summary.wins)}</strong>
          </div>
          <div>
            <span>Поражения</span>
            <strong>{formatNumber(summary.losses)}</strong>
          </div>
          <div>
            <span>Средний счёт</span>
            <strong>{Number(summary.avg_score || 0).toFixed(1)}</strong>
          </div>
          <div>
            <span>Доля побед</span>
            <strong>{percent(summary.win_rate)}</strong>
          </div>
        </div>
      ) : rows.length ? (
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Игра</th>
                <th>Сессии ↓</th>
                <th>Сред. время</th>
                <th>Завершения</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {rows.map((game, index) => (
                <tr key={game.id}>
                  <td>
                    <button className="table-game" onClick={() => onOpen(game)}>
                      <span className={`mini-art mini-${index % 4}`}>
                        {['↝', '☀', '✦', 'ϟ'][index % 4]}
                      </span>
                      <span>
                        {game.title}
                        <small>{game.genre || 'Игра'}</small>
                      </span>
                    </button>
                  </td>
                  <td>{formatNumber(game.sessions)}</td>
                  <td>{duration(game.avg_session_sec)}</td>
                  <td>
                    <div className="table-rate">
                      <span>{percent(game.completion_rate)}</span>
                      <i>
                        <b
                          style={{
                            width: percent(game.completion_rate),
                          }}
                        />
                      </i>
                    </div>
                  </td>
                  <td>
                    <button
                      className="icon-btn"
                      onClick={() => onOpen(game)}
                      aria-label={`Открыть ${game.title}`}
                    >
                      <ArrowUpRight size={15} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState
          title="Нужна первая игра"
          text="После публикации здесь появится сравнение результатов."
        />
      )}
    </section>
  );
}

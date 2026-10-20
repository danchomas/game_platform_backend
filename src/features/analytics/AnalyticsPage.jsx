import { download, toCSV } from '../../shared/lib/download';
import { AnalyticsFilters } from './components/AnalyticsFilters';
import { StatsOverview } from './components/StatsOverview';
import { ActivityChart } from './components/ActivityChart';
import { EngagementCard } from './components/EngagementCard';
import { GamesPerformance } from './components/GamesPerformance';
import { SessionDuration } from './components/SessionDuration';
import { CustomEvents } from './components/CustomEvents';

export function AnalyticsPage({ analytics, demo, games, onOpen }) {
  const { data, loading, days } = analytics;
  const daily = data?.daily || [];
  function exportStats() {
    download(
      toCSV([
        ['Дата', 'Сессии', 'Начала игры', 'Победы', 'Поражения'],
        ...daily.map((day) => [day.day, day.sessions, day.game_starts, day.wins, day.losses]),
      ]),
      `analytics-${days}days${demo ? '-demo' : ''}.csv`,
      'text/csv;charset=utf-8',
    );
  }
  return (
    <div className="page analytics-page">
      <div className="page-heading">
        <div>
          <div className="eyebrow">ЦИФРЫ, КОТОРЫЕ ИМЕЮТ ЗНАЧЕНИЕ</div>
          <h1>
            Играют. Возвращаются.
            <br />
            <span>Остаются с вами.</span>
          </h1>
          <p>Всё о том, как ваши игры вовлекают аудиторию.</p>
        </div>
        <div className="heading-note">
          <span className="big-asterisk">✳</span>
          <span>
            Меньше догадок.
            <br />
            Больше данных.
          </span>
        </div>
      </div>
      <AnalyticsFilters
        days={days}
        setDays={analytics.setDays}
        filter={analytics.filter}
        setFilter={analytics.setFilter}
        games={games}
        loading={loading}
        refresh={analytics.refresh}
        scope={analytics.scope}
        setScope={analytics.setScope}
        demo={demo}
        onExport={exportStats}
        hasData={daily.length > 0}
      />
      <StatsOverview summary={data?.summary} demo={demo} loading={loading} />
      <div className="charts-grid">
        <ActivityChart daily={daily} loading={loading} days={days} />
        <EngagementCard summary={data?.summary} />
      </div>
      <div className="analytics-bottom">
        <GamesPerformance data={data} filter={analytics.filter} onOpen={onOpen} />
        <SessionDuration data={data} />
      </div>
      <CustomEvents events={data?.custom_events} />
    </div>
  );
}

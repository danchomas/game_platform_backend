import { ArrowDownToLine, CalendarDays, ChevronDown, RefreshCw } from 'lucide-react';

export function AnalyticsFilters({
  days,
  setDays,
  filter,
  setFilter,
  games,
  loading,
  refresh,
  scope,
  setScope,
  demo,
  onExport,
  hasData,
}) {
  return (
    <div className="analytics-toolbar">
      <div className="select-wrap">
        <CalendarDays size={15} />
        <select
          aria-label="Период аналитики"
          value={days}
          onChange={(event) => setDays(Number(event.target.value))}
        >
          <option value="7">Последние 7 дней</option>
          <option value="30">Последние 30 дней</option>
          <option value="90">Последние 90 дней</option>
        </select>
        <ChevronDown size={13} />
      </div>
      <div className="select-wrap">
        <select
          aria-label="Игра для аналитики"
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
        >
          <option value="all">Все игры</option>
          {games.map((game) => (
            <option key={game.id} value={game.id}>
              {game.title}
            </option>
          ))}
        </select>
        <ChevronDown size={13} />
      </div>
      {!demo && (
        <div className="select-wrap">
          <select
            aria-label="Область аналитики"
            value={scope}
            onChange={(event) => setScope(event.target.value)}
          >
            <option value="mine">Мои игры</option>
            <option value="all">Вся платформа</option>
          </select>
          <ChevronDown size={13} />
        </div>
      )}
      <span className="toolbar-spacer" />
      {demo && <span className="demo-label">Демонстрационные данные</span>}
      <button
        className="icon-btn"
        onClick={refresh}
        disabled={loading}
        aria-label="Обновить аналитику"
      >
        <RefreshCw size={16} className={loading ? 'spin' : ''} />
      </button>
      <button className="button secondary small" onClick={onExport} disabled={!hasData || loading}>
        <ArrowDownToLine size={15} /> Отчёт
      </button>
    </div>
  );
}

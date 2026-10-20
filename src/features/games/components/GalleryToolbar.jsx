import { Search, ChevronDown, RefreshCw } from 'lucide-react';
import { GAME_FILTERS } from '../galleryFilters';

export function GalleryToolbar({
  query,
  setQuery,
  filter,
  setFilter,
  sort,
  setSort,
  onRefresh,
  loading,
}) {
  return (
    <div className="gallery-toolbar">
      <div className="filter-tabs">
        {GAME_FILTERS.map(({ label: label, value }) => (
          <button
            key={label}
            className={filter === value ? 'active' : ''}
            onClick={() => setFilter(value)}
          >
            {label}
          </button>
        ))}
      </div>
      <label className="search">
        <Search size={15} />
        <input
          placeholder="Найти игру"
          aria-label="Поиск игр"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </label>
      <div className="select-wrap">
        <select
          aria-label="Сортировка игр"
          value={sort}
          onChange={(event) => setSort(event.target.value)}
        >
          <option value="updated">Сначала новые</option>
          <option value="popular">По популярности</option>
          <option value="title">По названию</option>
        </select>
        <ChevronDown size={13} />
      </div>
      <button
        className="icon-btn"
        onClick={onRefresh}
        disabled={loading}
        aria-label="Обновить игры"
      >
        <RefreshCw size={15} className={loading ? 'spin' : ''} />
      </button>
    </div>
  );
}

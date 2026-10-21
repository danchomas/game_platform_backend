import { useState } from 'react';
import { Plus, ArrowUpRight, Globe } from 'lucide-react';
import { EmptyState } from '../../shared/ui/EmptyState';
import { Spinner } from '../../shared/ui/Spinner';
import { GameCard } from './components/GameCard';
import { GalleryToolbar } from './components/GalleryToolbar';
import { filterGames } from './galleryFilters';

export function GalleryPage({ library, demo, onOpen, onNew, onDelete, onRename }) {
  const {
    games,
    loading,
    collection,
    setCollection,
    refresh: onRefresh,
    loadMore: onMore,
    hasMore,
  } = library;
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [sort, setSort] = useState('updated');
  const filtered = filterGames(games, {
    query,
    status: filter,
    sort,
  });
  return (
    <div className="page gallery-page">
      <div className="page-heading">
        <div>
          <div className="eyebrow">ВАШИ ИДЕИ УЖЕ ИГРАЮТ</div>
          <h1>
            Маленькие миры.
            <br />
            <span>Большие возможности.</span>
          </h1>
          <p>Все игры в одном месте. Найдите ту самую или создайте новую.</p>
        </div>
        <button className="button yellow" onClick={onNew}>
          <Plus size={16} /> Создать игру
        </button>
      </div>
      <div className="gallery-collection">
        <button
          className={collection === 'mine' ? 'active' : ''}
          onClick={() => setCollection('mine')}
        >
          Мои игры <span>{collection === 'mine' ? games.length : '↗'}</span>
        </button>
        <button
          className={collection === 'public' ? 'active' : ''}
          onClick={() => setCollection('public')}
        >
          <Globe size={14} /> Галерея платформы
        </button>
        {demo && <span className="demo-label">Готовые демо-примеры</span>}
      </div>
      <GalleryToolbar
        query={query}
        setQuery={setQuery}
        filter={filter}
        setFilter={setFilter}
        sort={sort}
        setSort={setSort}
        onRefresh={onRefresh}
        loading={loading}
      />
      {loading ? (
        <div className="loading-section">
          <Spinner /> Загружаем ваши игры…
        </div>
      ) : (
        <>
          <div className="game-grid">
            {filtered.map((game, index) => (
              <GameCard
                key={game.id}
                game={game}
                index={index}
                demo={demo}
                isOwner={collection === 'mine'}
                onOpen={onOpen}
                onRename={onRename}
                onDelete={onDelete}
              />
            ))}
            {!query && filter === 'all' && collection === 'mine' && (
              <button className="new-game-card" onClick={onNew}>
                <span>
                  <Plus size={26} strokeWidth={1.3} />
                </span>
                <h3>Здесь будет ваша идея</h3>
                <p>Пара слов — и новая игра готова</p>
                <span className="text-button">
                  Создать игру <ArrowUpRight size={14} />
                </span>
              </button>
            )}
          </div>
          {!filtered.length && (
            <EmptyState
              title={query ? 'Пока ничего не нашлось' : 'Здесь начнётся ваша коллекция'}
              text={
                query
                  ? 'Попробуйте другое название или измените фильтр.'
                  : 'Создайте первую игру с помощью ИИ.'
              }
              action={
                !query && (
                  <button className="button yellow" onClick={onNew}>
                    <Plus size={15} />
                    Создать игру
                  </button>
                )
              }
            />
          )}
        </>
      )}
      {hasMore && (
        <div className="load-more">
          <button className="button secondary" onClick={onMore} disabled={loading}>
            Загрузить ещё
          </button>
        </div>
      )}
      <div className="gallery-note">
        <span>Создавайте, пробуйте, находите свою аудиторию.</span>
        <span>Каждая игра — новая возможность. ✳</span>
      </div>
    </div>
  );
}

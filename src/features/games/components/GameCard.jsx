import { useState } from 'react';
import { ArrowUpRight, Play, MoreHorizontal, Eye, Pencil, Trash2 } from 'lucide-react';
import { GameArt } from './GameArt';
import { GameStatus } from './GameStatus';
import { formatNumber } from '../../../shared/lib/format';

export function GameCard({ game, index, demo, isOwner, onOpen, onRename, onDelete }) {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <article className="game-card">
      <button
        className="game-card-art"
        onClick={() => onOpen(game)}
        aria-label={`Открыть игру ${game.title}`}
      >
        <GameArt
          kind={game.kind || ['snake', 'catch', 'memory', 'click'][index % 4]}
          color={game.color}
        />
        <span className="art-genre">{game.genre || 'HTML5'}</span>
        <span className="card-play">
          <Play size={19} fill="currentColor" />
        </span>
        {demo && <span className="card-demo">ДЕМО</span>}
      </button>
      <div className="game-card-info">
        <div>
          <button className="card-title" onClick={() => onOpen(game)}>
            {game.title}
            <ArrowUpRight size={15} />
          </button>
          <span className="game-audience">{game.audience || 'Для всех'}</span>
        </div>
        {isOwner && (
          <div className="card-menu">
            <button
              className="icon-btn"
              onClick={() => setMenuOpen((previous) => !previous)}
              aria-label={`Действия с игрой ${game.title}`}
              aria-expanded={menuOpen}
            >
              <MoreHorizontal size={19} />
            </button>
            {menuOpen && (
              <>
                <button
                  className="menu-dismiss"
                  tabIndex={-1}
                  aria-label="Закрыть меню"
                  onClick={() => setMenuOpen(false)}
                />
                <div className="dropdown">
                  <button
                    onClick={() => {
                      setMenuOpen(false);
                      onRename(game);
                    }}
                  >
                    <Pencil size={14} />
                    Переименовать
                  </button>
                  <button
                    className="danger-text"
                    onClick={() => {
                      setMenuOpen(false);
                      onDelete(game);
                    }}
                  >
                    <Trash2 size={14} />
                    Удалить
                  </button>
                </div>
              </>
            )}
          </div>
        )}
      </div>
      <div className="game-card-footer">
        <GameStatus published={game.published} />
        <span>
          <Eye size={13} />
          {formatNumber(game.plays)}
        </span>
        <span>v{game.version}</span>
      </div>
    </article>
  );
}

import { Download, Link2, SlidersHorizontal, History } from 'lucide-react';
import { GameStatus } from '../../games/components/GameStatus';

export function PreviewFooter({
  game,
  demo,
  busy,
  html,
  panel,
  setPanel,
  versions,
  onExport,
  onEmbed,
}) {
  return (
    <div className="preview-bottom">
      <div className="preview-bottom-title">
        <div>
          <h3>{game?.title || 'Ваша следующая игра'}</h3>
          <p>
            {game
              ? `${game.genre || 'HTML5'} · ${game.audience || 'Для всех'}`
              : 'Начните с пары слов в чате'}
          </p>
        </div>
        {game && <GameStatus published={!demo && game.published} status={!demo && game.status} />}
      </div>
      <div className="game-actions">
        <button
          className={`button secondary small ${panel === 'settings' ? 'selected' : ''}`}
          disabled={!game || busy}
          onClick={() => setPanel(panel === 'settings' ? '' : 'settings')}
        >
          <SlidersHorizontal size={14} />
          Параметры
        </button>
        <button
          className={`button secondary small ${panel === 'versions' ? 'selected' : ''}`}
          disabled={!game || busy}
          onClick={() => setPanel(panel === 'versions' ? '' : 'versions')}
        >
          <History size={14} />
          Версии<span className="count">{versions.length || game?.version || 0}</span>
        </button>
        <span className="toolbar-spacer" />
        <button className="button secondary small" onClick={onEmbed} disabled={!game || busy}>
          <Link2 size={14} />
          Ссылка на игру
        </button>
        <button className="button yellow small" disabled={!html || busy} onClick={onExport}>
          <Download size={14} /> Скачать HTML
        </button>
      </div>
    </div>
  );
}

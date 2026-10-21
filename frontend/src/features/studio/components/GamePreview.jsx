import { useEffect, useState, useMemo } from 'react';
import { Spinner } from '../../../shared/ui/Spinner';
import { EmptyState } from '../../../shared/ui/EmptyState';
import { previewHTML } from '../previewHTML';
import { PreviewToolbar } from './PreviewToolbar';
import { PreviewFooter } from './PreviewFooter';
import { GameSettings } from './GameSettings';
import { VersionHistory } from './VersionHistory';

export function GamePreview({ editor, demo, onEmbed, onSelectPrompt }) {
  const {
    game,
    html,
    versions,
    options,
    setOptions,
    busy,
    generating,
    exportGame: onExport,
    restore: onRestore,
  } = editor;
  const [view, setView] = useState('preview');
  const [reload, setReload] = useState(0);
  const [panel, setPanel] = useState('');
  const [expanded, setExpanded] = useState(false);
  const safeHTML = useMemo(() => previewHTML(html || ''), [html]);
  useEffect(() => {
    setView('preview');
    setReload((value) => value + 1);
    setPanel('');
  }, [game?.id]);
  useEffect(() => {
    if (!expanded) {
      return;
    }
    const close = (event) => event.key === 'Escape' && setExpanded(false);
    document.addEventListener('keydown', close);
    return () => document.removeEventListener('keydown', close);
  }, [expanded]);
  return (
    <section className={`preview-panel panel ${expanded ? 'expanded' : ''}`}>
      <PreviewToolbar
        view={view}
        setView={setView}
        html={html}
        expanded={expanded}
        setExpanded={setExpanded}
        onReload={() => setReload((value) => value + 1)}
      />
      <div className={`preview-stage ${view === 'code' ? 'code-stage' : ''}`}>
        {html ? (
          view === 'preview' ? (
            <div className="iframe-shell">
              <iframe
                key={`${game?.id}-${reload}`}
                srcDoc={safeHTML}
                sandbox="allow-scripts"
                title={`Игра: ${game?.title || 'предпросмотр'}`}
                allow="autoplay"
                data-preview="game"
              />
            </div>
          ) : (
            <pre className="code-view" tabIndex={0}>
              <code>{html}</code>
            </pre>
          )
        ) : (
          <EmptyState
            title="Здесь оживёт ваша идея"
            text="Опишите игру в чате слева — и сыграйте в результат прямо здесь."
          />
        )}
        {busy && (
          <div className="preview-busy">
            <Spinner />
            <span>{generating ? 'Генерируем новую версию…' : 'Загружаем игру…'}</span>
            <small>
              {generating ? 'Это может занять до нескольких минут' : 'Подготавливаем предпросмотр'}
            </small>
          </div>
        )}
      </div>
      <div className="preview-infobar">
        <span>
          <i />
          {html
            ? demo
              ? 'HTML5 Canvas · автономно'
              : 'HTML5 · предпросмотр'
            : 'Готово к вашей идее'}
        </span>
        <span>
          Адаптивный экран
          <span className="info-divider" />{' '}
          {html
            ? `${new Blob([html]).size / 1024 < 1 ? '< 1' : Math.round(new Blob([html]).size / 1024)} КБ`
            : '—'}
        </span>
      </div>
      <PreviewFooter
        game={game}
        demo={demo}
        busy={busy}
        html={html}
        panel={panel}
        setPanel={setPanel}
        versions={versions}
        onExport={onExport}
        onEmbed={onEmbed}
      />
      {panel === 'settings' && (
        <GameSettings
          demo={demo}
          options={options}
          setOptions={setOptions}
          onClose={() => setPanel('')}
          onSelectPrompt={(value) => {
            onSelectPrompt(value);
            setPanel('');
          }}
        />
      )}
      {panel === 'versions' && (
        <VersionHistory
          versions={versions}
          game={game}
          demo={demo}
          busy={busy}
          onRestore={onRestore}
          onClose={() => setPanel('')}
        />
      )}
    </section>
  );
}

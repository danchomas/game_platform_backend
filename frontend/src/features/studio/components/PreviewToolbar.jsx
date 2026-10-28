import { RotateCcw, Maximize2, Code2, Play, X, Smartphone } from 'lucide-react';

export function PreviewToolbar({
  view,
  setView,
  html,
  expanded,
  setExpanded,
  onReload,
  orientation,
  onRotate,
}) {
  const landscape = orientation === 'landscape';
  return (
    <div className="preview-toolbar">
      <div className="preview-tabs">
        <button className={view === 'preview' ? 'active' : ''} onClick={() => setView('preview')}>
          <Play size={13} />
          Предпросмотр
        </button>
        <button
          className={view === 'code' ? 'active' : ''}
          onClick={() => setView('code')}
          disabled={!html}
        >
          <Code2 size={15} />
          Код
        </button>
      </div>
      <div className="preview-actions">
        <button
          className={`icon-btn rotate-btn ${landscape ? 'landscape' : ''}`}
          onClick={onRotate}
          disabled={view !== 'preview'}
          aria-label={
            landscape ? 'Повернуть телефон вертикально' : 'Повернуть телефон горизонтально'
          }
          title={landscape ? 'Вертикально' : 'Горизонтально'}
        >
          <Smartphone size={15} />
        </button>
        <button
          className="icon-btn"
          onClick={onReload}
          disabled={!html}
          aria-label="Перезапустить игру"
        >
          <RotateCcw size={15} />
        </button>
        <button
          className="icon-btn"
          onClick={() => setExpanded(!expanded)}
          aria-label={expanded ? 'Закрыть полный экран' : 'Развернуть предпросмотр'}
        >
          {expanded ? <X size={16} /> : <Maximize2 size={15} />}
        </button>
      </div>
    </div>
  );
}

import { X } from 'lucide-react';

export function VersionHistory({ versions, game, demo, busy, onRestore, onClose }) {
  return (
    <div className="settings-drawer">
      <div className="drawer-title">
        <h3>История версий</h3>
        <button className="icon-btn" aria-label="Закрыть версии" onClick={onClose}>
          <X size={15} />
        </button>
      </div>
      {(versions.length
        ? versions
        : [
            {
              version: game.version,
              instruction: 'Готовый демо-пример',
            },
          ]
      ).map((version) => (
        <div className="version-row" key={version.version}>
          <span className="version-badge">v{version.version}</span>
          <div>
            {version.instruction || 'Новая версия'}
            <small>
              {version.created_at
                ? new Date(version.created_at).toLocaleString('ru-RU')
                : demo
                  ? 'Готовая демонстрационная игра'
                  : 'Дата не указана'}
            </small>
          </div>
          {version.version === game.version ? (
            <span className="subtle-label">Текущая</span>
          ) : (
            <button
              className="text-button"
              disabled={busy}
              onClick={() => onRestore(version.version)}
            >
              Вернуть
            </button>
          )}
        </div>
      ))}
    </div>
  );
}

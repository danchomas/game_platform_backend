import { ArrowUpRight, X } from 'lucide-react';

export function GameSettings({ demo, options, setOptions, onClose, onSelectPrompt }) {
  return (
    <div className="settings-drawer">
      <div className="drawer-title">
        <h3>Параметры игры</h3>
        <button className="icon-btn" aria-label="Закрыть параметры" onClick={onClose}>
          <X size={15} />
        </button>
      </div>
      {demo ? (
        <>
          <label className="range-label">
            Темп игры{' '}
            <strong>
              {options.speed <= 3 ? 'Спокойный' : options.speed >= 8 ? 'Быстрый' : 'Средний'}
            </strong>
            <input
              type="range"
              min="1"
              max="10"
              value={options.speed}
              onChange={(event) => {
                setOptions({
                  ...options,
                  speed: Number(event.target.value),
                });
              }}
            />
          </label>
          <label className="toggle-label">
            Крупные кнопки
            <button
              className={`toggle ${options.large ? 'on' : ''}`}
              role="switch"
              aria-checked={options.large}
              onClick={() => {
                setOptions({
                  ...options,
                  large: !options.large,
                });
              }}
              aria-label="Крупные кнопки"
            >
              <i />
            </button>
          </label>
          <p className="drawer-note">
            Демо-параметры применяются при новом запуске. Для ИИ-игры опишите изменения в чате.
          </p>
        </>
      ) : (
        <>
          <p className="drawer-note">
            Параметры зависят от сгенерированной игры. Напишите, что изменить — ИИ обновит код.
          </p>
          {[
            'Уменьши скорость на 30%',
            'Увеличь кнопки управления',
            'Добавь три уровня сложности',
          ].map((suggestion) => (
            <button
              key={suggestion}
              className="parameter-prompt"
              onClick={() => {
                onSelectPrompt(suggestion);
              }}
            >
              {suggestion}
              <ArrowUpRight size={13} />
            </button>
          ))}
        </>
      )}
    </div>
  );
}

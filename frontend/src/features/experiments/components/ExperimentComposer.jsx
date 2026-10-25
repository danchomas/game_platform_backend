import { useState } from 'react';
import { FlaskConical } from 'lucide-react';

const VARIANT_OPTIONS = [2, 3, 4];

export function ExperimentComposer({ busy, disabled, onCreate }) {
  const [prompt, setPrompt] = useState('');
  const [variants, setVariants] = useState(3);

  async function handleSubmit(event) {
    event.preventDefault();
    if (await onCreate(prompt.trim(), variants)) {
      setPrompt('');
    }
  }

  return (
    <form className="panel experiment-composer" onSubmit={handleSubmit}>
      <div className="panel-title">
        <div>
          <h2>Новый мультиген</h2>
          <p>Опишите игру один раз — соберём несколько разных версий</p>
        </div>
      </div>
      <textarea
        value={prompt}
        onChange={(event) => setPrompt(event.target.value)}
        placeholder="Например: аркада про сбор звёзд для пассажиров в метро"
        rows={4}
        maxLength={4000}
        disabled={disabled}
      />
      <div className="experiment-composer-row">
        <div className="segment" role="group" aria-label="Сколько вариантов собрать">
          {VARIANT_OPTIONS.map((count) => (
            <button
              type="button"
              key={count}
              aria-pressed={variants === count}
              className={variants === count ? 'active' : ''}
              onClick={() => setVariants(count)}
              disabled={disabled}
            >
              {count} варианта
            </button>
          ))}
        </div>
        <button
          className="button yellow"
          type="submit"
          disabled={disabled || busy || prompt.trim().length < 3}
        >
          <FlaskConical size={15} />
          {busy ? 'Запускаем…' : 'Собрать варианты'}
        </button>
      </div>
    </form>
  );
}

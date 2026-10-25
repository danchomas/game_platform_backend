import { useState } from 'react';
import { ArrowUp, Sparkles, ChevronDown, Mic } from 'lucide-react';
import { Spinner } from '../../../shared/ui/Spinner';
import { useVoiceInput } from '../hooks/useVoiceInput';
import { MAX_PROMPT_LENGTH } from '../generationPrompt';

const NEW_GAME_SUGGESTIONS = ['Змейка для бабушки', 'Пазл для детей', 'Кликер про космос'];
const PLAN_SUGGESTIONS = ['Управление кнопками', 'Сделай темп спокойнее', 'Добавь уровни'];
const EDIT_SUGGESTIONS = ['Сделай кнопки больше', 'Добавь уровни', 'Сделай темп спокойнее'];

function getSuggestions(game, hasPlan) {
  if (hasPlan) {
    return PLAN_SUGGESTIONS;
  }
  return game ? EDIT_SUGGESTIONS : NEW_GAME_SUGGESTIONS;
}

function getPlaceholder(game, hasPlan) {
  if (hasPlan) {
    return 'Что поправить в плане?';
  }
  return game ? 'Что изменим в игре?' : 'Например, замути змейку для бабушки…';
}

export function PromptComposer({
  game,
  hasPlan,
  busy,
  demo,
  prompt,
  setPrompt,
  onGenerate,
  notify,
}) {
  const [audience, setAudience] = useState('Для всех');
  const { recording, toggleRecording: handleVoiceInput } = useVoiceInput({
    onTranscript: (text) => setPrompt((previous) => [previous, text].filter(Boolean).join(' ')),
    notify,
  });
  async function handleSubmit(event) {
    event?.preventDefault();
    if (busy || prompt.trim().length < 3) {
      return;
    }
    const success = await onGenerate(prompt.trim(), audience);
    if (success) {
      setPrompt('');
    }
  }
  const suggestions = getSuggestions(game, hasPlan);
  return (
    <div className="composer-area">
      <div className="suggestion-list">
        {suggestions.map((suggestion, index) => (
          <button key={suggestion} onClick={() => setPrompt(suggestion)} disabled={busy}>
            <span>{['↗', '✳', '↝'][index]}</span>
            {suggestion}
          </button>
        ))}
      </div>
      <form onSubmit={handleSubmit} className="composer">
        <textarea
          aria-label="Запрос для ИИ"
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder={getPlaceholder(game, hasPlan)}
          maxLength={MAX_PROMPT_LENGTH}
          disabled={busy}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault();
              handleSubmit();
            }
          }}
        />
        <div className="composer-bottom">
          <div className="audience-select">
            <span>Для кого</span>
            <select
              aria-label="Целевая аудитория"
              value={audience}
              onChange={(event) => setAudience(event.target.value)}
              disabled={busy}
            >
              <option>Для всех</option>
              <option>Для детей</option>
              <option>Для молодёжи</option>
              <option>Для старшего поколения</option>
            </select>
            <ChevronDown size={12} />
          </div>
          <button
            type="button"
            className={`icon-btn ${recording ? 'recording' : ''}`}
            onClick={handleVoiceInput}
            disabled={busy}
            aria-label={recording ? 'Остановить запись' : 'Ввести запрос голосом'}
          >
            <Mic size={17} />
          </button>
          <button
            className="send-button"
            type="submit"
            disabled={busy || prompt.trim().length < 3}
            aria-label="Сгенерировать игру"
          >
            {busy ? <Spinner /> : <ArrowUp size={19} />}
          </button>
        </div>
      </form>
      <div className="composer-note">
        <span>
          <Sparkles size={10} />{' '}
          {demo ? 'Демо · генерация доступна с сервером' : 'LLM · с учётом игровой аналитики'}
        </span>
        <span>Enter ↵</span>
      </div>
    </div>
  );
}

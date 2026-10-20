import { useMemo, useRef, useState } from 'react';
import { getDemoMessages } from '../../../demo/messages.js';
import { renderDemoGame } from '../../../demo/renderDemoGame.js';
import { download } from '../../../shared/lib/download.js';
import { useDemoEvents } from './useDemoEvents.js';
import { buildGenerationPrompt, getVisibleMessage } from '../generationPrompt.js';

export function useGameEditor({
  api,
  isDemo,
  userId,
  initialGame,
  onChanged,
  onError,
  notify,
  onRequireConnection,
}) {
  const [game, setGame] = useState(() => (isDemo ? initialGame : null));
  const [sourceHTML, setSourceHTML] = useState('');
  const [messages, setMessages] = useState(() => (isDemo ? getDemoMessages(initialGame) : []));
  const [versions, setVersions] = useState([]);
  const [options, setOptions] = useState({
    speed: 5,
    large: true,
  });
  const [operation, setOperation] = useState(null);
  const operationLock = useRef(false);
  const html = useMemo(
    () => (isDemo ? (game ? renderDemoGame(game, options) : '') : sourceHTML),
    [isDemo, game, options, sourceHTML],
  );
  const busy = operation !== null;
  useDemoEvents({
    enabled: isDemo,
    gameId: game?.id,
  });
  function reset(nextGame = null, demo = isDemo) {
    setGame(nextGame);
    setSourceHTML('');
    setMessages(demo ? getDemoMessages(nextGame) : []);
    setVersions([]);
  }

  // The ref locks synchronously, including two clicks before React has rendered the busy state.
  async function run(name, action) {
    if (operationLock.current) {
      notify('Дождитесь завершения текущего запроса.');
      return false;
    }
    operationLock.current = true;
    setOperation(name);
    onError('');
    try {
      await action();
      return true;
    } catch (error) {
      onError(error.message);
      return false;
    } finally {
      operationLock.current = false;
      setOperation(null);
    }
  }
  async function open(nextGame) {
    return run('open', async () => {
      reset(nextGame);
      if (isDemo) {
        return;
      }
      setSourceHTML(await api.getGameHTML(nextGame.id));
      if (nextGame.owner_id !== userId) {
        setMessages([
          {
            role: 'assistant',
            content:
              'Игра из галереи платформы. Можно поиграть в предпросмотр. Доработки и публикация доступны автору.',
          },
        ]);
        return;
      }
      const results = await Promise.allSettled([
        api.getMessages(nextGame.id),
        api.getVersions(nextGame.id),
      ]);
      if (results[0].status === 'fulfilled') {
        setMessages((results[0].value || []).map(getVisibleMessage));
      }
      if (results[1].status === 'fulfilled') {
        setVersions(results[1].value || []);
      }
      const failed = results.find((result) => result.status === 'rejected');
      if (failed) {
        notify(`Игра загружена, но история недоступна: ${failed.reason.message}`);
      }
    });
  }
  async function generate(prompt, audience) {
    if (isDemo) {
      onRequireConnection();
      notify('Для генерации через ИИ подключите ваш бэкенд.');
      return false;
    }
    if (game && game.owner_id !== userId) {
      notify('Эту игру может редактировать только автор. Создайте новый проект.');
      return false;
    }
    const previous = messages;
    return run('generate', async () => {
      const userMessage = {
        role: 'user',
        content: prompt,
      };
      const instruction = buildGenerationPrompt(prompt, audience);
      setMessages([...previous, userMessage]);
      let result;
      try {
        result = await (game ? api.iterateGame(game.id, instruction) : api.createGame(instruction));
      } catch (error) {
        setMessages(previous);
        throw error;
      }
      setGame(result.game);
      setSourceHTML(result.game.html || '');
      setMessages([
        ...previous,
        userMessage,
        {
          role: 'assistant',
          content: result.reply || 'Игра готова. Попробуйте её в предпросмотре.',
          version: result.version,
        },
      ]);
      onChanged();
      await reloadVersions(result.game.id);
    });
  }
  async function reloadVersions(id) {
    try {
      setVersions((await api.getVersions(id)) || []);
    } catch {
      notify('Игра готова. Историю версий пока не удалось загрузить.');
    }
  }
  async function restore(version) {
    if (isDemo || !game) {
      return false;
    }
    return run('restore', async () => {
      const result = await api.restoreVersion(game.id, version);
      setGame(result.game);
      setSourceHTML(result.game.html || '');
      setMessages((previous) => [
        ...previous,
        {
          role: 'assistant',
          content: result.reply,
          version: result.version,
        },
      ]);
      onChanged();
      await reloadVersions(game.id);
      notify(`Версия ${version} восстановлена.`);
    });
  }
  async function exportGame() {
    if (!game || !html) {
      return;
    }
    try {
      const source = isDemo ? html : await api.getGameHTML(game.id);
      download(source, `game-${game.id}.html`);
      notify('HTML-файл игры скачан.');
    } catch (error) {
      onError(error.message);
    }
  }
  function updateGame(patch) {
    setGame((previous) =>
      previous?.id === patch.id
        ? {
            ...previous,
            ...patch,
          }
        : previous,
    );
  }
  function removeGame(id) {
    if (game?.id === id) {
      reset();
    }
  }
  return {
    game,
    html,
    messages,
    versions,
    options,
    setOptions,
    busy,
    generating: operation === 'generate',
    reset,
    open,
    generate,
    restore,
    exportGame,
    updateGame,
    removeGame,
  };
}

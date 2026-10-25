import { useMemo, useRef, useState } from 'react';
import { getDemoMessages } from '../../../demo/messages.js';
import { renderDemoGame } from '../../../demo/renderDemoGame.js';
import { download } from '../../../shared/lib/download.js';
import { useDemoEvents } from './useDemoEvents.js';
import { buildGenerationPrompt, getVisibleMessage } from '../generationPrompt.js';
import { STATUS, isBusyStatus, isPlanStage, parseSpec } from '../../games/pipelineStatus.js';

const POLL_INTERVAL_MS = 1500;
const BUILD_TIMEOUT_MS = 10 * 60_000;

// Thrown when the editor switched to another game while a build was being awaited.
const SUPERSEDED = Symbol('superseded');

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

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
  const [plan, setPlan] = useState(null);
  const [stage, setStage] = useState(null);
  const [progress, setProgress] = useState(null);
  const [options, setOptions] = useState({
    speed: 5,
    large: true,
  });
  const [operation, setOperation] = useState(null);
  const operationLock = useRef(false);
  const editorSession = useRef(0);
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
    editorSession.current++;
    setGame(nextGame);
    setSourceHTML('');
    setMessages(demo ? getDemoMessages(nextGame) : []);
    setVersions([]);
    setPlan(null);
    setStage(null);
    setProgress(null);
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
      if (error !== SUPERSEDED) {
        onError(error.message);
      }
      return false;
    } finally {
      operationLock.current = false;
      setOperation(null);
      setStage(null);
      setProgress(null);
    }
  }

  // Builds run in the background on the server; the editor follows them through the status endpoint.
  async function waitForBuild(gameId) {
    const session = editorSession.current;
    const deadline = Date.now() + BUILD_TIMEOUT_MS;
    while (Date.now() < deadline) {
      await delay(POLL_INTERVAL_MS);
      if (session !== editorSession.current) {
        throw SUPERSEDED;
      }
      const status = await api.getStatus(gameId);
      if (session !== editorSession.current) {
        throw SUPERSEDED;
      }
      setStage(status.status);
      setProgress(status.progress ? { ...status.progress, receivedAt: Date.now() } : null);
      if (status.game && isBusyStatus(status.status)) {
        setGame(status.game);
      }
      if (!isBusyStatus(status.status)) {
        return status;
      }
    }
    throw new Error('Сборка идёт дольше обычного. Откройте игру позже из коллекции.');
  }

  async function applyBuild(status, previousVersion) {
    const nextGame = status.game;
    setGame(nextGame);
    onChanged();
    if (status.status === STATUS.failed) {
      setPlan(parseSpec(nextGame.spec));
      throw new Error(
        status.detail || 'Не удалось собрать игру. Поправьте план или попробуйте ещё раз.',
      );
    }
    const [source, history] = await Promise.all([
      api.getGameHTML(nextGame.id),
      api.getMessages(nextGame.id),
    ]);
    setSourceHTML(source);
    setMessages((history || []).map(getVisibleMessage));
    await reloadVersions(nextGame.id);
    if (nextGame.version === previousVersion) {
      throw new Error(status.detail || 'Изменения не применены. Попробуйте сформулировать иначе.');
    }
    if (status.detail) {
      notify(status.detail);
    }
  }

  async function followBuild(gameToFollow) {
    return run('generate', async () => {
      setStage(gameToFollow.status);
      await applyBuild(await waitForBuild(gameToFollow.id), gameToFollow.version);
    });
  }

  async function open(nextGame) {
    const opened = await run('open', async () => {
      reset(nextGame);
      if (isDemo) {
        return;
      }
      const hasCode = nextGame.version > 0;
      if (nextGame.owner_id !== userId) {
        setSourceHTML(await api.getGameHTML(nextGame.id));
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
        hasCode ? api.getGameHTML(nextGame.id) : Promise.resolve(''),
      ]);
      if (results[0].status === 'fulfilled') {
        setMessages((results[0].value || []).map(getVisibleMessage));
      }
      if (results[1].status === 'fulfilled') {
        setVersions(results[1].value || []);
      }
      if (results[2].status === 'rejected') {
        throw results[2].reason;
      }
      setSourceHTML(results[2].value);
      if (isPlanStage(nextGame)) {
        setPlan(parseSpec(nextGame.spec));
      }
      const failed = results.slice(0, 2).find((result) => result.status === 'rejected');
      if (failed) {
        notify(`Игра загружена, но история недоступна: ${failed.reason.message}`);
      }
    });
    // A build started earlier (or in another tab) is still running: keep following it.
    if (opened && !isDemo && nextGame.owner_id === userId && isBusyStatus(nextGame.status)) {
      void followBuild(nextGame);
    }
    return opened;
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
    if (game && isBusyStatus(game.status)) {
      notify('Игра ещё собирается. Дождитесь окончания сборки.');
      return false;
    }
    const previous = messages;
    const userMessage = {
      role: 'user',
      content: prompt,
    };
    return run('generate', async () => {
      setMessages([...previous, userMessage]);
      if (!game || isPlanStage(game)) {
        setStage(STATUS.planning);
        let result;
        try {
          result = game
            ? await api.revisePlan(game.id, prompt)
            : await api.createGame(buildGenerationPrompt(prompt, audience));
        } catch (error) {
          setMessages(previous);
          onChanged();
          throw error;
        }
        setGame(result.game);
        setPlan(result.plan);
        setMessages([...previous, userMessage, { role: 'assistant', content: result.reply }]);
        onChanged();
        return;
      }
      try {
        await api.iterateGame(game.id, buildGenerationPrompt(prompt, audience));
      } catch (error) {
        setMessages(previous);
        throw error;
      }
      setStage(STATUS.generating);
      await applyBuild(await waitForBuild(game.id), game.version);
    });
  }

  async function approve() {
    if (isDemo || !isPlanStage(game)) {
      return false;
    }
    return run('generate', async () => {
      const result = await api.approvePlan(game.id);
      setGame(result.game);
      setPlan(null);
      setStage(STATUS.generating);
      await applyBuild(await waitForBuild(game.id), game.version);
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
    plan,
    stage,
    progress,
    options,
    setOptions,
    busy,
    generating: operation === 'generate',
    reset,
    open,
    generate,
    approve,
    restore,
    exportGame,
    updateGame,
    removeGame,
  };
}

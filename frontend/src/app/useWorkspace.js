import { useCallback, useRef, useState } from 'react';
import { useSession } from '../features/connection/hooks/useSession';
import { useGames } from '../features/games/hooks/useGames';
import { useGameEditor } from '../features/studio/hooks/useGameEditor';
import { useAnalytics } from '../features/analytics/hooks/useAnalytics';
import { useExperiments } from '../features/experiments/hooks/useExperiments';
import { useNotifications } from '../shared/ui/useNotifications';

/** Coordinates navigation and dialogs; feature state and server work stay in feature hooks. */
export function useWorkspace() {
  const session = useSession();
  const notifications = useNotifications();
  const { notify, setError, clearError } = notifications;
  const [page, setPage] = useState('studio');
  const [menuOpen, setMenuOpen] = useState(false);
  const [dialog, setDialog] = useState(null);
  const [dialogBusy, setDialogBusy] = useState(false);
  const [dialogError, setDialogError] = useState('');
  const dialogLock = useRef(false);
  const openConnection = useCallback(
    () =>
      setDialog({
        type: 'settings',
      }),
    [],
  );
  const openHelp = useCallback(
    () =>
      setDialog({
        type: 'help',
      }),
    [],
  );
  const library = useGames({
    api: session.api,
    isDemo: session.isDemo,
    onError: setError,
  });
  const editor = useGameEditor({
    api: session.api,
    isDemo: session.isDemo,
    userId: session.auth?.user?.id,
    initialGame: library.demoGames[0] || null,
    onChanged: library.refresh,
    onError: setError,
    notify,
    onRequireConnection: openConnection,
  });
  const analytics = useAnalytics({
    api: session.api,
    isDemo: session.isDemo,
    enabled: page === 'analytics',
    demoGames: library.demoGames,
    onError: setError,
  });
  const experiments = useExperiments({
    api: session.api,
    isDemo: session.isDemo,
    enabled: page === 'experiments',
    onError: setError,
    onChanged: library.refresh,
    notify,
  });
  const closeDialog = useCallback(() => {
    if (dialogLock.current) {
      return;
    }
    setDialog(null);
    setDialogError('');
  }, []);
  function navigate(nextPage) {
    setPage(nextPage);
    setMenuOpen(false);
    clearError();
  }
  function canSwitchGame() {
    if (!editor.busy) {
      return true;
    }
    notify('Дождитесь завершения текущего запроса.');
    return false;
  }
  function newGame() {
    if (!canSwitchGame()) {
      return;
    }
    editor.reset();
    navigate('studio');
  }
  // A variant is an ordinary game owned by the caller; the library may not have loaded it yet.
  function openVariant(variant) {
    return openGame({
      id: variant.game_id,
      title: variant.title,
      version: variant.version,
      status: variant.status,
      published: variant.published,
      owner_id: session.auth?.user?.id,
    });
  }
  function openGame(game) {
    if (!canSwitchGame()) {
      return;
    }
    navigate('studio');
    return editor.open(library.games.find((item) => item.id === game.id) || game);
  }
  function connect(config, auth) {
    if (!canSwitchGame()) {
      return;
    }
    session.connect(config, auth);
    editor.reset(null, false);
    library.setCollection('mine');
    analytics.reset();
    clearError();
    closeDialog();
    notify('Сервер подключён. Можно создавать игры.');
  }
  function enterDemo(config) {
    if (!canSwitchGame()) {
      return;
    }
    session.enterDemo(config);
    editor.reset(library.demoGames[0] || null, true);
    analytics.reset();
    clearError();
    closeDialog();
  }
  function openPublish() {
    if (!editor.game) {
      return;
    }
    if (
      !session.isDemo &&
      !editor.game.published &&
      editor.game.owner_id !== session.auth?.user?.id
    ) {
      notify('Публикация доступна автору игры.');
      return;
    }
    setDialog({
      type: 'publish',
    });
  }
  async function runDialogAction(action) {
    if (dialogLock.current) {
      return;
    }
    dialogLock.current = true;
    setDialogBusy(true);
    setDialogError('');
    try {
      await action();
    } catch (error) {
      setDialogError(error.message);
    } finally {
      dialogLock.current = false;
      setDialogBusy(false);
    }
  }
  async function publish(published) {
    return runDialogAction(async () => {
      const game = await session.api.setPublished(editor.game.id, published);
      editor.updateGame(game);
      library.refresh();
      analytics.refresh();
      notify(
        published ? 'Игра опубликована. Ссылка готова для WebView.' : 'Игра снята с публикации.',
      );
    });
  }
  async function confirmMutation() {
    if (!dialog?.game) {
      return;
    }
    return runDialogAction(async () => {
      const game = dialog.game;
      if (dialog.type === 'delete') {
        await library.remove(game);
        editor.removeGame(game.id);
      } else {
        await library.rename(game, dialog.title.trim());
        editor.updateGame({
          id: game.id,
          title: dialog.title.trim(),
        });
      }
      analytics.refresh();
      setDialog(null);
      notify(dialog.type === 'delete' ? 'Игра удалена.' : 'Название обновлено.');
    });
  }
  const playBase = session.config.base || window.location.origin;
  // <img> cannot send the Authorization header; drafts' previews accept ?token= like the play iframe.
  const previewURL = (gameId, version) =>
    `${playBase}/api/games/${encodeURIComponent(gameId)}/preview.jpg?v=${version}` +
    (session.auth?.token ? `&token=${encodeURIComponent(session.auth.token)}` : '');
  const gameURL = editor.game
    ? `${session.config.base || window.location.origin}/g/${encodeURIComponent(editor.game.id)}`
    : '';
  return {
    session,
    library,
    editor,
    analytics,
    experiments,
    multigen: {
      playBase,
      previewURL,
      openVariant,
    },
    notifications,
    navigation: {
      page,
      menuOpen,
      setMenuOpen,
      navigate,
      newGame,
      openGame,
    },
    dialogs: {
      current: dialog,
      set: setDialog,
      close: closeDialog,
      busy: dialogBusy,
      error: dialogError,
      openConnection,
      openHelp,
      connect,
      enterDemo,
      openPublish,
      publish,
      confirmMutation,
      gameURL,
    },
  };
}

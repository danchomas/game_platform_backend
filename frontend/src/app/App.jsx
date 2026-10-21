import { useCallback } from 'react';
import { Sidebar } from './layout/Sidebar';
import { Topbar } from './layout/Topbar';
import { ErrorBanner } from '../shared/ui/ErrorBanner';
import { Toast } from '../shared/ui/Toast';
import { StudioPage } from '../features/studio/StudioPage';
import { GalleryPage } from '../features/games/GalleryPage';
import { AnalyticsPage } from '../features/analytics/AnalyticsPage';
import { WorkspaceDialogs } from './WorkspaceDialogs';
import { useWorkspace } from './useWorkspace';

export default function App() {
  const { session, library, editor, analytics, notifications, navigation, dialogs } =
    useWorkspace();
  const { page, menuOpen, setMenuOpen, navigate, newGame, openGame } = navigation;
  const closeMenu = useCallback(() => setMenuOpen(false), [setMenuOpen]);
  return (
    <div className="app">
      <Sidebar
        page={page}
        menuOpen={menuOpen}
        onClose={closeMenu}
        demo={session.isDemo}
        games={library.games}
        demoGames={library.demoGames}
        active={editor.game}
        busy={editor.busy}
        auth={session.auth}
        initial={session.initial}
        navigate={navigate}
        newGame={newGame}
        openGame={openGame}
        onSettings={dialogs.openConnection}
        onHelp={dialogs.openHelp}
      />
      {menuOpen && (
        <button className="sidebar-overlay" aria-label="Закрыть меню" onClick={closeMenu} />
      )}
      <main
        className={`main ${page === 'studio' ? 'main--studio' : ''}`}
        inert={menuOpen ? '' : undefined}
      >
        <Topbar
          page={page}
          menuOpen={menuOpen}
          setMenuOpen={setMenuOpen}
          demo={session.isDemo}
          initial={session.initial}
          onSettings={dialogs.openConnection}
          onHelp={dialogs.openHelp}
        />
        <ErrorBanner
          error={notifications.error}
          onClose={notifications.clearError}
          onSettings={dialogs.openConnection}
        />
        {page === 'studio' && (
          <StudioPage
            editor={editor}
            demo={session.isDemo}
            onNew={newGame}
            onEmbed={dialogs.openPublish}
            notify={notifications.notify}
            menuOpen={menuOpen}
            onOpenMenu={() => setMenuOpen(true)}
          />
        )}
        {page === 'gallery' && (
          <GalleryPage
            library={library}
            demo={session.isDemo}
            onOpen={openGame}
            onNew={newGame}
            onRename={(game) =>
              dialogs.set({
                type: 'rename',
                game,
                title: game.title,
              })
            }
            onDelete={(game) =>
              dialogs.set({
                type: 'delete',
                game,
              })
            }
          />
        )}
        {page === 'analytics' && (
          <AnalyticsPage
            analytics={analytics}
            demo={session.isDemo}
            games={library.games}
            onOpen={openGame}
          />
        )}
      </main>
      <Toast toast={notifications.toast} onClose={notifications.clearToast} />
      <WorkspaceDialogs
        dialogs={dialogs}
        session={session}
        editor={editor}
        notify={notifications.notify}
      />
    </div>
  );
}

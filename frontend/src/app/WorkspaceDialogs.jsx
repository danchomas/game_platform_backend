import { ConnectionDialog } from '../features/connection/components/ConnectionDialog';
import { PublishDialog } from '../features/games/components/PublishDialog';
import { GameMutationDialog } from '../features/games/components/GameMutationDialog';
import { HelpDialog } from '../shared/ui/HelpDialog';

export function WorkspaceDialogs({ dialogs, session, editor, notify }) {
  const dialog = dialogs.current;
  if (!dialog) {
    return null;
  }
  switch (dialog.type) {
    case 'settings':
      return (
        <ConnectionDialog
          config={session.config}
          auth={session.auth}
          demo={session.isDemo}
          onClose={dialogs.close}
          onConnected={dialogs.connect}
          onDemo={dialogs.enterDemo}
        />
      );
    case 'help':
      return <HelpDialog onClose={dialogs.close} />;
    case 'publish':
      return (
        <PublishDialog
          demo={session.isDemo}
          canPublish={editor.game?.owner_id === session.auth?.user?.id && !session.isDemo}
          game={editor.game}
          gameURL={dialogs.gameURL}
          busy={dialogs.busy}
          error={dialogs.error}
          onClose={dialogs.close}
          onPublish={dialogs.publish}
          onConnect={dialogs.openConnection}
          onExport={editor.exportGame}
          notify={notify}
        />
      );
    case 'rename':
    case 'delete':
      return (
        <GameMutationDialog
          modal={dialog}
          busy={dialogs.busy}
          error={dialogs.error}
          onClose={dialogs.close}
          onChange={dialogs.set}
          onConfirm={dialogs.confirmMutation}
        />
      );
    default:
      return null;
  }
}

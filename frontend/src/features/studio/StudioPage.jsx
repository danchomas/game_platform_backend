import { useEffect, useState } from 'react';
import { ChatPanel } from './components/ChatPanel';
import { GamePreview } from './components/GamePreview';

export function StudioPage({ editor, demo, onNew, onEmbed, notify, menuOpen, onOpenMenu }) {
  const [prompt, setPrompt] = useState('');
  const [mobilePanel, setMobilePanel] = useState('preview');
  const gameId = editor.game?.id;

  useEffect(() => {
    setMobilePanel(gameId ? 'preview' : 'chat');
  }, [gameId]);

  function handleNewGame() {
    if (editor.busy) {
      return;
    }
    onNew();
    setMobilePanel('chat');
  }

  function handleSelectPrompt(value) {
    setPrompt(value);
    setMobilePanel('chat');
  }

  return (
    <div className="page studio-page">
      <div className="studio-mobile-tabs" aria-label="Рабочие панели">
        <button aria-pressed={mobilePanel === 'chat'} onClick={() => setMobilePanel('chat')}>
          Чат
        </button>
        <button aria-pressed={mobilePanel === 'preview'} onClick={() => setMobilePanel('preview')}>
          Игра
        </button>
      </div>
      <div className="studio-layout" data-mobile-panel={mobilePanel}>
        <ChatPanel
          editor={editor}
          demo={demo}
          prompt={prompt}
          setPrompt={setPrompt}
          notify={notify}
          onNew={handleNewGame}
          menuOpen={menuOpen}
          onOpenMenu={onOpenMenu}
        />
        <GamePreview
          editor={editor}
          demo={demo}
          onEmbed={onEmbed}
          onSelectPrompt={handleSelectPrompt}
        />
      </div>
    </div>
  );
}

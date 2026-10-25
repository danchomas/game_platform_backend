import { Menu, Plus, Sparkles } from 'lucide-react';
import { ChatMessages } from './ChatMessages';
import { PromptComposer } from './PromptComposer';

export function ChatPanel({
  editor,
  demo,
  prompt,
  setPrompt,
  notify,
  onNew,
  menuOpen,
  onOpenMenu,
}) {
  return (
    <section className="chat-panel panel">
      <div className="chat-header">
        <div>
          <button
            className="icon-btn studio-menu-toggle"
            onClick={onOpenMenu}
            aria-label="Открыть меню"
            aria-expanded={menuOpen}
            aria-controls="workspace-menu"
            title="Открыть меню"
          >
            <Menu size={19} />
          </button>
          <span className="assistant-icon">
            <Sparkles size={18} />
          </span>
          <span className="chat-project">
            <span className="project-title">{editor.game?.title || 'Новый проект'}</span>
            <small>
              ИИ-соавтор{editor.game ? ` · v${editor.game.version}` : ''}
              {demo ? ' · Демо' : ''}
            </small>
          </span>
        </div>
        <button
          className="icon-btn"
          onClick={onNew}
          disabled={editor.busy}
          aria-label="Новый проект"
          title="Новый проект"
        >
          <Plus size={18} />
        </button>
      </div>
      <ChatMessages
        messages={editor.messages}
        demo={demo}
        busy={editor.busy}
        generating={editor.generating}
        stage={editor.stage}
        plan={editor.plan}
        onApprove={editor.approve}
      />
      <PromptComposer
        game={editor.game}
        hasPlan={Boolean(editor.plan)}
        busy={editor.busy}
        demo={demo}
        prompt={prompt}
        setPrompt={setPrompt}
        onGenerate={editor.generate}
        notify={notify}
      />
    </section>
  );
}

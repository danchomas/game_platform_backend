import { HelpCircle, ChevronDown, Menu } from 'lucide-react';
import { PAGE_NAMES } from '../navigation';

export function Topbar({ page, menuOpen, setMenuOpen, demo, initial, onSettings, onHelp }) {
  return (
    <header className="topbar">
      <button
        className="icon-btn menu-toggle"
        onClick={() => setMenuOpen(!menuOpen)}
        aria-label="Открыть меню"
        aria-expanded={menuOpen}
        aria-controls="workspace-menu"
      >
        <Menu size={20} />
      </button>
      <div className="breadcrumbs">
        <span>Рабочее пространство</span>
        <span>/</span>
        <strong>{PAGE_NAMES[page]}</strong>
      </div>
      <div className="topbar-right">
        <button className={`connection-status ${demo ? 'demo' : ''}`} onClick={onSettings}>
          <i />
          {demo ? 'Демо-режим' : 'Сервер подключён'}
          <ChevronDown size={12} />
        </button>
        <span className="top-divider" />
        <button className="help-button" onClick={onHelp}>
          <HelpCircle size={16} />
          <span>Как это работает</span>
        </button>
        <span className="top-avatar">{initial}</span>
      </div>
    </header>
  );
}

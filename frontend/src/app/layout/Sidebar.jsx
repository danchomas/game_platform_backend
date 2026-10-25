import { useEffect, useRef } from 'react';
import { Settings2, HelpCircle, ArrowUpRight, Plus, ChevronDown, X } from 'lucide-react';
import { NAVIGATION, PAGE_NAMES } from '../navigation';

export function Sidebar({
  menuOpen,
  onClose,
  page,
  demo,
  games,
  demoGames,
  active,
  busy,
  auth,
  initial,
  navigate,
  newGame,
  openGame,
  onSettings,
  onHelp,
}) {
  const menuRef = useRef(null);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }
    const previousFocus = document.activeElement;
    menuRef.current.scrollTop = 0;
    menuRef.current.querySelector('button').focus({ preventScroll: true });
    function handleKeyDown(event) {
      if (event.key === 'Escape') {
        onClose();
      }
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocus?.focus({ preventScroll: true });
    };
  }, [menuOpen, onClose]);

  return (
    <aside
      ref={menuRef}
      id="workspace-menu"
      className={`sidebar ${menuOpen ? 'open' : ''}`}
      aria-label="Навигация студии"
      inert={menuOpen ? undefined : ''}
    >
      <button className="icon-btn sidebar-close" onClick={onClose} aria-label="Скрыть меню">
        <X size={16} />
      </button>
      <a
        className="brand"
        href="#studio"
        onClick={(event) => {
          event.preventDefault();
          navigate('studio');
        }}
      >
        <span>
          Студия игр<small>ПРОСТРАНСТВО ДЛЯ ИДЕЙ</small>
        </span>
      </a>
      <button className="workspace-switch" onClick={onSettings}>
        <span className="workspace-avatar">И</span>
        <span>
          Рабочее пространство<small>{demo ? 'Демо-пространство' : 'Моя команда'}</small>
        </span>
        <ChevronDown size={13} />
      </button>
      <div className="nav-label">ПРОСТРАНСТВО ДЛЯ ИДЕЙ</div>
      <nav>
        {NAVIGATION.map(({ id, icon: Icon }) => (
          <button className={page === id ? 'active' : ''} onClick={() => navigate(id)} key={id}>
            <Icon size={18} strokeWidth={1.6} />
            <span>{PAGE_NAMES[id]}</span>
            {id === 'studio' ? (
              <span className="nav-star">✳</span>
            ) : id === 'gallery' ? (
              <span className="nav-count">{games.length}</span>
            ) : null}
          </button>
        ))}
      </nav>
      <div className="sidebar-projects">
        <div className="nav-label">
          НЕДАВНИЕ ИГРЫ
          <button onClick={newGame} disabled={busy} aria-label="Новая игра">
            <Plus size={13} />
          </button>
        </div>
        {(demo ? demoGames : games).slice(0, 3).map((game, index) => (
          <button
            key={game.id}
            onClick={() => openGame(game)}
            className={active?.id === game.id && page === 'studio' ? 'selected' : ''}
          >
            <i className={`project-color color-${index}`} />
            <span>{game.title}</span>
          </button>
        ))}
        {!games.length && !demo && (
          <span className="sidebar-empty">Ваша первая игра уже близко</span>
        )}
      </div>
      <div className="sidebar-bottom">
        <div className="sidebar-promo">
          <span className="promo-spark">✳</span>
          <p>
            Всё начинается
            <br />с «а что, если…»
          </p>
          <button onClick={newGame} disabled={busy}>
            Создать свою игру <ArrowUpRight size={14} />
          </button>
        </div>
        <button className="sidebar-utility" onClick={onHelp}>
          <HelpCircle size={17} />
          Как это работает
          <ArrowUpRight size={13} />
        </button>
        <button className="sidebar-utility" onClick={onSettings}>
          <Settings2 size={17} />
          Подключение{demo && <span className="connection-dot" />}
        </button>
        <button className="profile" onClick={onSettings}>
          <span className="profile-avatar">
            {initial}
            <i />
          </span>
          <span>
            {auth?.user?.name || 'Менеджер'}
            <small>{demo ? 'Демонстрационный режим' : 'Менеджер платформы'}</small>
          </span>
          <ChevronDown size={13} />
        </button>
      </div>
    </aside>
  );
}

import { useEffect, useRef } from 'react';
import { X, Sparkles } from 'lucide-react';

export function Modal({ title, subtitle, children, onClose, wide = false }) {
  const dialog = useRef(null);
  useEffect(() => {
    const previouslyFocusedElement = document.activeElement;
    dialog.current.focus();
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        onClose();
      }
      if (event.key === 'Tab') {
        const items = [
          ...dialog.current.querySelectorAll(
            'button, input, select, textarea, a[href], [tabindex="0"]',
          ),
        ].filter((element) => !element.disabled);
        if (!items.length) {
          event.preventDefault();
          return;
        }
        if (
          event.shiftKey &&
          (document.activeElement === items[0] || document.activeElement === dialog.current)
        ) {
          event.preventDefault();
          items.at(-1).focus();
        } else if (!event.shiftKey && document.activeElement === items.at(-1)) {
          event.preventDefault();
          items[0].focus();
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = previousOverflow;
      previouslyFocusedElement?.focus();
    };
  }, [onClose]);
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        className={`modal ${wide ? 'wide' : ''}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        tabIndex={-1}
        ref={dialog}
      >
        <button className="icon-btn close" onClick={onClose} aria-label="Закрыть">
          <X size={20} />
        </button>
        <div className="modal-symbol">
          <Sparkles size={23} />
        </div>
        <h2 id="modal-title">{title}</h2>
        {subtitle && <p className="modal-subtitle">{subtitle}</p>}
        {children}
      </section>
    </div>
  );
}

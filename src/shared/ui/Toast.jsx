import { Check, X } from 'lucide-react';

export function Toast({ toast, onClose }) {
  if (!toast) {
    return null;
  }
  return (
    <div className="toast" role="status">
      <span>
        <Check size={14} />
      </span>
      {toast}
      <button onClick={onClose} aria-label="Закрыть уведомление">
        <X size={14} />
      </button>
    </div>
  );
}

import { X, AlertCircle } from 'lucide-react';

export function ErrorBanner({ error, onClose, onSettings }) {
  if (!error) {
    return null;
  }
  return (
    <div className="error-banner" role="alert">
      <AlertCircle size={17} />
      <span>{error}</span>
      <button className="text-button" onClick={onSettings}>
        Подключение
      </button>
      <button className="icon-btn" onClick={onClose} aria-label="Закрыть ошибку">
        <X size={16} />
      </button>
    </div>
  );
}

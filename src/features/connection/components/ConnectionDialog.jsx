import { ArrowRight, LogOut, AlertCircle } from 'lucide-react';
import { Modal } from '../../../shared/ui/Modal';
import { Spinner } from '../../../shared/ui/Spinner';
import { useConnectionForm } from '../hooks/useConnectionForm';

export function ConnectionDialog({ config, auth, demo, onClose, onConnected, onDemo }) {
  const {
    base,
    setBase,
    tab,
    setTab,
    name,
    setName,
    code,
    setCode,
    managerCode,
    setManagerCode,
    busy,
    error,
    setError,
    connect,
    continueDemo,
  } = useConnectionForm({
    config,
    onConnected,
    onDemo,
  });
  return (
    <Modal
      title={demo ? 'Подключим вашу студию' : 'Ваше подключение'}
      subtitle="Генерация и игровая аналитика работают через ваш Go-бэкенд."
      onClose={busy ? () => {} : onClose}
    >
      {!demo && auth && (
        <div className="connected-account">
          <span className="published-pill">
            <i />
            Аккаунт подключён
          </span>
          <strong>{auth.user.name}</strong>
          {auth.access_key && (
            <label className="field-label">
              Ваш код для повторного входа
              <input readOnly value={auth.access_key} />
            </label>
          )}
          <p>Сохраните код доступа, чтобы войти с другого устройства.</p>
        </div>
      )}
      <form onSubmit={connect}>
        <label className="field-label">
          Адрес API
          <input
            value={base}
            onChange={(event) => setBase(event.target.value)}
            placeholder="Пусто — API через локальный прокси"
            disabled={busy}
          />
          <span className="field-help">
            При запуске через Vite оставьте пустым: запросы идут на localhost:8080. Другой сервер
            задаётся в .env.
          </span>
        </label>
        <div className="segment connection-tabs">
          <button
            type="button"
            className={tab === 'register' ? 'active' : ''}
            disabled={busy}
            onClick={() => {
              setTab('register');
              setError('');
            }}
          >
            Новый менеджер
          </button>
          <button
            type="button"
            className={tab === 'login' ? 'active' : ''}
            disabled={busy}
            onClick={() => {
              setTab('login');
              setError('');
            }}
          >
            Войти по коду
          </button>
        </div>
        {tab === 'register' ? (
          <>
            <label className="field-label">
              Ваше имя
              <input
                required
                minLength={2}
                maxLength={40}
                placeholder="Например, Саша"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={busy}
              />
            </label>
            <label className="field-label">
              Код менеджера <span className="optional">если задан на сервере</span>
              <input
                type="password"
                value={managerCode}
                onChange={(event) => setManagerCode(event.target.value)}
                placeholder="MANAGER_CODE"
                disabled={busy}
              />
            </label>
          </>
        ) : (
          <label className="field-label">
            Код доступа
            <input
              required
              placeholder="Код, полученный при регистрации"
              value={code}
              onChange={(event) => setCode(event.target.value)}
              disabled={busy}
            />
          </label>
        )}
        {error && (
          <div className="form-error" role="alert">
            <AlertCircle size={15} />
            {error}
          </div>
        )}
        <button className="button yellow full" type="submit" disabled={busy}>
          {busy ? <Spinner /> : <ArrowRight size={16} />}{' '}
          {busy ? 'Подключаемся…' : 'Подключить сервер'}
        </button>
      </form>
      <button className="button secondary full demo-button" disabled={busy} onClick={continueDemo}>
        {demo ? (
          'Продолжить в деморежиме'
        ) : (
          <>
            <LogOut size={15} /> Выйти и открыть демо
          </>
        )}
      </button>
      <p className="connection-footer">
        Ключ OpenRouter хранится только на сервере.
        <br />
        Деморежим работает без интернета.
      </p>
    </Modal>
  );
}

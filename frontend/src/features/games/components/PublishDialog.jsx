import { ArrowUpRight, Copy, Download, Globe, Link2 } from 'lucide-react';
import { Modal } from '../../../shared/ui/Modal';
import { Spinner } from '../../../shared/ui/Spinner';
import { copyToClipboard } from '../../../shared/lib/clipboard';

export function PublishDialog({
  game: active,
  gameURL,
  busy: actionBusy,
  error: modalError,
  onClose: closeModal,
  onPublish: publish,
  onExport: exportGame,
  notify,
  demo,
  canPublish,
  onConnect,
}) {
  const copy = (text) => copyToClipboard(text, notify);
  return (
    <Modal
      title={
        demo
          ? 'Поделитесь своей игрой'
          : active?.published
            ? 'Ссылка на игру'
            : 'Пора выпустить игру'
      }
      subtitle={
        demo
          ? 'Постоянная ссылка появляется после публикации на сервере.'
          : 'Игра по ссылке открывается в браузере и мобильном WebView.'
      }
      onClose={closeModal}
    >
      {modalError && (
        <div className="form-error" role="alert">
          {modalError}
        </div>
      )}
      {demo ? (
        <>
          <div className="info-note">
            <Link2 size={17} />
            <p>
              Сейчас открыт локальный демо-пример. Подключите сервер и создайте игру, чтобы
              опубликовать её и получить ссылку. Демо можно скачать одним HTML-файлом.
            </p>
          </div>
          <div className="modal-actions">
            <button className="button secondary" onClick={exportGame}>
              <Download size={15} /> Скачать HTML
            </button>
            <button className="button yellow" onClick={onConnect}>
              Подключить сервер
            </button>
          </div>
        </>
      ) : active?.published ? (
        <>
          <span className="published-pill">
            <i />
            Опубликована · версия {active.version}
          </span>
          <label className="field-label">
            Ссылка на игру
            <div className="copy-field">
              <input readOnly value={gameURL} />
              <button
                className="icon-btn"
                onClick={() => copy(gameURL)}
                aria-label="Скопировать ссылку"
              >
                <Copy size={16} />
              </button>
            </div>
          </label>
          <label className="field-label">
            Код для встраивания
            <div className="embed-code">
              <code>{`<iframe src="${gameURL}"\n  title="${active.title.replace(/["<>]/g, '')}"\n  width="100%" height="640"\n  sandbox="allow-scripts"\n  allow="autoplay"></iframe>`}</code>
              <button
                className="icon-btn"
                onClick={() =>
                  copy(
                    `<iframe src="${gameURL}" title="${active.title.replace(/["<>]/g, '')}" width="100%" height="640" sandbox="allow-scripts" allow="autoplay"></iframe>`,
                  )
                }
                aria-label="Скопировать код"
              >
                <Copy size={16} />
              </button>
            </div>
          </label>
          <p className="field-help">
            Для мобильного приложения используйте ссылку как URL WebView. Опциональный параметр
            ?uid=ID позволяет связать игровые события с игроком.
          </p>
          <div className="modal-actions">
            <button className="button secondary" onClick={exportGame}>
              <Download size={15} />
              Скачать HTML
            </button>
            <a className="button yellow" href={gameURL} target="_blank" rel="noreferrer">
              Открыть игру
              <ArrowUpRight size={15} />
            </a>
          </div>
          {canPublish && (
            <button
              className="text-button unpublish"
              disabled={actionBusy}
              onClick={() => publish(false)}
            >
              {actionBusy ? <Spinner /> : null}Снять с публикации
            </button>
          )}
        </>
      ) : (
        <>
          <div className="publish-summary">
            <Globe size={25} />
            <div>
              <strong>{active?.title}</strong>
              <span>HTML5 · v{active?.version} · адаптивное управление</span>
            </div>
          </div>
          <div className="info-note">
            <Link2 size={17} />
            <p>
              Игра станет доступна по публичной ссылке и в галерее. Последующие правки изменят
              опубликованную версию.
            </p>
          </div>
          <button
            className="button yellow full"
            onClick={() => publish(true)}
            disabled={actionBusy}
          >
            {actionBusy ? <Spinner /> : <Globe size={16} />}Опубликовать игру
          </button>
        </>
      )}
    </Modal>
  );
}

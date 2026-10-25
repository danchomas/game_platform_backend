import { Modal } from '../../../shared/ui/Modal';
import { Spinner } from '../../../shared/ui/Spinner';

export function GameMutationDialog({
  modal,
  busy: actionBusy,
  error: modalError,
  onClose: closeModal,
  onChange: setModal,
  onConfirm: mutateGame,
}) {
  return (
    <Modal
      title={modal.type === 'delete' ? 'Удалить игру?' : 'Дадим игре новое имя'}
      subtitle={
        modal.type === 'delete'
          ? `«${modal.game.title}» будет удалена вместе с историей. Это действие нельзя отменить.`
          : 'Короткое название поможет найти её в коллекции.'
      }
      onClose={closeModal}
    >
      {modalError && <div className="form-error">{modalError}</div>}
      {modal.type === 'rename' && (
        <label className="field-label">
          Название
          <input
            autoFocus
            maxLength={80}
            value={modal.title}
            onChange={(event) =>
              setModal({
                ...modal,
                title: event.target.value,
              })
            }
          />
        </label>
      )}
      <div className="modal-actions">
        <button className="button secondary" onClick={closeModal} disabled={actionBusy}>
          Отмена
        </button>
        <button
          className={`button ${modal.type === 'delete' ? 'danger' : 'yellow'}`}
          disabled={actionBusy || (modal.type === 'rename' && !modal.title?.trim())}
          onClick={mutateGame}
        >
          {actionBusy ? <Spinner /> : null}
          {modal.type === 'delete' ? 'Удалить' : 'Сохранить'}
        </button>
      </div>
    </Modal>
  );
}

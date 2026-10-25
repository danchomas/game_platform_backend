import { Sparkles, ArrowRight } from 'lucide-react';
import { Modal } from './Modal';

export function HelpDialog({ onClose: closeModal }) {
  return (
    <Modal
      title="Идея → игра → вовлечение"
      subtitle="Ваша мини-студия HTML5-игр. Четыре простых шага."
      onClose={closeModal}
    >
      <div className="help-steps">
        {[
          [
            '01',
            'Опишите идею',
            'Расскажите о жанре, аудитории и настроении. ИИ сгенерирует код игры через подключённый сервер.',
          ],
          [
            '02',
            'Сыграйте и доработайте',
            'Проверьте управление и мобильный экран. Попросите ИИ изменить механику или оформление.',
          ],
          [
            '03',
            'Поделитесь игрой',
            'Опубликуйте игру, скопируйте ссылку для WebView или скачайте автономный HTML.',
          ],
          [
            '04',
            'Узнайте свою аудиторию',
            'События опубликованной игры собираются бэкендом и появляются в аналитике.',
          ],
        ].map(([stepNumber, title, description]) => (
          <div key={stepNumber}>
            <span>{stepNumber}</span>
            <div>
              <h3>{title}</h3>
              <p>{description}</p>
            </div>
          </div>
        ))}
      </div>
      <div className="info-note">
        <Sparkles size={17} />
        <p>
          В деморежиме доступны четыре готовые игры и пример аналитики. Генерация новых игр требует
          запущенного бэкенда с ключом OpenRouter.
        </p>
      </div>
      <button className="button yellow full" onClick={closeModal}>
        Всё понятно <ArrowRight size={15} />
      </button>
    </Modal>
  );
}

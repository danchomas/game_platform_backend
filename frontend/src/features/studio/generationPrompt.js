export const MAX_PROMPT_LENGTH = 3000;

const REQUIREMENTS_MARKER = '\n\n[GAME_STUDIO_TECHNICAL_REQUIREMENTS]\n';

const TECHNICAL_REQUIREMENTS = [
  'Верни полную рабочую HTML5-игру. Игровое поле и геймплей реализуй на чистом Canvas 2D или WebGL; HTML допустим для меню и кнопок.',
  'Результат — один автономный HTML-файл со встроенными JavaScript, CSS и ресурсами. Не используй CDN, внешние библиотеки, удалённые ассеты, серверный рендеринг или обязательные сетевые запросы во время игры.',
  'Игра должна работать в мобильном WebView: адаптивный размер, touch/pointer-управление, старт, геймплей, победа или поражение и повторный запуск.',
  'Не делай доступность SDK аналитики или сети условием запуска и прохождения игры.',
].join('\n');

const requirementsSuffix = REQUIREMENTS_MARKER + TECHNICAL_REQUIREMENTS;

export function buildGenerationPrompt(prompt, audience = 'Для всех') {
  const text = prompt.trim();
  if (text.length < 3 || text.length > MAX_PROMPT_LENGTH) {
    throw new Error(`Запрос должен содержать от 3 до ${MAX_PROMPT_LENGTH} символов.`);
  }
  const audienceInstruction =
    audience === 'Для всех'
      ? ''
      : `\nЦелевая аудитория: ${audience}. Учти её при выборе управления, темпа и оформления.`;
  return text + audienceInstruction + requirementsSuffix;
}

// Keep the manager's chat readable when the server returns stored generation instructions.

// Keep the manager's chat readable when the server returns stored generation instructions.
export function getVisibleMessage(message) {
  if (message.role !== 'user' || !message.content?.endsWith(requirementsSuffix)) {
    return message;
  }
  return {
    ...message,
    content: message.content.slice(0, -requirementsSuffix.length),
  };
}

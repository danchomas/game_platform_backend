export function getDemoMessages(game) {
  if (!game) {
    return [];
  }
  return [
    {
      role: 'user',
      content: game.prompt,
    },
    {
      role: 'assistant',
      content:
        'Это готовый демо-пример — можно поиграть, настроить темп и скачать HTML. Подключите сервер в настройках, чтобы создавать новые игры по своим идеям.',
      version: game.version,
    },
  ];
}

export const GAME_FILTERS = [
  {
    value: 'all',
    label: 'Все игры',
  },
  {
    value: 'published',
    label: 'Опубликованные',
  },
  {
    value: 'drafts',
    label: 'Черновики',
  },
];

export function filterGames(games, { query = '', status = 'all', sort = 'updated' } = {}) {
  const search = query.trim().toLocaleLowerCase('ru-RU');
  const result = games.filter((game) => {
    const text = [game.title, game.genre, game.audience]
      .filter(Boolean)
      .join(' ')
      .toLocaleLowerCase('ru-RU');
    const matchesStatus =
      status === 'all' || (status === 'published' ? game.published : !game.published);
    return text.includes(search) && matchesStatus;
  });
  return result.sort((firstGame, secondGame) => {
    if (sort === 'popular') {
      return (secondGame.plays || 0) - (firstGame.plays || 0);
    }
    if (sort === 'title') {
      return firstGame.title.localeCompare(secondGame.title, 'ru');
    }
    return new Date(secondGame.updated_at || 0) - new Date(firstGame.updated_at || 0);
  });
}

export function GameStatus({ published }) {
  return (
    <span className={`status ${published ? 'live' : ''}`}>
      <i />
      {published ? 'Опубликована' : 'Черновик'}
    </span>
  );
}

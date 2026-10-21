import { getStatusLabel } from '../pipelineStatus';

// Pipeline stages take priority: a draft that is still being built is not a plain draft.
export function GameStatus({ published, status }) {
  const pipelineLabel = getStatusLabel(status);
  if (pipelineLabel) {
    return (
      <span className="status working">
        <i />
        {pipelineLabel}
      </span>
    );
  }
  return (
    <span className={`status ${published ? 'live' : ''}`}>
      <i />
      {published ? 'Опубликована' : 'Черновик'}
    </span>
  );
}

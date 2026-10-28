import { getExperimentLabel } from '../experimentStatus';

export function ExperimentList({ list, selectedId, onSelect }) {
  if (!list.length) {
    return null;
  }
  return (
    <section className="panel experiment-list">
      <div className="panel-title">
        <h2>Эксперименты</h2>
        <span className="counter">{list.length}</span>
      </div>
      <ul>
        {list.map((experiment) => (
          <li key={experiment.id}>
            <button
              className={experiment.id === selectedId ? 'selected' : ''}
              onClick={() => onSelect(experiment.id)}
            >
              <span className="experiment-list-title">{experiment.title}</span>
              <span className="experiment-list-meta">
                <span className={`status ${statusClass(experiment.status)}`}>
                  <i />
                  {getExperimentLabel(experiment.status)}
                </span>
                <small>
                  собрано {experiment.variants_ready} из {experiment.variants_total}
                </small>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

export function statusClass(status) {
  if (status === 'running') {
    return 'live';
  }
  return status === 'draft' ? 'working' : '';
}

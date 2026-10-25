import { formatNumber } from '../../../shared/lib/format';

export function CustomEvents({ events = [] }) {
  if (!events.length) {
    return null;
  }
  return (
    <section className="events-strip">
      <span>События в играх</span>
      {events.map((event) => (
        <span key={event.type}>
          <code>{event.type}</code>
          <strong>{formatNumber(event.count)}</strong>
        </span>
      ))}
    </section>
  );
}

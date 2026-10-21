import { Sparkles } from 'lucide-react';

export function EmptyState({ title, text, action }) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Sparkles size={28} strokeWidth={1.3} />
      </div>
      <h3>{title}</h3>
      <p>{text}</p>
      {action}
    </div>
  );
}

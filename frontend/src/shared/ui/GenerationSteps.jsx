import { Check } from 'lucide-react';

export function GenerationSteps({ busy }) {
  return (
    <div className="gen-steps">
      {['Понимаем вашу идею', 'Генерируем код игры', 'Готовим предпросмотр'].map((text) => (
        <span key={text} className={busy ? 'processing' : ''}>
          {busy ? <span className="step-dot" /> : <Check size={12} />}
          <span>{text}</span>
        </span>
      ))}
    </div>
  );
}

import { Check } from 'lucide-react';

// steps: [{ label, state: 'done' | 'active' | 'pending' }] — the caller maps server stages to steps.
export function GenerationSteps({ steps }) {
  return (
    <div className="gen-steps">
      {steps.map((step) => (
        <span key={step.label} className={step.state}>
          {step.state === 'done' ? <Check size={12} /> : <span className="step-dot" />}
          <span>{step.label}</span>
        </span>
      ))}
    </div>
  );
}

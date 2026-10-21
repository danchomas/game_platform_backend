import { useEffect, useRef } from 'react';
import { ArrowUpRight, Sparkles, Check } from 'lucide-react';
import { GenerationSteps as Steps } from '../../../shared/ui/GenerationSteps';
import { STATUS, getBuildSteps } from '../../games/pipelineStatus';
import { PlanCard } from './PlanCard';

function ProgressNote({ stage }) {
  if (stage === STATUS.planning) {
    return (
      <p>
        Продумываю игру: механику, управление и оформление.
        <br />
        Обычно это 10–30 секунд.
      </p>
    );
  }
  return (
    <>
      <p>
        Собираю игру и проверяю её в мобильном браузере.
        <br />
        Обычно это 1–3 минуты.
      </p>
      <Steps steps={getBuildSteps(stage)} />
    </>
  );
}

export function ChatMessages({ messages, demo, busy, generating, stage, plan, onApprove }) {
  const end = useRef(null);
  useEffect(() => {
    end.current?.scrollIntoView({
      behavior: 'smooth',
      block: 'nearest',
    });
  }, [messages, busy, stage, plan]);
  return (
    <div className="chat-messages" aria-live="polite">
      {!messages.length ? (
        <div className="welcome">
          <span className="welcome-icon">
            <Sparkles size={28} />
          </span>
          <h3>С чего начнём?</h3>
          <p>
            Расскажите, какая игра вам нужна и кто будет в неё играть. Сначала я предложу план, а
            после вашего подтверждения соберу и проверю игру.
          </p>
          <div className="welcome-example">
            Например: «Змейка для бабушки — крупные кнопки, спокойный темп, тёплые цвета».
          </div>
        </div>
      ) : (
        messages.map((message, index) => (
          <div className={`message ${message.role}`} key={message.id || index}>
            {message.role !== 'user' && (
              <span className="message-avatar">
                <Sparkles size={14} />
              </span>
            )}
            <div>
              <span className="message-author">
                {message.role === 'user' ? 'Вы' : 'Студия игр'}
                {demo && message.role !== 'user' && <small>демо</small>}
              </span>
              <div className="message-body">{message.content}</div>
              {message.version && message.role !== 'user' && (
                <div className="message-result">
                  <span className="result-check">
                    <Check size={12} />
                  </span>
                  <div>
                    Игра готова к запуску<small>HTML5 · версия {message.version} · touch</small>
                  </div>
                  <ArrowUpRight size={15} />
                </div>
              )}
            </div>
          </div>
        ))
      )}
      {plan && !generating && <PlanCard plan={plan} busy={busy} onApprove={onApprove} />}
      {generating && (
        <div className="message assistant generating">
          <span className="message-avatar">
            <Sparkles size={14} />
          </span>
          <div>
            <span className="message-author">Студия игр</span>
            <div className="typing">
              <i />
              <i />
              <i />
            </div>
            <ProgressNote stage={stage} />
          </div>
        </div>
      )}
      <div ref={end} />
    </div>
  );
}

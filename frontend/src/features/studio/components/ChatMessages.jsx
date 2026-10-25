import { useEffect, useRef } from 'react';
import { ArrowUpRight, Sparkles, Check } from 'lucide-react';
import { GenerationSteps as Steps } from '../../../shared/ui/GenerationSteps';

export function ChatMessages({ messages, demo, busy, generating }) {
  const end = useRef(null);
  useEffect(() => {
    end.current?.scrollIntoView({
      behavior: 'smooth',
      block: 'nearest',
    });
  }, [messages, busy]);
  return (
    <div className="chat-messages" aria-live="polite">
      {!messages.length ? (
        <div className="welcome">
          <span className="welcome-icon">
            <Sparkles size={28} />
          </span>
          <h3>С чего начнём?</h3>
          <p>
            Расскажите, какая игра вам нужна и кто будет в неё играть. Остальное возьму на себя.
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
            <p>
              Создаю игру по вашему запросу.
              <br />
              Обычно это занимает 20–90 секунд.
            </p>
            <Steps busy />
          </div>
        </div>
      )}
      <div ref={end} />
    </div>
  );
}

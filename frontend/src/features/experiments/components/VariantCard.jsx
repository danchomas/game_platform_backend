import { useState } from 'react';
import { ArrowUpRight, CircleCheck, CircleAlert, Trophy } from 'lucide-react';
import { GameStatus } from '../../games/components/GameStatus';
import { isBusyStatus } from '../../games/pipelineStatus';
import { formatCost } from '../experimentStatus';

export function VariantCard({ variant, previewURL, isWinner, isLeader, onOpen }) {
  const [previewFailed, setPreviewFailed] = useState(false);
  const busy = isBusyStatus(variant.status);
  const showPreview = variant.version > 0 && !previewFailed;

  return (
    <article className={`panel variant-card ${isWinner ? 'winner' : ''}`}>
      <div className="variant-preview">
        {showPreview ? (
          <img
            src={previewURL(variant.game_id, variant.version)}
            alt={`Скриншот варианта ${variant.label}`}
            loading="lazy"
            onError={() => setPreviewFailed(true)}
          />
        ) : (
          <span className="variant-letter">{variant.label}</span>
        )}
        <span className="variant-label">Вариант {variant.label}</span>
        {isWinner ? (
          <span className="variant-badge winner">
            <Trophy size={12} /> Победитель
          </span>
        ) : (
          isLeader && <span className="variant-badge">Лидирует</span>
        )}
      </div>
      <div className="variant-body">
        <div className="variant-head">
          <h3>{variant.version > 0 ? variant.title : `Вариант ${variant.label}`}</h3>
          <GameStatus status={variant.status} published={variant.published} />
        </div>
        <p className="variant-direction">{variant.direction}</p>
        {!busy &&
          (variant.eligible ? (
            <p className="variant-check ok">
              <CircleCheck size={13} /> Прошёл автопроверки
            </p>
          ) : (
            <p className="variant-check rejected">
              <CircleAlert size={13} /> Отсеян: {variant.reject_reason}
            </p>
          ))}
        {variant.status_detail && <p className="variant-detail">{variant.status_detail}</p>}
        <div className="variant-foot">
          <span>
            {formatCost(variant.cost_usd)}
            {variant.fix_rounds > 0 && ` · исправлений: ${variant.fix_rounds}`}
          </span>
          <button
            className="button secondary small"
            onClick={() => onOpen(variant)}
            disabled={variant.version === 0}
          >
            В студии <ArrowUpRight size={13} />
          </button>
        </div>
      </div>
    </article>
  );
}

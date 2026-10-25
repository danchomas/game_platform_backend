import { Copy, ExternalLink, Rocket } from 'lucide-react';
import { copyToClipboard } from '../../shared/lib/clipboard';
import { EmptyState } from '../../shared/ui/EmptyState';
import { ExperimentComposer } from './components/ExperimentComposer';
import { ExperimentList, statusClass } from './components/ExperimentList';
import { ExperimentReport } from './components/ExperimentReport';
import { VariantCard } from './components/VariantCard';
import {
  MIN_VARIANTS_TO_LAUNCH,
  countEligible,
  formatCost,
  getExperimentLabel,
} from './experimentStatus';

export function ExperimentsPage({
  experiments,
  demo,
  playBase,
  previewURL,
  onOpenVariant,
  notify,
}) {
  const { list, selectedId, view, report, busy, building } = experiments;

  return (
    <div className="page experiments-page">
      <div className="page-heading">
        <div>
          <div className="eyebrow">МУЛЬТИГЕН И A/B-ТЕСТ</div>
          <h1>
            Одна идея.
            <br />
            <span>Несколько игр.</span>
          </h1>
          <p>
            Платформа соберёт разные версии игры, отсеет сломанные, а игроки покажут, какая лучше.
          </p>
        </div>
        <div className="heading-note">
          <span className="big-asterisk">✳</span>
          <span>
            Решают игроки,
            <br />а не догадки.
          </span>
        </div>
      </div>
      {demo ? (
        <div className="panel">
          <EmptyState
            title="Нужен сервер"
            text="Мультиген собирает настоящие игры: подключите сервер в настройках."
          />
        </div>
      ) : (
        <div className="experiments-layout">
          <div className="experiments-side">
            <ExperimentComposer
              busy={busy === 'create'}
              disabled={Boolean(busy)}
              onCreate={experiments.create}
            />
            <ExperimentList list={list} selectedId={selectedId} onSelect={experiments.select} />
          </div>
          <div className="experiments-main">
            {view ? (
              <ExperimentDetail
                view={view}
                report={report}
                busy={busy}
                building={building}
                playURL={`${playBase}/e/${encodeURIComponent(view.experiment.id)}`}
                previewURL={previewURL}
                onOpenVariant={onOpenVariant}
                onLaunch={experiments.launch}
                onChooseWinner={experiments.chooseWinner}
                notify={notify}
              />
            ) : (
              <div className="panel">
                <EmptyState
                  title={
                    selectedId ? 'Загружаем эксперимент…' : 'Выберите или создайте эксперимент'
                  }
                  text="Каждый вариант — отдельная игра со своим подходом к механике. Рабочие варианты получат общую ссылку."
                />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ExperimentDetail({
  view,
  report,
  busy,
  building,
  playURL,
  previewURL,
  onOpenVariant,
  onLaunch,
  onChooseWinner,
  notify,
}) {
  const { experiment, variants } = view;
  const eligible = countEligible(variants);
  const totalCost = variants.reduce((sum, variant) => sum + (variant.cost_usd || 0), 0);
  const isDraft = experiment.status === 'draft';
  const winnerId = experiment.winner_id;

  return (
    <>
      <section className="panel experiment-header">
        <div>
          <span className={`status ${statusClass(experiment.status)}`}>
            <i />
            {getExperimentLabel(experiment.status)}
          </span>
          <h2>{experiment.title}</h2>
          <p>
            Рабочих вариантов: {eligible} из {variants.length}
            {building && ' · сборка продолжается'} · стоимость генерации {formatCost(totalCost)}
          </p>
        </div>
        {isDraft ? (
          <div className="experiment-actions">
            <button
              className="button yellow"
              onClick={onLaunch}
              disabled={Boolean(busy) || eligible < MIN_VARIANTS_TO_LAUNCH}
            >
              <Rocket size={15} />
              {busy === 'launch' ? 'Запускаем…' : 'Запустить эксперимент'}
            </button>
            <small>
              {eligible < MIN_VARIANTS_TO_LAUNCH
                ? `Нужно хотя бы ${MIN_VARIANTS_TO_LAUNCH} рабочих варианта`
                : building
                  ? 'Можно запускать сейчас: недособранные варианты в эксперимент не попадут'
                  : 'Сломанные варианты игроки не увидят'}
            </small>
          </div>
        ) : (
          <div className="experiment-link">
            <code>{playURL}</code>
            <button
              className="icon-btn"
              onClick={() => copyToClipboard(playURL, notify)}
              aria-label="Скопировать ссылку"
            >
              <Copy size={15} />
            </button>
            <a
              className="icon-btn"
              href={playURL}
              target="_blank"
              rel="noreferrer"
              aria-label="Открыть ссылку"
            >
              <ExternalLink size={15} />
            </a>
          </div>
        )}
      </section>
      <div className="variant-grid">
        {variants.map((variant) => (
          <VariantCard
            key={variant.game_id}
            variant={variant}
            previewURL={previewURL}
            isWinner={variant.game_id === winnerId}
            isLeader={!winnerId && variant.game_id === report?.leader_id}
            onOpen={onOpenVariant}
          />
        ))}
      </div>
      <ExperimentReport report={report} busy={busy} onChooseWinner={onChooseWinner} />
    </>
  );
}
